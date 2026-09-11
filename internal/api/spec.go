package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/ymhhh/vibecoding/internal/llm"
	"github.com/ymhhh/vibecoding/internal/model"
)

type specRequestBody struct {
	Prompt           string              `json:"prompt"`
	Messages         []model.ChatMessage `json:"messages"`
	SubRequirementID string              `json:"subRequirementId"`
	Scope            string              `json:"scope"` // all | sub | ""
}

func (s *Server) completeLLMJSON(ctx context.Context, cfg model.ModelConfig, system string, msgs []llm.ChatMessage, onDelta func(string)) (string, error) {
	chatReqJSON := llm.ChatRequest{
		ModelConfig: cfg,
		System:      system,
		Messages:    msgs,
		Temperature: 0.3,
		JSONMode:    true,
	}
	chatReqPlain := llm.ChatRequest{
		ModelConfig: cfg,
		System:      system,
		Messages:    msgs,
		Temperature: 0.3,
	}
	if onDelta != nil {
		var gotDelta bool
		wrap := func(delta string) {
			if delta != "" {
				gotDelta = true
			}
			onDelta(delta)
		}
		text, err := s.LLM.ChatStream(ctx, chatReqJSON, wrap)
		if err != nil && !gotDelta {
			text, err = s.LLM.ChatStream(ctx, chatReqPlain, wrap)
		}
		return text, err
	}
	text, err := s.LLM.Chat(ctx, chatReqJSON)
	if err != nil {
		text, err = s.LLM.Chat(ctx, chatReqPlain)
	}
	return text, err
}

func (s *Server) streamOrCompleteJSON(w http.ResponseWriter, r *http.Request, cfg model.ModelConfig, system string, msgs []llm.ChatMessage, finish func(text string) (any, error)) {
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Minute)
	defer cancel()

	if wantsStream(r) {
		sse, err := newSSE(w)
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		_ = sse.event(map[string]any{"type": "status", "message": "calling_model"})
		var lastFlush time.Time
		var buf strings.Builder
		text, err := s.completeLLMJSON(ctx, cfg, system, msgs, func(delta string) {
			buf.WriteString(delta)
			now := time.Now()
			if now.Sub(lastFlush) < 40*time.Millisecond && buf.Len() < 64 {
				return
			}
			chunk := buf.String()
			buf.Reset()
			lastFlush = now
			_ = sse.event(map[string]any{"type": "delta", "text": chunk})
		})
		if rem := buf.String(); rem != "" {
			_ = sse.event(map[string]any{"type": "delta", "text": rem})
		}
		if err != nil {
			_ = sse.event(map[string]any{"type": "error", "error": err.Error()})
			return
		}
		payload, err := finish(text)
		if err != nil {
			_ = sse.event(map[string]any{"type": "error", "error": err.Error()})
			return
		}
		_ = sse.event(payload)
		return
	}

	text, err := s.completeLLMJSON(ctx, cfg, system, msgs, nil)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	payload, err := finish(text)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, payload)
}

func issueRepoDesc(issue *model.Issue, proj *model.Project) string {
	if issue == nil || proj == nil {
		return ""
	}
	var b strings.Builder
	for _, rid := range issue.AssociatedRepoIDs {
		for _, gr := range proj.GitRepos {
			if gr.ID == rid {
				b.WriteString(fmt.Sprintf("- %s path=%s branch=%s\n", gr.Name, gr.Path, gr.DefaultBranch))
			}
		}
	}
	return b.String()
}

func recentChatMsgs(messages []model.ChatMessage) []llm.ChatMessage {
	start := 0
	if len(messages) > 12 {
		start = len(messages) - 12
	}
	msgs := make([]llm.ChatMessage, 0, 8)
	for _, m := range messages[start:] {
		role := "assistant"
		if m.Sender == "user" {
			role = "user"
		} else if m.Sender == "system" {
			continue
		}
		text := m.Text
		if role == "assistant" && len(text) > 800 {
			text = text[:800] + "…"
		}
		msgs = append(msgs, llm.ChatMessage{Role: role, Content: text})
	}
	return msgs
}

func subIndexMarkdown(issue *model.Issue) string {
	if issue == nil || !issue.HasSubRequirements() {
		return ""
	}
	var b strings.Builder
	for _, sub := range issue.SubRequirements {
		b.WriteString(fmt.Sprintf("- [%d] id=%s title=%s status=%s\n", sub.Order, sub.ID, sub.Title, sub.Status))
		if strings.TrimSpace(sub.Description) != "" {
			b.WriteString("  desc: " + sub.Description + "\n")
		}
	}
	return b.String()
}

func allSubSpecsMarkdown(issue *model.Issue) string {
	if issue == nil {
		return ""
	}
	var b strings.Builder
	for _, sub := range issue.SubRequirements {
		b.WriteString(fmt.Sprintf("\n===== SUB %d id=%s title=%s =====\n", sub.Order, sub.ID, sub.Title))
		if sub.DevSpec != nil {
			b.WriteString(sub.DevSpec.RawMarkdown)
		} else {
			b.WriteString("(empty)")
		}
		b.WriteString("\n")
	}
	return b.String()
}

func specDonePayload(spec *model.DevSpec, chatReply, process string, issue *model.Issue) map[string]any {
	out := map[string]any{
		"type":      "done",
		"spec":      spec,
		"text":      chatReply,
		"chatReply": chatReply,
		"process":   process,
	}
	if issue != nil {
		out["subRequirements"] = issue.SubRequirements
		out["devSpec"] = issue.DevSpec
	}
	return out
}

func (s *Server) handleSplitIssue(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	issue, err := s.Store.GetIssue(id)
	if err != nil || issue == nil {
		writeErr(w, 404, "issue not found")
		return
	}
	var body specRequestBody
	_ = decodeJSON(r, &body)

	proj, _ := s.Store.GetProject(issue.ProjectID)
	cfg, _ := s.resolveModel(issue.ProjectID)
	repoDesc := issueRepoDesc(issue, proj)

	system := `You are a Senior VibeCoding AI Architect.
Split a LARGE product requirement into ordered, independently implementable sub-requirements.

Return ONLY a JSON object (no markdown fences) with:
- chatReply: short natural-language reply (2-8 sentences)
- overviewMarkdown: parent overview Dev Spec in Markdown listing the split and overall architecture
- title: parent title
- subRequirements: array of 2-8 items, each:
  - title, description
  - rawMarkdown: COMPLETE Dev Spec for THAT sub-requirement only (Markdown with Title, Summary, Architecture, Target Files, Implementation Steps, Test Cases)
  - summary, architectureDesign
  - fileChanges: [{filePath, repoName, action(create|modify|delete), summary}]
  - implementationSteps, testCases

Rules:
- Order subRequirements by implementation sequence (dependencies first).
- Each sub-requirement must be small enough for one Auto-Dev coding pass.
- Do not overlap file ownership unless a later sub must extend an earlier one.
- Use exact repo names from context. Relative file paths only.
- If the user already provided a Dev Spec, reuse and partition it rather than inventing unrelated work.`

	prev := ""
	if issue.DevSpec != nil {
		prev = issue.DevSpec.RawMarkdown
	}
	prompt := strings.TrimSpace(body.Prompt)
	if prompt == "" {
		prompt = "请将当前需求拆分成若干可独立实施的子需求，每个子需求一份完整待开发文档，按实施顺序排列。"
	}
	user := fmt.Sprintf(`Issue title: %s
Description: %s
Repos:
%s

Previous Dev Spec Markdown (may be empty):
-----
%s
-----

Latest user message:
%s

Split into ordered sub-requirements and return JSON.`,
		issue.Title, issue.PromptDescription(), repoDesc, prev, prompt)

	msgs := recentChatMsgs(body.Messages)
	msgs = append(msgs, llm.ChatMessage{Role: "user", Content: user})

	s.streamOrCompleteJSON(w, r, cfg, system, msgs, func(text string) (any, error) {
		split, err := llm.ParseSplitJSON(text, issue.Title)
		if err != nil {
			return nil, err
		}
		for i := range split.SubRequirements {
			if strings.TrimSpace(split.SubRequirements[i].ID) == "" || strings.HasPrefix(split.SubRequirements[i].ID, "sub-item") {
				split.SubRequirements[i].ID = "sub-" + uuid.NewString()[:8]
			}
			split.SubRequirements[i].Order = i + 1
		}
		issue.SubRequirements = split.SubRequirements
		issue.DevSpec = split.OverviewSpec
		issue.CurrentSubID = ""
		issue.NormalizeSubs()
		issue.UpdatedAt = model.NowISO()
		if err := s.Store.UpsertIssue(*issue); err != nil {
			return nil, err
		}
		saved, _ := s.Store.GetIssue(issue.ID)
		if saved != nil {
			issue = saved
		}
		return specDonePayload(issue.DevSpec, split.ChatReply, text, issue), nil
	})
}

func (s *Server) handleGenerateSpec(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	issue, err := s.Store.GetIssue(id)
	if err != nil || issue == nil {
		writeErr(w, 404, "issue not found")
		return
	}
	var body specRequestBody
	_ = decodeJSON(r, &body)

	proj, _ := s.Store.GetProject(issue.ProjectID)
	cfg, _ := s.resolveModel(issue.ProjectID)
	repoDesc := issueRepoDesc(issue, proj)

	scope := strings.ToLower(strings.TrimSpace(body.Scope))
	subID := strings.TrimSpace(body.SubRequirementID)
	updateAll := issue.HasSubRequirements() && (scope == "all" || (scope == "" && subID == ""))
	updateOne := issue.HasSubRequirements() && subID != "" && !updateAll
	if updateOne && issue.SubByID(subID) == nil {
		writeErr(w, 400, "sub-requirement not found")
		return
	}

	msgs := recentChatMsgs(body.Messages)

	switch {
	case updateAll:
		s.generateAllSubSpecs(w, r, cfg, issue, repoDesc, body.Prompt, msgs)
	case updateOne:
		s.generateOneSubSpec(w, r, cfg, issue, repoDesc, subID, body.Prompt, msgs)
	default:
		s.generateParentSpec(w, r, cfg, issue, repoDesc, body.Prompt, msgs)
	}
}

func (s *Server) generateParentSpec(w http.ResponseWriter, r *http.Request, cfg model.ModelConfig, issue *model.Issue, repoDesc, prompt string, msgs []llm.ChatMessage) {
	system := `You are a Senior VibeCoding AI Architect assisting via chat.
Each user message should UPDATE the Development Spec document.

Return ONLY a JSON object (no markdown fences) with:
- chatReply: short natural-language reply to the user (what you understood / changed); 2-8 sentences; NOT the full document
- rawMarkdown: the COMPLETE updated Dev Spec in Markdown (this is the stored document). Include:
  # Title
  ## Executive Summary / 概述
  ## Architecture Design / 架构设计
  ## Target Files & Changes / 修改文件
  ## Implementation Steps / 实施步骤
  ## Test Cases / 测试用例
- title: short title
Optional agent fields aligned with the markdown:
- summary, architectureDesign
- fileChanges: [{filePath, repoName, action(create|modify|delete), summary}]
- implementationSteps, testCases

Rules:
- If a previous Dev Spec is provided, revise it in place based on the latest user message; do not discard unrelated sections.
- Use exact repo names from context. Relative file paths only.`

	prevSpec := ""
	if issue.DevSpec != nil && strings.TrimSpace(issue.DevSpec.RawMarkdown) != "" {
		prevSpec = issue.DevSpec.RawMarkdown
	}
	user := fmt.Sprintf(`Issue title: %s
Description: %s
Repos:
%s

Previous Dev Spec Markdown (may be empty):
-----
%s
-----

Latest user message:
%s

Update the Dev Spec accordingly and return JSON with chatReply + rawMarkdown.`,
		issue.Title, issue.PromptDescription(), repoDesc, prevSpec, prompt)
	msgs = append(append([]llm.ChatMessage{}, msgs...), llm.ChatMessage{Role: "user", Content: user})

	s.streamOrCompleteJSON(w, r, cfg, system, msgs, func(text string) (any, error) {
		spec, chatReply, err := llm.ParseDevSpecJSON(text, issue.Title)
		if err != nil {
			return nil, err
		}
		issue.DevSpec = spec
		if err := s.Store.UpsertIssue(*issue); err != nil {
			return nil, err
		}
		return specDonePayload(spec, chatReply, text, issue), nil
	})
}

func (s *Server) generateOneSubSpec(w http.ResponseWriter, r *http.Request, cfg model.ModelConfig, issue *model.Issue, repoDesc, subID, prompt string, msgs []llm.ChatMessage) {
	sub := issue.SubByID(subID)
	if sub == nil {
		writeErr(w, 400, "sub-requirement not found")
		return
	}
	system := `You are a Senior VibeCoding AI Architect.
Update ONE sub-requirement's Dev Spec. Do not rewrite sibling sub-requirements.

Return ONLY a JSON object (no markdown fences) with:
- chatReply: short natural-language reply (2-8 sentences)
- rawMarkdown: COMPLETE updated Dev Spec for THIS sub-requirement
- title, summary, architectureDesign
- fileChanges: [{filePath, repoName, action(create|modify|delete), summary}]
- implementationSteps, testCases

Rules:
- Revise in place; keep unrelated sections.
- Stay within this sub-requirement's scope. Use exact repo names. Relative paths only.`

	prev := ""
	if sub.DevSpec != nil {
		prev = sub.DevSpec.RawMarkdown
	}
	user := fmt.Sprintf(`Parent issue: %s
Parent description: %s
Repos:
%s

Sibling sub-requirements (context only, do not rewrite them):
%s

THIS sub-requirement id=%s title=%s
Description: %s

Previous Dev Spec Markdown (may be empty):
-----
%s
-----

Latest user message:
%s

Update THIS sub-requirement Dev Spec and return JSON.`,
		issue.Title, issue.PromptDescription(), repoDesc, subIndexMarkdown(issue),
		sub.ID, sub.Title, sub.Description, prev, prompt)
	msgs = append(append([]llm.ChatMessage{}, msgs...), llm.ChatMessage{Role: "user", Content: user})

	s.streamOrCompleteJSON(w, r, cfg, system, msgs, func(text string) (any, error) {
		spec, chatReply, err := llm.ParseDevSpecJSON(text, sub.Title)
		if err != nil {
			return nil, err
		}
		target := issue.SubByID(subID)
		if target == nil {
			return nil, fmt.Errorf("sub-requirement not found")
		}
		target.DevSpec = spec
		if spec.Title != "" {
			target.Title = spec.Title
		}
		if target.Status != model.SubReqInProgress && target.Status != model.SubReqDone {
			target.Status = model.SubReqReady
		}
		issue.UpdatedAt = model.NowISO()
		if err := s.Store.UpsertIssue(*issue); err != nil {
			return nil, err
		}
		saved, _ := s.Store.GetIssue(issue.ID)
		if saved != nil {
			issue = saved
		}
		return specDonePayload(spec, chatReply, text, issue), nil
	})
}

func (s *Server) generateAllSubSpecs(w http.ResponseWriter, r *http.Request, cfg model.ModelConfig, issue *model.Issue, repoDesc, prompt string, msgs []llm.ChatMessage) {
	system := `You are a Senior VibeCoding AI Architect.
A GLOBAL user instruction must be applied to ALL sub-requirement Dev Specs (and the parent overview).

Return ONLY a JSON object (no markdown fences) with:
- chatReply: short natural-language reply (2-8 sentences)
- overviewMarkdown: updated parent overview Markdown
- title: parent title
- subRequirements: array covering EVERY existing sub-requirement:
  - id: MUST match the existing sub id
  - title, description
  - rawMarkdown: COMPLETE updated Dev Spec for that sub
  - summary, architectureDesign, fileChanges, implementationSteps, testCases

Rules:
- Apply the global instruction to every sub-spec; keep each sub's distinct scope.
- Do not drop a sub-requirement or change ids.
- Use exact repo names. Relative paths only.`

	user := fmt.Sprintf(`Parent issue: %s
Parent description: %s
Repos:
%s

Existing sub-requirements:
%s

Parent overview Markdown:
-----
%s
-----

All current sub-requirement Dev Specs:
%s

Latest GLOBAL user message (apply to ALL sub-specs):
%s

Return JSON with overviewMarkdown + subRequirements (same ids).`,
		issue.Title, issue.PromptDescription(), repoDesc, subIndexMarkdown(issue),
		func() string {
			if issue.DevSpec != nil {
				return issue.DevSpec.RawMarkdown
			}
			return ""
		}(),
		allSubSpecsMarkdown(issue), prompt)
	msgs = append(append([]llm.ChatMessage{}, msgs...), llm.ChatMessage{Role: "user", Content: user})

	s.streamOrCompleteJSON(w, r, cfg, system, msgs, func(text string) (any, error) {
		parsed, err := llm.ParseMultiSubSpecJSON(text, issue.Title)
		if err != nil {
			return nil, err
		}
		llm.ApplyMultiSubSpec(issue, parsed)
		issue.UpdatedAt = model.NowISO()
		if err := s.Store.UpsertIssue(*issue); err != nil {
			return nil, err
		}
		saved, _ := s.Store.GetIssue(issue.ID)
		if saved != nil {
			issue = saved
		}
		reply := parsed.ChatReply
		spec := issue.DevSpec
		return specDonePayload(spec, reply, text, issue), nil
	})
}

func (s *Server) handleExportSpec(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	issue, err := s.Store.GetIssue(id)
	if err != nil || issue == nil {
		writeErr(w, 404, "issue not found")
		return
	}
	subID := strings.TrimSpace(r.URL.Query().Get("subRequirementId"))
	title := issue.Title
	var md string
	if subID != "" && subID != "all" {
		sub := issue.SubByID(subID)
		if sub == nil {
			writeErr(w, 404, "sub-requirement not found")
			return
		}
		title = firstNonEmpty(sub.Title, title)
		if sub.DevSpec != nil {
			md = firstNonEmpty(sub.DevSpec.RawMarkdown, sub.DevSpec.Summary)
		}
	} else if issue.HasSubRequirements() {
		var b strings.Builder
		if issue.DevSpec != nil && strings.TrimSpace(issue.DevSpec.RawMarkdown) != "" {
			b.WriteString(strings.TrimSpace(issue.DevSpec.RawMarkdown))
			b.WriteString("\n\n")
		}
		for _, sub := range issue.SubRequirements {
			b.WriteString("\n---\n\n")
			if sub.DevSpec != nil && strings.TrimSpace(sub.DevSpec.RawMarkdown) != "" {
				b.WriteString(strings.TrimSpace(sub.DevSpec.RawMarkdown))
			} else {
				b.WriteString("# ")
				b.WriteString(sub.Title)
				b.WriteString("\n\n")
				b.WriteString(sub.Description)
			}
			b.WriteString("\n")
		}
		md = b.String()
		if issue.DevSpec != nil && issue.DevSpec.Title != "" {
			title = issue.DevSpec.Title
		}
	} else if issue.DevSpec != nil {
		md = firstNonEmpty(issue.DevSpec.RawMarkdown, issue.DevSpec.Summary)
		if issue.DevSpec.Title != "" {
			title = issue.DevSpec.Title
		}
	}
	if strings.TrimSpace(md) == "" {
		writeErr(w, 404, "no Dev Spec to export")
		return
	}
	filename := specExportFileName(title)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", specContentDisposition(filename))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(md))
}

type exportFileBody struct {
	Filename string `json:"filename"`
	Contents string `json:"contents"`
}

// handleExportFile writes a markdown file to the user's Downloads folder.
// WebView / Safari ignore <a download>, so the local server saves the file instead.
func (s *Server) handleExportFile(w http.ResponseWriter, r *http.Request) {
	var body exportFileBody
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, 400, "invalid json")
		return
	}
	if strings.TrimSpace(body.Contents) == "" {
		writeErr(w, 400, "no content to export")
		return
	}
	dir, err := userDownloadsDir()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	base := strings.TrimSuffix(filepath.Base(strings.TrimSpace(body.Filename)), ".md")
	name := specExportFileName(base)
	path := uniqueFilePath(dir, name)
	if err := os.WriteFile(path, []byte(body.Contents), 0o644); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"path": path})
}

func userDownloadsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "Downloads")
	if st, err := os.Stat(dir); err == nil && st.IsDir() {
		return dir, nil
	}
	return home, nil
}

func uniqueFilePath(dir, name string) string {
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err != nil {
		return path
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 1; i < 1000; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s-%d%s", stem, i, ext))
		if _, err := os.Stat(candidate); err != nil {
			return candidate
		}
	}
	return filepath.Join(dir, fmt.Sprintf("%s-%d%s", stem, time.Now().Unix(), ext))
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func specExportFileName(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "dev-spec.md"
	}
	var b strings.Builder
	lastDash := false
	for _, r := range title {
		switch {
		case r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|':
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		case unicode.IsSpace(r):
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		default:
			b.WriteRune(r)
			lastDash = false
		}
	}
	name := strings.Trim(b.String(), "-")
	if name == "" {
		name = "dev-spec"
	}
	if len([]rune(name)) > 80 {
		name = string([]rune(name)[:80])
	}
	return name + ".md"
}

func specContentDisposition(filename string) string {
	escaped := url.PathEscape(filename)
	return fmt.Sprintf(`attachment; filename="dev-spec.md"; filename*=UTF-8''%s`, escaped)
}

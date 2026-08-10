package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ymhhh/go-common/logger"
	"github.com/ymhhh/vibecoding/internal/autodev"
	"github.com/ymhhh/vibecoding/internal/db"
	"github.com/ymhhh/vibecoding/internal/gitx"
	"github.com/ymhhh/vibecoding/internal/llm"
	"github.com/ymhhh/vibecoding/internal/model"
)

type Server struct {
	Store  *db.Store
	LLM    *llm.Client
	Runner *autodev.Runner
	Hub    *autodev.Hub
	Static fs.FS
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.handleHealth)

	mux.HandleFunc("GET /api/settings/model", s.handleGetModel)
	mux.HandleFunc("PUT /api/settings/model", s.handlePutModel)
	mux.HandleFunc("GET /api/ui-prefs", s.handleGetUIPrefs)
	mux.HandleFunc("PUT /api/ui-prefs", s.handlePutUIPrefs)

	mux.HandleFunc("GET /api/projects", s.handleListProjects)
	mux.HandleFunc("POST /api/projects", s.handleCreateProject)
	mux.HandleFunc("PUT /api/projects/{id}", s.handleUpdateProject)
	mux.HandleFunc("DELETE /api/projects/{id}", s.handleDeleteProject)

	mux.HandleFunc("GET /api/issues", s.handleListIssues)
	mux.HandleFunc("POST /api/issues", s.handleCreateIssue)
	mux.HandleFunc("GET /api/issues/{id}", s.handleGetIssue)
	mux.HandleFunc("PUT /api/issues/{id}", s.handleUpdateIssue)
	mux.HandleFunc("DELETE /api/issues/{id}", s.handleDeleteIssue)
	mux.HandleFunc("POST /api/issues/{id}/approve-merge", s.handleApproveMerge)

	mux.HandleFunc("POST /api/repos/validate", s.handleValidateRepo)

	mux.HandleFunc("POST /api/chat", s.handleChat)
	mux.HandleFunc("POST /api/test-openapi", s.handleTestOpenAPI)
	mux.HandleFunc("POST /api/issues/{id}/spec", s.handleGenerateSpec)

	mux.HandleFunc("POST /api/auto-dev/start", s.handleAutoDevStart)
	mux.HandleFunc("GET /api/auto-dev/jobs/{id}", s.handleGetJob)
	mux.HandleFunc("GET /api/auto-dev/jobs/{id}/events", s.handleJobEvents)
	mux.HandleFunc("POST /api/auto-dev/jobs/{id}/cancel", s.handleCancelJob)

	if s.Static != nil {
		fileServer := http.FileServer(http.FS(s.Static))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.NotFound(w, r)
				return
			}
			// SPA fallback
			path := strings.TrimPrefix(r.URL.Path, "/")
			if path == "" {
				path = "index.html"
			}
			f, err := s.Static.Open(path)
			if err != nil {
				r.URL.Path = "/"
				fileServer.ServeHTTP(w, r)
				return
			}
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
		})
	}

	return withAccessLog(withCORS(mux))
}

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func withAccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		// Skip noisy static/health probes; keep /api/* (except health at debug).
		path := r.URL.Path
		if !strings.HasPrefix(path, "/api/") {
			return
		}
		fields := logger.Fields{
			"method":      r.Method,
			"path":        path,
			"status":      sw.status,
			"bytes":       sw.bytes,
			"duration_ms": time.Since(start).Milliseconds(),
		}
		entry := logger.L().WithFields(fields)
		switch {
		case path == "/api/health":
			entry.Debug("http")
		case sw.status >= 500:
			entry.Error("http")
		case sw.status >= 400:
			entry.Warn("http")
		default:
			entry.Info("http")
		}
	})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	if status >= 500 {
		logger.L().WithField("status", status).Error(msg)
	} else if status >= 400 {
		logger.L().WithField("status", status).Warn(msg)
	}
	writeJSON(w, status, map[string]string{"error": msg})
}

func wantsStream(r *http.Request) bool {
	if r.URL.Query().Get("stream") == "1" {
		return true
	}
	return strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/event-stream")
}

type sseWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

func newSSE(w http.ResponseWriter) (*sseWriter, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming unsupported")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()
	return &sseWriter{w: w, flusher: flusher}, nil
}

func (s *sseWriter) event(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(s.w, "data: %s\n\n", b); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

func decodeJSON(r *http.Request, dest any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 10<<20))
	return dec.Decode(dest)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	cfg, _ := s.Store.GetModelConfig()
	configured, _ := model.MaskKey(cfg.OpenAIAPIKey)
	writeJSON(w, 200, map[string]any{
		"status":         "ok",
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
		"llmConfigured":  configured,
	})
}

func (s *Server) handleGetModel(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.Store.GetModelConfig()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, cfg.Public())
}

func (s *Server) handlePutModel(w http.ResponseWriter, r *http.Request) {
	var cfg model.ModelConfig
	if err := decodeJSON(r, &cfg); err != nil {
		writeErr(w, 400, "invalid JSON")
		return
	}
	if cfg.OpenAIBaseURL != "" {
		if err := llm.ValidateBaseURL(cfg.OpenAIBaseURL); err != nil {
			writeErr(w, 400, err.Error())
			return
		}
	}
	if err := s.Store.PutModelConfig(cfg); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	saved, _ := s.Store.GetModelConfig()
	writeJSON(w, 200, saved.Public())
}

func (s *Server) handleGetUIPrefs(w http.ResponseWriter, r *http.Request) {
	prefs, err := s.Store.GetUIPrefs()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, prefs)
}

func (s *Server) handlePutUIPrefs(w http.ResponseWriter, r *http.Request) {
	var prefs model.UIPrefs
	if err := decodeJSON(r, &prefs); err != nil {
		writeErr(w, 400, "invalid JSON")
		return
	}
	if err := s.Store.PutUIPrefs(prefs); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, prefs)
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.ListProjects()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	// strip project custom keys
	for i := range list {
		if list[i].CustomModelConfig != nil {
			pub := list[i].CustomModelConfig.Public()
			list[i].CustomModelConfig = &pub
		}
	}
	writeJSON(w, 200, list)
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var p model.Project
	if err := decodeJSON(r, &p); err != nil {
		writeErr(w, 400, "invalid JSON")
		return
	}
	if p.ID == "" {
		p.ID = "proj-" + uuid.NewString()[:8]
	}
	if p.BranchPrefixConfig == nil {
		bp := model.DefaultBranchPrefix()
		p.BranchPrefixConfig = &bp
	}
	if err := s.Store.UpsertProject(p); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	saved, _ := s.Store.GetProject(p.ID)
	writeJSON(w, 201, saved)
}

func (s *Server) handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, err := s.Store.GetProject(id)
	if err != nil || existing == nil {
		writeErr(w, 404, "project not found")
		return
	}
	var p model.Project
	if err := decodeJSON(r, &p); err != nil {
		writeErr(w, 400, "invalid JSON")
		return
	}
	p.ID = id
	p.CreatedAt = existing.CreatedAt
	// preserve custom key if blank
	if p.CustomModelConfig != nil && p.CustomModelConfig.OpenAIAPIKey == "" && existing.CustomModelConfig != nil {
		p.CustomModelConfig.OpenAIAPIKey = existing.CustomModelConfig.OpenAIAPIKey
	}
	if err := s.Store.UpsertProject(p); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	saved, _ := s.Store.GetProject(id)
	if saved.CustomModelConfig != nil {
		pub := saved.CustomModelConfig.Public()
		saved.CustomModelConfig = &pub
	}
	writeJSON(w, 200, saved)
}

func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.Store.DeleteProject(id); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) handleListIssues(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("projectId")
	list, err := s.Store.ListIssues(projectID)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, list)
}

func (s *Server) handleGetIssue(w http.ResponseWriter, r *http.Request) {
	iss, err := s.Store.GetIssue(r.PathValue("id"))
	if err != nil || iss == nil {
		writeErr(w, 404, "issue not found")
		return
	}
	writeJSON(w, 200, iss)
}

func (s *Server) handleCreateIssue(w http.ResponseWriter, r *http.Request) {
	var iss model.Issue
	if err := decodeJSON(r, &iss); err != nil {
		writeErr(w, 400, "invalid JSON")
		return
	}
	if iss.ID == "" {
		iss.ID = "issue-" + uuid.NewString()[:8]
	}
	if iss.Status == "" {
		iss.Status = model.StatusRequirements
	}
	if err := s.Store.UpsertIssue(iss); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	saved, _ := s.Store.GetIssue(iss.ID)
	writeJSON(w, 201, saved)
}

func (s *Server) handleUpdateIssue(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, err := s.Store.GetIssue(id)
	if err != nil || existing == nil {
		writeErr(w, 404, "issue not found")
		return
	}
	var iss model.Issue
	if err := decodeJSON(r, &iss); err != nil {
		writeErr(w, 400, "invalid JSON")
		return
	}
	iss.ID = id
	iss.CreatedAt = existing.CreatedAt
	if err := validateStatusTransition(existing, &iss); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if err := s.Store.UpsertIssue(iss); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	saved, _ := s.Store.GetIssue(id)
	writeJSON(w, 200, saved)
}

func validateStatusTransition(old, neu *model.Issue) error {
	if old.Status == neu.Status {
		return nil
	}
	switch neu.Status {
	case model.StatusBacklog:
		if neu.DevSpec == nil || strings.TrimSpace(neu.DevSpec.RawMarkdown) == "" {
			return fmt.Errorf("cannot move to backlog without a Markdown Dev Spec")
		}
		if len(neu.AssociatedRepoIDs) == 0 {
			return fmt.Errorf("cannot move to backlog without associated repositories")
		}
	case model.StatusInReview:
		if neu.PRInfo == nil && old.PRInfo == nil {
			return fmt.Errorf("cannot move to in_review without a completed auto-dev job")
		}
	}
	return nil
}

func (s *Server) handleDeleteIssue(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteIssue(r.PathValue("id")); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) handleValidateRepo(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
	}
	if err := decodeJSON(r, &body); err != nil || strings.TrimSpace(body.Path) == "" {
		writeErr(w, 400, "path required")
		return
	}
	info, err := gitx.ValidateRepo(body.Path)
	if err != nil {
		writeJSON(w, 400, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, 200, info)
}

func (s *Server) resolveModel(projectID string) (model.ModelConfig, error) {
	cfg, err := s.Store.GetModelConfig()
	if err != nil {
		return cfg, err
	}
	if projectID == "" {
		return cfg, nil
	}
	p, err := s.Store.GetProject(projectID)
	if err != nil || p == nil {
		return cfg, nil
	}
	if p.UseCustomModelConfig && p.CustomModelConfig != nil && p.CustomModelConfig.OpenAIAPIKey != "" {
		return *p.CustomModelConfig, nil
	}
	return cfg, nil
}

type chatBody struct {
	Prompt            string             `json:"prompt"`
	Messages          []model.ChatMessage `json:"messages"`
	IssueTitle        string             `json:"issueTitle"`
	IssueDescription  string             `json:"issueDescription"`
	AssociatedRepos  []repoRef          `json:"associatedRepos"`
	GenerateSpec      bool               `json:"generateSpec"`
	ProjectID         string             `json:"projectId"`
	IssueID           string             `json:"issueId"`
}

type repoRef struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	DefaultBranch string `json:"defaultBranch"`
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	var body chatBody
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, 400, "invalid JSON")
		return
	}
	cfg, err := s.resolveModel(body.ProjectID)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}

	var repoLines []string
	for _, repo := range body.AssociatedRepos {
		repoLines = append(repoLines, fmt.Sprintf("%s @ %s (branch %s)", repo.Name, repo.Path, repo.DefaultBranch))
	}

	system := `You are a Senior VibeCoding AI Architect & Staff Software Engineer.
Assist with requirements analysis, architecture, and Development Specification Documents.

Issue Context:
- Title: ` + body.IssueTitle + `
- Description: ` + body.IssueDescription + `
- Associated Local Repositories: ` + strings.Join(repoLines, "; ")

	if body.GenerateSpec {
		system += `

When asked to create/update a Dev Spec, respond with helpful analysis in markdown.
Prefer structured sections: Summary, Architecture, Target Files, Implementation Steps, Test Cases.`
	}

	msgs := make([]llm.ChatMessage, 0, len(body.Messages)+1)
	for _, m := range body.Messages {
		role := "assistant"
		if m.Sender == "user" {
			role = "user"
		} else if m.Sender == "system" {
			role = "system"
		}
		msgs = append(msgs, llm.ChatMessage{Role: role, Content: m.Text})
	}
	if body.Prompt != "" {
		msgs = append(msgs, llm.ChatMessage{Role: "user", Content: body.Prompt})
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()
	chatReq := llm.ChatRequest{
		ModelConfig: cfg,
		System:      system,
		Messages:    msgs,
		Temperature: cfg.Temperature,
	}

	if wantsStream(r) {
		sse, err := newSSE(w)
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		_ = sse.event(map[string]any{"type": "status", "message": "calling_model"})
		text, err := s.LLM.ChatStream(ctx, chatReq, func(delta string) {
			_ = sse.event(map[string]any{"type": "delta", "text": delta})
		})
		if err != nil {
			_ = sse.event(map[string]any{"type": "error", "error": err.Error()})
			return
		}
		_ = sse.event(map[string]any{"type": "done", "text": text})
		return
	}

	text, err := s.LLM.Chat(ctx, chatReq)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"text": text})
}

func (s *Server) handleGenerateSpec(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	issue, err := s.Store.GetIssue(id)
	if err != nil || issue == nil {
		writeErr(w, 404, "issue not found")
		return
	}
	var body struct {
		Prompt   string `json:"prompt"`
		Messages []model.ChatMessage `json:"messages"`
	}
	_ = decodeJSON(r, &body)

	proj, _ := s.Store.GetProject(issue.ProjectID)
	cfg, _ := s.resolveModel(issue.ProjectID)

	var repos []repoRef
	if proj != nil {
		for _, rid := range issue.AssociatedRepoIDs {
			for _, gr := range proj.GitRepos {
				if gr.ID == rid {
					repos = append(repos, repoRef{Name: gr.Name, Path: gr.Path, DefaultBranch: gr.DefaultBranch})
				}
			}
		}
	}

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

	repoDesc := ""
	for _, repo := range repos {
		repoDesc += fmt.Sprintf("- %s path=%s branch=%s\n", repo.Name, repo.Path, repo.DefaultBranch)
	}
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
		issue.Title, issue.Description, repoDesc, prevSpec, body.Prompt)

	// Keep recent chat for context (skip dumping full prior AI markdown into history).
	msgs := make([]llm.ChatMessage, 0, 8)
	start := 0
	if len(body.Messages) > 12 {
		start = len(body.Messages) - 12
	}
	for _, m := range body.Messages[start:] {
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
	msgs = append(msgs, llm.ChatMessage{Role: "user", Content: user})

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()
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

	runSpec := func(stream bool, onDelta func(string)) (string, error) {
		if stream {
			var gotDelta bool
			wrap := func(delta string) {
				if delta != "" {
					gotDelta = true
				}
				if onDelta != nil {
					onDelta(delta)
				}
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

	if wantsStream(r) {
		sse, err := newSSE(w)
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		_ = sse.event(map[string]any{"type": "status", "message": "calling_model"})
		var lastFlush time.Time
		var buf strings.Builder
		text, err := runSpec(true, func(delta string) {
			buf.WriteString(delta)
			// Coalesce tiny chunks so the UI isn't flooded.
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
		spec, chatReply, err := llm.ParseDevSpecJSON(text, issue.Title)
		if err != nil {
			_ = sse.event(map[string]any{"type": "error", "error": err.Error()})
			return
		}
		issue.DevSpec = spec
		_ = s.Store.UpsertIssue(*issue)
		_ = sse.event(map[string]any{
			"type":      "done",
			"spec":      spec,
			"text":      chatReply,
			"chatReply": chatReply,
			"process":   text,
		})
		return
	}

	text, err := runSpec(false, nil)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	spec, chatReply, err := llm.ParseDevSpecJSON(text, issue.Title)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	issue.DevSpec = spec
	_ = s.Store.UpsertIssue(*issue)
	// `process` is the raw model output for ephemeral UI only — not persisted separately.
	writeJSON(w, 200, map[string]any{
		"spec":      spec,
		"text":      chatReply,
		"chatReply": chatReply,
		"process":   text,
	})
}

func (s *Server) handleTestOpenAPI(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OpenAIBaseURL string `json:"openAIBaseUrl"`
		OpenAIAPIKey  string `json:"openAIApiKey"`
		OpenAIModel   string `json:"openAIModel"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, 400, "invalid JSON")
		return
	}
	key := body.OpenAIAPIKey
	if key == "" {
		cfg, _ := s.Store.GetModelConfig()
		key = cfg.OpenAIAPIKey
		if body.OpenAIBaseURL == "" {
			body.OpenAIBaseURL = cfg.OpenAIBaseURL
		}
		if body.OpenAIModel == "" {
			body.OpenAIModel = cfg.OpenAIModel
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	reply, err := s.LLM.TestOpenAPI(ctx, body.OpenAIBaseURL, key, body.OpenAIModel)
	if err != nil {
		writeJSON(w, 400, map[string]any{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"success": true, "message": reply})
}

func (s *Server) handleAutoDevStart(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IssueID string `json:"issueId"`
	}
	if err := decodeJSON(r, &body); err != nil || body.IssueID == "" {
		writeErr(w, 400, "issueId required")
		return
	}
	issue, err := s.Store.GetIssue(body.IssueID)
	if err != nil || issue == nil {
		writeErr(w, 404, "issue not found")
		return
	}
	if issue.DevSpec == nil {
		writeErr(w, 400, "Dev Spec required before Auto-Dev")
		return
	}
	if len(issue.AssociatedRepoIDs) == 0 {
		writeErr(w, 400, "associated repositories required")
		return
	}
	if active, _ := s.Store.ActiveJobForRepoIssue(body.IssueID); active != nil {
		writeErr(w, 409, "auto-dev already running for this issue")
		return
	}

	issue.Status = model.StatusInProgress
	issue.AutoDevProgress = 5
	issue.AutoDevLogs = append(issue.AutoDevLogs, model.AutoDevLog{
		ID:        "log-" + uuid.NewString()[:8],
		Timestamp: time.Now().Format("15:04:05"),
		Phase:     "analyzing",
		Message:   "Starting VibeBot Auto-Dev job...",
	})
	_ = s.Store.UpsertIssue(*issue)

	job, err := s.Store.CreateJob(body.IssueID)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	s.Runner.Start(job.ID)
	writeJSON(w, 202, job)
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	job, err := s.Store.GetJob(r.PathValue("id"))
	if err != nil || job == nil {
		writeErr(w, 404, "job not found")
		return
	}
	logs, _ := s.Store.ListJobLogs(job.ID)
	writeJSON(w, 200, map[string]any{"job": job, "logs": logs})
}

func (s *Server) handleJobEvents(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	job, err := s.Store.GetJob(jobID)
	if err != nil || job == nil {
		writeErr(w, 404, "job not found")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, 500, "streaming unsupported")
		return
	}

	// Replay existing logs
	logs, _ := s.Store.ListJobLogs(jobID)
	for _, log := range logs {
		ev := model.JobEvent{Type: "log", Phase: log.Phase, Message: log.Message, Details: log.Details, Log: &log, Progress: job.Progress}
		b, _ := json.Marshal(ev)
		fmt.Fprintf(w, "data: %s\n\n", b)
	}
	flusher.Flush()

	if job.Status == model.JobCompleted || job.Status == model.JobFailed || job.Status == model.JobCancelled {
		ev := model.JobEvent{Type: "done", Status: string(job.Status), Progress: job.Progress, PRInfo: job.PRInfo, Error: job.Error, Phase: job.Phase}
		b, _ := json.Marshal(ev)
		fmt.Fprintf(w, "data: %s\n\n", b)
		flusher.Flush()
		return
	}

	ch, unsub := s.Hub.Subscribe(jobID)
	defer unsub()

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			b, _ := json.Marshal(ev)
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
			if ev.Type == "done" || ev.Type == "error" {
				return
			}
		case <-time.After(15 * time.Second):
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

func (s *Server) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	job, err := s.Store.GetJob(id)
	if err != nil || job == nil {
		writeErr(w, 404, "job not found")
		return
	}
	ok := s.Hub.Cancel(id)
	job.Status = model.JobCancelled
	job.Error = "cancelled by user"
	_ = s.Store.UpdateJob(job)
	s.Hub.Publish(id, model.JobEvent{Type: "status", Status: string(model.JobCancelled), Message: "cancelled"})
	writeJSON(w, 200, map[string]any{"ok": ok, "job": job})
}

func (s *Server) handleApproveMerge(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	issue, err := s.Store.GetIssue(id)
	if err != nil || issue == nil {
		writeErr(w, 404, "issue not found")
		return
	}
	if issue.PRInfo == nil || issue.PRInfo.BranchName == "" {
		writeErr(w, 400, "no PR/branch to merge")
		return
	}
	proj, err := s.Store.GetProject(issue.ProjectID)
	if err != nil || proj == nil {
		writeErr(w, 404, "project not found")
		return
	}
	var repos []model.GitRepo
	for _, rid := range issue.AssociatedRepoIDs {
		for _, gr := range proj.GitRepos {
			if gr.ID == rid {
				repos = append(repos, gr)
			}
		}
	}
	for _, repo := range repos {
		if err := gitx.MergeBranch(repo.Path, repo.DefaultBranch, issue.PRInfo.BranchName); err != nil {
			writeErr(w, 500, fmt.Sprintf("merge %s: %v", repo.Name, err))
			return
		}
	}
	issue.Status = model.StatusCompleted
	issue.PRInfo.Status = "merged"
	issue.AutoDevLogs = append(issue.AutoDevLogs, model.AutoDevLog{
		ID:        "log-" + uuid.NewString()[:8],
		Timestamp: time.Now().Format("15:04:05"),
		Phase:     "completed",
		Message:   "Developer approved; merged into default branch.",
	})
	issue.UpdatedAt = model.NowISO()
	if err := s.Store.UpsertIssue(*issue); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, issue)
}

package llm

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ymhhh/vibecoding/internal/model"
)

// ParseReqDocJSON extracts a requirement document from model JSON.
func ParseReqDocJSON(raw string, titleFallback string) (*model.ReqDoc, string, error) {
	raw = unwrapJSONObject(raw)
	if raw == "" {
		return nil, "", fmt.Errorf("empty model response")
	}

	var parsed struct {
		ChatReply   string `json:"chatReply"`
		Title       string `json:"title"`
		Summary     string `json:"summary"`
		Scope       string `json:"scope"`
		NonGoals    string `json:"nonGoals"`
		Acceptance  string `json:"acceptance"`
		Constraints string `json:"constraints"`
		RawMarkdown string `json:"rawMarkdown"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		// Models often emit almost-JSON (trailing commas, raw newlines in strings).
		repaired := repairLooseJSON(raw)
		if err2 := json.Unmarshal([]byte(repaired), &parsed); err2 != nil {
			parsed.RawMarkdown, _ = extractJSONStringField(raw, "rawMarkdown")
			parsed.ChatReply, _ = extractJSONStringField(raw, "chatReply")
			parsed.Title, _ = extractJSONStringField(raw, "title")
			parsed.Summary, _ = extractJSONStringField(raw, "summary")
			parsed.Scope, _ = extractJSONStringField(raw, "scope")
			parsed.NonGoals, _ = extractJSONStringField(raw, "nonGoals")
			parsed.Acceptance, _ = extractJSONStringField(raw, "acceptance")
			parsed.Constraints, _ = extractJSONStringField(raw, "constraints")
		}
	}

	title := firstNonEmpty(parsed.Title, titleFallback)
	md := CoerceReqMarkdown(strings.TrimSpace(parsed.RawMarkdown))
	if md == "" || looksLikeReqDocJSONEnvelope(md) {
		md = buildReqMarkdown(title, parsed.Summary, parsed.Scope, parsed.NonGoals, parsed.Acceptance, parsed.Constraints)
	}
	if title != "" && md != "" && !strings.HasPrefix(strings.TrimSpace(md), "#") {
		md = "# " + title + "\n\n" + md
	}
	reply := strings.TrimSpace(parsed.ChatReply)
	if reply == "" {
		reply = "已更新需求文档，请确认后再生成开发设计。"
	}
	return &model.ReqDoc{
		Title:       title,
		Summary:     parsed.Summary,
		Scope:       parsed.Scope,
		NonGoals:    parsed.NonGoals,
		Acceptance:  parsed.Acceptance,
		Constraints: parsed.Constraints,
		RawMarkdown: md,
		UpdatedAt:   model.NowISO(),
	}, reply, nil
}

// CoerceReqMarkdown recovers Markdown when a ReqDoc body accidentally stored the
// model JSON envelope ({"chatReply":...,"rawMarkdown":...}).
func CoerceReqMarkdown(md string) string {
	md = strings.TrimSpace(md)
	if md == "" {
		return ""
	}
	if !looksLikeReqDocJSONEnvelope(md) {
		return md
	}
	unwrapped := unwrapJSONObject(md)
	var parsed struct {
		RawMarkdown string `json:"rawMarkdown"`
	}
	if err := json.Unmarshal([]byte(unwrapped), &parsed); err == nil {
		if out := strings.TrimSpace(parsed.RawMarkdown); out != "" && !looksLikeReqDocJSONEnvelope(out) {
			return out
		}
	}
	if repaired := repairLooseJSON(unwrapped); repaired != unwrapped {
		if err := json.Unmarshal([]byte(repaired), &parsed); err == nil {
			if out := strings.TrimSpace(parsed.RawMarkdown); out != "" && !looksLikeReqDocJSONEnvelope(out) {
				return out
			}
		}
	}
	if out, ok := extractJSONStringField(unwrapped, "rawMarkdown"); ok {
		out = strings.TrimSpace(out)
		if out != "" && !looksLikeReqDocJSONEnvelope(out) {
			return out
		}
	}
	return md
}

func looksLikeReqDocJSONEnvelope(s string) bool {
	t := strings.TrimSpace(s)
	if !strings.HasPrefix(t, "{") {
		return false
	}
	return strings.Contains(t, `"rawMarkdown"`) || strings.Contains(t, `"chatReply"`)
}

// repairLooseJSON applies cheap fixes for common LLM JSON mistakes.
func repairLooseJSON(raw string) string {
	s := strings.TrimSpace(raw)
	// Drop trailing commas before } or ]
	var b strings.Builder
	b.Grow(len(s))
	inString := false
	escape := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inString {
			b.WriteByte(c)
			if escape {
				escape = false
				continue
			}
			if c == '\\' {
				escape = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			b.WriteByte(c)
			continue
		}
		if c == ',' {
			j := i + 1
			for j < len(s) && (s[j] == ' ' || s[j] == '\n' || s[j] == '\r' || s[j] == '\t') {
				j++
			}
			if j < len(s) && (s[j] == '}' || s[j] == ']') {
				continue // skip trailing comma
			}
		}
		b.WriteByte(c)
	}
	return b.String()
}

// extractJSONStringField pulls a string field from imperfect JSON by scanning
// for "key": "value", allowing raw newlines inside the value.
func extractJSONStringField(raw, key string) (string, bool) {
	needle := `"` + key + `"`
	idx := strings.Index(raw, needle)
	if idx < 0 {
		return "", false
	}
	i := idx + len(needle)
	for i < len(raw) && (raw[i] == ' ' || raw[i] == '\n' || raw[i] == '\r' || raw[i] == '\t') {
		i++
	}
	if i >= len(raw) || raw[i] != ':' {
		return "", false
	}
	i++
	for i < len(raw) && (raw[i] == ' ' || raw[i] == '\n' || raw[i] == '\r' || raw[i] == '\t') {
		i++
	}
	if i >= len(raw) || raw[i] != '"' {
		return "", false
	}
	i++ // opening quote
	var out strings.Builder
	for i < len(raw) {
		c := raw[i]
		if c == '\\' && i+1 < len(raw) {
			n := raw[i+1]
			switch n {
			case 'n':
				out.WriteByte('\n')
			case 'r':
				out.WriteByte('\r')
			case 't':
				out.WriteByte('\t')
			case '"', '\\', '/':
				out.WriteByte(n)
			case 'u':
				// Keep \uXXXX as-is if short; best-effort decode 4 hex digits.
				if i+5 < len(raw) {
					hex := raw[i+2 : i+6]
					if r, err := strconv.ParseInt(hex, 16, 32); err == nil {
						out.WriteRune(rune(r))
						i += 6
						continue
					}
				}
				out.WriteByte('\\')
				out.WriteByte(n)
			default:
				out.WriteByte(n)
			}
			i += 2
			continue
		}
		if c == '"' {
			return out.String(), true
		}
		out.WriteByte(c)
		i++
	}
	return "", false
}

func buildReqMarkdown(title, summary, scope, nonGoals, acceptance, constraints string) string {
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(title)
	b.WriteString("\n\n## 概述 / Summary\n")
	b.WriteString(summary)
	b.WriteString("\n\n## 范围 / Scope\n")
	b.WriteString(scope)
	b.WriteString("\n\n## 非目标 / Non-goals\n")
	b.WriteString(nonGoals)
	b.WriteString("\n\n## 验收标准 / Acceptance\n")
	b.WriteString(acceptance)
	b.WriteString("\n\n## 约束 / Constraints\n")
	b.WriteString(constraints)
	b.WriteString("\n")
	return b.String()
}

// BriefToReqMarkdown turns an issue brief (title/description/attachments) into a Markdown ReqDoc body.
func BriefToReqMarkdown(issue *model.Issue) string {
	if issue == nil {
		return "# Requirement\n\n## 概述 / Summary\n\n_(empty)_\n"
	}
	title := strings.TrimSpace(issue.Title)
	if title == "" {
		title = "Requirement"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n## 概述 / Summary\n\n", title)
	desc := strings.TrimSpace(issue.Description)
	if desc == "" {
		b.WriteString("_(待补充)_\n")
	} else {
		b.WriteString(desc)
		b.WriteString("\n")
	}
	hasTextAtt := false
	for _, a := range issue.Attachments {
		if strings.TrimSpace(a.Text) != "" {
			hasTextAtt = true
			break
		}
	}
	if hasTextAtt {
		b.WriteString("\n## 附件原文 / Attached briefs\n\n")
		for _, a := range issue.Attachments {
			text := strings.TrimSpace(a.Text)
			if text == "" {
				continue
			}
			name := strings.TrimSpace(a.Name)
			if name == "" {
				name = "attachment"
			}
			fmt.Fprintf(&b, "### %s\n\n%s\n\n", name, text)
		}
	}
	hasImg := false
	for _, a := range issue.Attachments {
		if a.Kind == "image" {
			hasImg = true
			break
		}
	}
	if hasImg {
		b.WriteString("## 附图 / Images\n\n")
		for _, a := range issue.Attachments {
			if a.Kind != "image" {
				continue
			}
			name := strings.TrimSpace(a.Name)
			if name == "" {
				name = "image"
			}
			fmt.Fprintf(&b, "- %s\n", name)
		}
		b.WriteString("\n")
	}
	b.WriteString(`## 范围 / Scope

_(待补充：本期要做什么)_

## 非目标 / Non-goals

_(待补充：明确不做的内容)_

## 验收标准 / Acceptance

_(待补充：可验证的完成条件)_

## 约束 / Constraints

_(待补充：技术/业务约束)_
`)
	return b.String()
}

// WantedFileRef is Pass-1 model output for files to read.
type WantedFileRef struct {
	RepoName string `json:"repoName"`
	FilePath string `json:"filePath"`
	HintLine int    `json:"hintLine"`
}

// ParseWantedFilesJSON extracts wantedFiles (+ optional chatReply) from Pass-1 JSON.
func ParseWantedFilesJSON(raw string) (files []WantedFileRef, chatReply string, err error) {
	raw = unwrapJSONObject(raw)
	if raw == "" {
		return nil, "", fmt.Errorf("empty model response")
	}
	var parsed struct {
		ChatReply   string          `json:"chatReply"`
		WantedFiles []WantedFileRef `json:"wantedFiles"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, "", fmt.Errorf("parse wantedFiles: %w", err)
	}
	return parsed.WantedFiles, strings.TrimSpace(parsed.ChatReply), nil
}

// ReqSplitResult is a requirement-only split (no Dev Spec per sub).
type ReqSplitResult struct {
	ChatReply        string
	OverviewMarkdown string
	SubRequirements  []model.SubRequirement
}

// ParseReqSplitJSON splits into ordered requirement slices without Dev Specs.
func ParseReqSplitJSON(raw string, titleFallback string) (*ReqSplitResult, error) {
	raw = unwrapJSONObject(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty model response")
	}
	var parsed struct {
		ChatReply        string `json:"chatReply"`
		OverviewMarkdown string `json:"overviewMarkdown"`
		Title            string `json:"title"`
		SubRequirements  []struct {
			ID          string `json:"id"`
			Title       string `json:"title"`
			Description string `json:"description"`
		} `json:"subRequirements"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("parse req split: %w", err)
	}
	if len(parsed.SubRequirements) == 0 {
		return nil, fmt.Errorf("model returned no subRequirements")
	}
	overview := strings.TrimSpace(parsed.OverviewMarkdown)
	title := firstNonEmpty(parsed.Title, titleFallback)
	if overview == "" {
		var b strings.Builder
		b.WriteString("# ")
		b.WriteString(title)
		b.WriteString("\n\n## 子需求拆分\n\n")
		for i, sub := range parsed.SubRequirements {
			t := firstNonEmpty(sub.Title, fmt.Sprintf("Sub %d", i+1))
			b.WriteString(fmt.Sprintf("%d. **%s** — %s\n", i+1, t, sub.Description))
		}
		overview = b.String()
	}
	subs := make([]model.SubRequirement, 0, len(parsed.SubRequirements))
	for i, sub := range parsed.SubRequirements {
		t := firstNonEmpty(sub.Title, fmt.Sprintf("%s (%d)", titleFallback, i+1))
		id := strings.TrimSpace(sub.ID)
		if id == "" {
			id = fmt.Sprintf("sub-%s-%d", shortID(titleFallback), i+1)
		}
		subs = append(subs, model.SubRequirement{
			ID:           id,
			Title:        t,
			Description:  sub.Description,
			Order:        i + 1,
			Status:       model.SubReqPending,
			DevSpec:      nil,
			ChatMessages: []model.ChatMessage{},
			AutoDevLogs:  []model.AutoDevLog{},
		})
	}
	reply := strings.TrimSpace(parsed.ChatReply)
	if reply == "" {
		reply = fmt.Sprintf("已将需求拆分为 %d 条，请先确认需求，再对每条子需求单独生成开发设计。", len(subs))
	}
	return &ReqSplitResult{
		ChatReply:        reply,
		OverviewMarkdown: overview,
		SubRequirements:  subs,
	}, nil
}

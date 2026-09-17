package llm

import (
	"encoding/json"
	"fmt"
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
		// Fallback: treat whole response as markdown requirement.
		return &model.ReqDoc{
			Title:       titleFallback,
			RawMarkdown: strings.TrimSpace(raw),
			UpdatedAt:   model.NowISO(),
		}, "已根据讨论更新需求文档。", nil
	}
	title := firstNonEmpty(parsed.Title, titleFallback)
	md := strings.TrimSpace(parsed.RawMarkdown)
	if md == "" {
		md = buildReqMarkdown(title, parsed.Summary, parsed.Scope, parsed.NonGoals, parsed.Acceptance, parsed.Constraints)
	}
	if title != "" && !strings.HasPrefix(md, "#") {
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

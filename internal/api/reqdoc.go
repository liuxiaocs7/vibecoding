package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/ymhhh/vibecoding/internal/llm"
	"github.com/ymhhh/vibecoding/internal/model"
)

func (s *Server) handleGenerateReqDoc(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	issue, err := s.Store.GetIssue(id)
	if err != nil || issue == nil {
		writeErr(w, 404, "issue not found")
		return
	}
	var body specRequestBody
	_ = decodeJSON(r, &body)

	cfg, _ := s.resolveModel(issue.ProjectID)
	system := `You are a Senior VibeCoding product analyst.
Update the REQUIREMENT document only (what to build). Do NOT write architecture, file lists, or implementation steps.

Return ONLY a JSON object (no markdown fences) with:
- chatReply: short natural-language reply (2-8 sentences)
- rawMarkdown: COMPLETE requirement Markdown with:
  # Title
  ## 概述 / Summary
  ## 范围 / Scope
  ## 非目标 / Non-goals
  ## 验收标准 / Acceptance
  ## 约束 / Constraints
- title, summary, scope, nonGoals, acceptance, constraints

Rules:
- If a previous requirement document is provided, revise in place; keep unrelated sections.
- Stay product-focused. No Target Files, no Implementation Steps.`

	prev := ""
	if issue.ReqDoc != nil {
		prev = issue.ReqDoc.RawMarkdown
	}
	prompt := resumeUserPrompt(body, "请根据当前讨论提炼完整需求文档（目标、范围、非目标、验收标准、约束）。")
	user := fmt.Sprintf(`Issue title: %s
Description: %s

Previous Requirement Markdown (may be empty):
-----
%s
-----

Latest user message:
%s

Update the requirement document and return JSON.`,
		issue.Title, issue.PromptDescription(), prev, prompt)

	msgs := recentChatMsgs(body.Messages)
	msgs = append(msgs, llm.ChatMessage{Role: "user", Content: user})

	s.streamOrCompleteJSON(w, r, cfg, system, msgs, func(text string) (any, error) {
		doc, chatReply, err := llm.ParseReqDocJSON(text, issue.Title)
		if err != nil {
			return nil, err
		}
		// Preserve AcceptedAt only if content unchanged enough — any regen clears accept.
		issue.ReqDoc = doc
		issue.TouchReqDoc()
		issue.DocPhase = model.DocPhaseRequirement
		issue.UpdatedAt = model.NowISO()
		if err := s.Store.UpsertIssue(*issue); err != nil {
			return nil, err
		}
		saved, _ := s.Store.GetIssue(issue.ID)
		if saved != nil {
			issue = saved
		}
		return reqDocDonePayload(issue.ReqDoc, chatReply, text, issue), nil
	})
}

func (s *Server) handleAcceptRequirement(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	issue, err := s.Store.GetIssue(id)
	if err != nil || issue == nil {
		writeErr(w, 404, "issue not found")
		return
	}
	if !issue.HasReqDoc() && !issue.LegacySpecOnly() {
		writeErr(w, 400, "requirement document required before accept")
		return
	}
	if issue.HasReqDoc() {
		issue.AcceptRequirement()
	} else {
		issue.DocPhase = model.DocPhaseDesign
	}
	issue.UpdatedAt = model.NowISO()
	if err := s.Store.UpsertIssue(*issue); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	saved, _ := s.Store.GetIssue(issue.ID)
	writeJSON(w, 200, saved)
}

func reqDocDonePayload(doc *model.ReqDoc, chatReply, process string, issue *model.Issue) map[string]any {
	out := map[string]any{
		"type":      "done",
		"reqDoc":    doc,
		"text":      chatReply,
		"chatReply": chatReply,
		"process":   process,
	}
	if issue != nil {
		out["issue"] = issue
		out["subRequirements"] = issue.SubRequirements
		out["devSpec"] = issue.DevSpec
		out["docPhase"] = issue.DocPhase
	}
	return out
}

func (s *Server) associatedRepos(issue *model.Issue, proj *model.Project) []model.GitRepo {
	if issue == nil || proj == nil {
		return nil
	}
	var out []model.GitRepo
	for _, rid := range issue.AssociatedRepoIDs {
		for _, gr := range proj.GitRepos {
			if gr.ID == rid {
				out = append(out, gr)
			}
		}
	}
	return out
}

func repoRoots(repos []model.GitRepo) map[string]string {
	m := map[string]string{}
	for _, r := range repos {
		m[r.Name] = r.Path
	}
	return m
}

func reqQueryText(issue *model.Issue, sub *model.SubRequirement) string {
	var b strings.Builder
	if issue.ReqDoc != nil {
		b.WriteString(issue.ReqDoc.RawMarkdown)
	} else {
		b.WriteString(issue.PromptDescription())
	}
	if sub != nil {
		b.WriteString("\n\n")
		b.WriteString(sub.Title)
		b.WriteString("\n")
		b.WriteString(sub.Description)
	}
	return b.String()
}

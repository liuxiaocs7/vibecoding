package api

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ymhhh/vibecoding/internal/executor"
	"github.com/ymhhh/vibecoding/internal/gitx"
	"github.com/ymhhh/vibecoding/internal/model"
)

func (s *Server) handleGetExecutor(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.Store.GetExecutorConfig()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, cfg.Normalize())
}

func (s *Server) handlePutExecutor(w http.ResponseWriter, r *http.Request) {
	var cfg model.ExecutorConfig
	if err := decodeJSON(r, &cfg); err != nil {
		writeErr(w, 400, "invalid JSON")
		return
	}
	cfg = cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if err := s.Store.PutExecutorConfig(cfg); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	saved, _ := s.Store.GetExecutorConfig()
	writeJSON(w, 200, saved)
}

func (s *Server) handleListExecutors(w http.ResponseWriter, r *http.Request) {
	mc, _ := s.Store.GetModelConfig()
	configured, _ := model.MaskKey(mc.OpenAIAPIKey)
	writeJSON(w, 200, map[string]any{
		"executors": executor.Probe(executor.ProbeOptions{LLMConfigured: configured}),
		"current":   executor.DisplayName(mustExecutorConfig(s)),
	})
}

func mustExecutorConfig(s *Server) model.ExecutorConfig {
	cfg, _ := s.Store.GetExecutorConfig()
	return cfg.Normalize()
}

type issueDiffResponse struct {
	IssueID    string           `json:"issueId"`
	BranchName string           `json:"branchName"`
	BaseBranch string           `json:"baseBranch,omitempty"`
	Executor   string           `json:"executor,omitempty"`
	Quality    *model.QualityGate `json:"quality,omitempty"`
	Repos      []repoDiff       `json:"repos"`
}

type repoDiff struct {
	RepoID     string            `json:"repoId"`
	RepoName   string            `json:"repoName"`
	BaseBranch string            `json:"baseBranch"`
	Stats      model.DiffStats   `json:"stats"`
	Ahead      int               `json:"ahead"`
	Behind     int               `json:"behind"`
	Files      []gitx.DiffFile   `json:"files"`
	Commits    []gitx.DiffCommit `json:"commits"`
	Error      string            `json:"error,omitempty"`
}

func (s *Server) handleIssueDiff(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	issue, err := s.Store.GetIssue(id)
	if err != nil || issue == nil {
		writeErr(w, 404, "issue not found")
		return
	}
	if issue.PRInfo == nil || strings.TrimSpace(issue.PRInfo.BranchName) == "" {
		writeErr(w, 400, "issue has no Auto-Dev branch yet")
		return
	}
	proj, err := s.Store.GetProject(issue.ProjectID)
	if err != nil || proj == nil {
		writeErr(w, 404, "project not found")
		return
	}
	resp := issueDiffResponse{
		IssueID:    issue.ID,
		BranchName: issue.PRInfo.BranchName,
		BaseBranch: issue.PRInfo.BaseBranch,
		Executor:   issue.PRInfo.Executor,
		Quality:    issue.PRInfo.Quality,
	}
	for _, rid := range issue.AssociatedRepoIDs {
		for _, gr := range proj.GitRepos {
			if gr.ID != rid {
				continue
			}
			base := issue.PRInfo.BaseBranch
			if base == "" {
				base = gr.DefaultBranch
			}
			stats, files, commits, ahead, behind, derr := gitx.DiffBetween(gr.Path, base, issue.PRInfo.BranchName)
			rd := repoDiff{
				RepoID:     gr.ID,
				RepoName:   gr.Name,
				BaseBranch: base,
				Stats:      stats,
				Ahead:      ahead,
				Behind:     behind,
				Files:      files,
				Commits:    commits,
			}
			if derr != nil {
				rd.Error = derr.Error()
			}
			resp.Repos = append(resp.Repos, rd)
		}
	}
	writeJSON(w, 200, resp)
}

type openEditorBody struct {
	App    string `json:"app"` // cursor | vscode
	RepoID string `json:"repoId,omitempty"`
}

func (s *Server) handleOpenEditor(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	issue, err := s.Store.GetIssue(id)
	if err != nil || issue == nil {
		writeErr(w, 404, "issue not found")
		return
	}
	if issue.PRInfo == nil || len(issue.PRInfo.Worktrees) == 0 {
		writeErr(w, 400, "no worktree available; run Auto-Dev first")
		return
	}
	var body openEditorBody
	_ = decodeJSON(r, &body)
	app := strings.ToLower(strings.TrimSpace(body.App))
	if app == "" {
		app = "cursor"
	}
	bin := ""
	switch app {
	case "cursor":
		bin = "cursor"
	case "vscode", "code":
		bin = "code"
		app = "vscode"
	default:
		writeErr(w, 400, "app must be cursor or vscode")
		return
	}
	if _, err := exec.LookPath(bin); err != nil {
		writeErr(w, 400, fmt.Sprintf("%s CLI not found on PATH", bin))
		return
	}
	target := issue.PRInfo.Worktrees[0]
	if body.RepoID != "" {
		found := false
		for _, wt := range issue.PRInfo.Worktrees {
			if wt.RepoID == body.RepoID {
				target = wt
				found = true
				break
			}
		}
		if !found {
			writeErr(w, 400, "repoId not found in worktrees")
			return
		}
	}
	if st, err := os.Stat(target.Path); err != nil || !st.IsDir() {
		writeErr(w, 400, "worktree path missing on disk")
		return
	}
	cmd := exec.Command(bin, target.Path)
	if err := cmd.Start(); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	_ = cmd.Process.Release()
	writeJSON(w, 200, map[string]any{"ok": true, "app": app, "path": target.Path})
}

// approveMergeSafe merges via MergeBranchAt and cleans worktrees.
func (s *Server) approveMergeSafe(issue *model.Issue, repos []model.GitRepo) error {
	branch := issue.PRInfo.BranchName
	for _, repo := range repos {
		base := issue.PRInfo.BaseBranch
		if base == "" {
			base = repo.DefaultBranch
		}
		if err := gitx.MergeBranchAt(repo.Path, base, branch); err != nil {
			return fmt.Errorf("%s: %w", repo.Name, err)
		}
	}
	for _, wt := range issue.PRInfo.Worktrees {
		for _, repo := range repos {
			if repo.ID == wt.RepoID || repo.Name == wt.RepoName {
				_ = gitx.RemoveWorktree(repo.Path, wt.Path)
			}
		}
	}
	issue.PRInfo.Worktrees = nil
	issue.PRInfo.Status = "merged"
	issue.Status = model.StatusCompleted
	issue.AutoDevLogs = append(issue.AutoDevLogs, model.AutoDevLog{
		ID:        "log-" + uuid.NewString()[:8],
		Timestamp: time.Now().Format("15:04:05"),
		Phase:     "completed",
		Message:   "Developer approved; merged into default branch (worktrees removed).",
	})
	issue.UpdatedAt = model.NowISO()
	return s.Store.UpsertIssue(*issue)
}

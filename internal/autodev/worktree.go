package autodev

import (
	"fmt"
	"path/filepath"

	"github.com/ymhhh/vibecoding/internal/gitx"
	"github.com/ymhhh/vibecoding/internal/model"
)

// worktreeSession binds a main repo to its isolated worktree for one Auto-Dev job.
type worktreeSession struct {
	Repo     model.GitRepo // Path points at the worktree
	MainPath string
	Base     string
	Branch   string
	WTPath   string
}

func (r *Runner) worktreeRootOrDefault() string {
	if r != nil && r.WorktreeRoot != "" {
		return r.WorktreeRoot
	}
	return filepath.Join(".", "worktrees")
}

func (r *Runner) worktreePath(issueID, repoID string) string {
	return filepath.Join(r.worktreeRootOrDefault(), issueID, repoID)
}

// prepareWorktrees creates linked worktrees under WorktreeRoot without touching the
// main working tree checkout. Repo metadata ops are briefly locked per main path.
func (r *Runner) prepareWorktrees(repos []model.GitRepo, issueID, branchName string) ([]worktreeSession, string, error) {
	sessions := make([]worktreeSession, 0, len(repos))
	baseBranch := ""
	for _, repo := range repos {
		unlock := r.Hub.lockRepo(repo.Path)
		base, inited, err := gitx.EnsureInitialCommit(repo.Path, repo.DefaultBranch)
		if err != nil {
			unlock()
			return nil, "", fmt.Errorf("repo %s: %w", repo.Name, err)
		}
		wtPath := r.worktreePath(issueID, repo.ID)
		if err := gitx.AddWorktree(repo.Path, wtPath, base, branchName); err != nil {
			unlock()
			return nil, "", fmt.Errorf("worktree %s: %w", repo.Name, err)
		}
		unlock()

		if baseBranch == "" {
			baseBranch = base
		}
		wtRepo := repo
		wtRepo.Path = wtPath
		sessions = append(sessions, worktreeSession{
			Repo:     wtRepo,
			MainPath: repo.Path,
			Base:     base,
			Branch:   branchName,
			WTPath:   wtPath,
		})
		_ = inited
	}
	return sessions, baseBranch, nil
}

func sessionsToRepos(sessions []worktreeSession) []model.GitRepo {
	out := make([]model.GitRepo, len(sessions))
	for i, s := range sessions {
		out[i] = s.Repo
	}
	return out
}

func sessionsToRefs(sessions []worktreeSession) []model.WorktreeRef {
	out := make([]model.WorktreeRef, len(sessions))
	for i, s := range sessions {
		out[i] = model.WorktreeRef{
			RepoID:   s.Repo.ID,
			RepoName: s.Repo.Name,
			Path:     s.WTPath,
		}
	}
	return out
}

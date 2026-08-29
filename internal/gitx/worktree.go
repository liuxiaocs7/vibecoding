package gitx

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AddWorktree creates (or reuses) a linked worktree at worktreePath on branch,
// rooted at base. The main working tree HEAD and dirty state are left unchanged.
//
// Behavior:
//   - If worktreePath already exists as a worktree for branch, it is reused.
//   - If branch does not exist: git worktree add -B branch worktreePath base
//   - If branch exists without a worktree: git worktree add worktreePath branch
func AddWorktree(repoPath, worktreePath, base, branch string) error {
	repoPath = strings.TrimSpace(repoPath)
	worktreePath = strings.TrimSpace(worktreePath)
	branch = strings.TrimSpace(branch)
	if repoPath == "" || worktreePath == "" || branch == "" {
		return fmt.Errorf("repoPath, worktreePath, and branch are required")
	}
	baseResolved, err := ResolveBaseBranch(repoPath, base)
	if err != nil {
		return err
	}
	if abs, err := filepath.Abs(worktreePath); err == nil {
		worktreePath = abs
	}
	if existing, ok := worktreePathForBranch(repoPath, branch); ok {
		existingAbs, _ := filepath.Abs(existing)
		if samePath(existingAbs, worktreePath) {
			return nil
		}
		// Branch already checked out elsewhere — remove stale registration if path missing.
		if _, err := os.Stat(existing); err != nil {
			_, _ = run(repoPath, "worktree", "prune")
		} else {
			return fmt.Errorf("branch %q already checked out at %s", branch, existing)
		}
	}
	if st, err := os.Stat(worktreePath); err == nil {
		if !st.IsDir() {
			return fmt.Errorf("worktree path exists and is not a directory: %s", worktreePath)
		}
		// Directory exists: try to treat as reusable worktree for this branch.
		if cur, err := CurrentBranch(worktreePath); err == nil && cur == branch {
			return nil
		}
		return fmt.Errorf("worktree path already exists: %s", worktreePath)
	}
	if err := os.MkdirAll(filepath.Dir(worktreePath), 0o755); err != nil {
		return fmt.Errorf("create worktree parent: %w", err)
	}
	if branchExists(repoPath, branch) {
		if _, err := run(repoPath, "worktree", "add", worktreePath, branch); err != nil {
			return fmt.Errorf("worktree add: %w", err)
		}
		return nil
	}
	if _, err := run(repoPath, "worktree", "add", "-B", branch, worktreePath, baseResolved); err != nil {
		return fmt.Errorf("worktree add -B: %w", err)
	}
	return nil
}


func samePath(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if a == b {
		return true
	}
	if ea, err := filepath.EvalSymlinks(a); err == nil {
		a = ea
	}
	if eb, err := filepath.EvalSymlinks(b); err == nil {
		b = eb
	}
	return a == b
}

func worktreePathForBranch(repoPath, branch string) (string, bool) {
	out, err := run(repoPath, "worktree", "list", "--porcelain")
	if err != nil {
		return "", false
	}
	var path, headBranch string
	flush := func() {
		path, headBranch = "", ""
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if headBranch == branch && path != "" {
				return path, true
			}
			flush()
			continue
		}
		if strings.HasPrefix(line, "worktree ") {
			path = strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
		} else if strings.HasPrefix(line, "branch ") {
			ref := strings.TrimSpace(strings.TrimPrefix(line, "branch "))
			headBranch = strings.TrimPrefix(ref, "refs/heads/")
		}
	}
	if headBranch == branch && path != "" {
		return path, true
	}
	return "", false
}

// RemoveWorktree removes a linked worktree. Missing paths are pruned away.
func RemoveWorktree(repoPath, worktreePath string) error {
	repoPath = strings.TrimSpace(repoPath)
	worktreePath = strings.TrimSpace(worktreePath)
	if repoPath == "" || worktreePath == "" {
		return fmt.Errorf("repoPath and worktreePath are required")
	}
	if abs, err := filepath.Abs(worktreePath); err == nil {
		worktreePath = abs
	}
	if _, err := os.Stat(worktreePath); err != nil {
		_, _ = run(repoPath, "worktree", "prune")
		return nil
	}
	if _, err := run(repoPath, "worktree", "remove", "--force", worktreePath); err != nil {
		// Best-effort cleanup when remove fails (e.g. already unregistered).
		_ = os.RemoveAll(worktreePath)
		_, _ = run(repoPath, "worktree", "prune")
		if _, err2 := os.Stat(worktreePath); err2 == nil {
			return fmt.Errorf("worktree remove: %w", err)
		}
	}
	return nil
}

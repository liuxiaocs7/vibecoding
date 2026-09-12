package gitx

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RebaseConflictError is returned when rebase stops with unresolved paths.
// The worktree is left in the conflicted rebase state (not aborted).
type RebaseConflictError struct {
	Files  []string
	Detail string
}

func (e *RebaseConflictError) Error() string {
	if e == nil {
		return "rebase conflict"
	}
	if len(e.Files) > 0 {
		return fmt.Sprintf("rebase conflict: %s", strings.Join(e.Files, ", "))
	}
	if strings.TrimSpace(e.Detail) != "" {
		return "rebase conflict: " + e.Detail
	}
	return "rebase conflict"
}

// RebaseOnto rebases the current branch at worktreePath onto base.
func RebaseOnto(worktreePath, base string) error {
	worktreePath = strings.TrimSpace(worktreePath)
	if worktreePath == "" {
		return fmt.Errorf("worktree path required")
	}
	baseResolved, err := ResolveBaseBranch(worktreePath, base)
	if err != nil {
		return err
	}
	_, err = run(worktreePath, "rebase", baseResolved)
	if err != nil {
		files := conflictedPaths(worktreePath)
		if len(files) > 0 || rebaseInProgress(worktreePath) {
			return &RebaseConflictError{Files: files, Detail: err.Error()}
		}
		return err
	}
	return nil
}

func conflictedPaths(dir string) []string {
	out, err := run(dir, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil
	}
	var files []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files
}

func rebaseInProgress(dir string) bool {
	for _, name := range []string{"rebase-merge", "rebase-apply"} {
		p, err := run(dir, "rev-parse", "--git-path", name)
		if err != nil {
			continue
		}
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return true
		}
	}
	return false
}

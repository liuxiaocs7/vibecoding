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
//   - If worktreePath exists but is stale (leftover dir, detached HEAD, wrong
//     branch), it is switched back or replaced so a rework / re-run can proceed.
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
		reused, err := reuseOrReplaceWorktree(repoPath, worktreePath, branch)
		if err != nil {
			return err
		}
		if reused {
			return nil
		}
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

// reuseOrReplaceWorktree recovers a path left behind by a previous Auto-Dev run.
// It returns reused=true when the existing directory is now a worktree on branch.
func reuseOrReplaceWorktree(repoPath, worktreePath, branch string) (bool, error) {
	if cur, err := CurrentBranch(worktreePath); err == nil && cur == branch {
		return true, nil
	}
	if insideWorkTree(worktreePath) && branchExists(worktreePath, branch) {
		if _, err := run(worktreePath, "checkout", branch); err == nil {
			if cur, err := CurrentBranch(worktreePath); err == nil && cur == branch {
				return true, nil
			}
		}
	}
	if err := RemoveWorktree(repoPath, worktreePath); err != nil {
		return false, fmt.Errorf("replace stale worktree path %s: %w", worktreePath, err)
	}
	return false, nil
}

func insideWorkTree(dir string) bool {
	out, err := run(dir, "rev-parse", "--is-inside-work-tree")
	return err == nil && strings.TrimSpace(out) == "true"
}

// EnforceFeatureBranch undoes an agent checking out / committing on the base
// branch. Commits that landed on base are kept on feature, then base is restored
// to baseSHABefore. The worktree is left on feature.
func EnforceFeatureBranch(mainPath, worktreePath, base, feature, baseSHABefore string) (relocated bool, err error) {
	mainPath = strings.TrimSpace(mainPath)
	worktreePath = strings.TrimSpace(worktreePath)
	feature = strings.TrimSpace(feature)
	baseSHABefore = strings.TrimSpace(baseSHABefore)
	if mainPath == "" || worktreePath == "" || feature == "" || baseSHABefore == "" {
		return false, fmt.Errorf("mainPath, worktreePath, feature, and baseSHABefore are required")
	}
	baseResolved, err := ResolveBaseBranch(mainPath, base)
	if err != nil {
		return false, err
	}
	if baseResolved == feature {
		return false, fmt.Errorf("feature branch %q must not be the base branch", feature)
	}
	baseRef := "refs/heads/" + baseResolved
	curBase, err := RefSHA(mainPath, baseRef)
	if err != nil {
		return false, fmt.Errorf("read base %s: %w", baseResolved, err)
	}

	if curBase != baseSHABefore {
		if err := checkoutFeatureAt(worktreePath, feature, curBase); err != nil {
			return false, fmt.Errorf("move commits onto %s: %w", feature, err)
		}
		if _, err := run(mainPath, "update-ref", baseRef, baseSHABefore); err != nil {
			return false, fmt.Errorf("restore base branch %s: %w", baseResolved, err)
		}
		relocated = true
	} else if cur, err := CurrentBranch(worktreePath); err != nil || cur != feature {
		tip, tipErr := RefSHA(worktreePath, "HEAD")
		if tipErr != nil {
			return false, fmt.Errorf("read worktree HEAD: %w", tipErr)
		}
		if err := checkoutFeatureAt(worktreePath, feature, tip); err != nil {
			return false, fmt.Errorf("return worktree to %s: %w", feature, err)
		}
	}

	cur, err := CurrentBranch(worktreePath)
	if err != nil || cur != feature {
		return relocated, fmt.Errorf("worktree not on feature branch %s (on %q)", feature, cur)
	}
	gotBase, err := RefSHA(mainPath, baseRef)
	if err != nil {
		return relocated, err
	}
	if gotBase != baseSHABefore {
		return relocated, fmt.Errorf("base branch %s still moved after restore", baseResolved)
	}
	return relocated, nil
}

func checkoutFeatureAt(worktreePath, feature, tip string) error {
	_, err := run(worktreePath, "checkout", "-B", feature, tip)
	return err
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

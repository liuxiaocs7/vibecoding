package gitx

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ymhhh/vibecoding/internal/model"
)

// MergeBranchAt merges feature into base without unconditionally checking out
// the user's main working tree when it is dirty or on another branch.
//
// Rules:
//  1. Main tree on base and clean → merge in place.
//  2. Main tree on base but dirty → refuse (no forced checkout).
//  3. Main tree not on base → temporary worktree on base, merge there, remove it.
func MergeBranchAt(repoPath, base, feature string) error {
	repoPath = strings.TrimSpace(repoPath)
	feature = strings.TrimSpace(feature)
	if repoPath == "" || feature == "" {
		return fmt.Errorf("repoPath and feature branch are required")
	}
	baseResolved, err := ResolveBaseBranch(repoPath, base)
	if err != nil {
		return err
	}
	if !branchExists(repoPath, feature) {
		return fmt.Errorf("feature branch %q does not exist", feature)
	}
	cur, err := CurrentBranch(repoPath)
	if err != nil {
		return err
	}
	dirty, err := IsDirty(repoPath)
	if err != nil {
		return err
	}
	if cur == baseResolved {
		if dirty {
			return fmt.Errorf("default branch %q has uncommitted changes; commit or stash before merge", baseResolved)
		}
		if _, err := run(repoPath, "merge", "--no-ff", "-m", "Merge "+feature+" via Vibecoding", feature); err != nil {
			return err
		}
		return nil
	}
	// Attach a disposable worktree to the existing base branch (not detached),
	// so the merge advances refs/heads/<base>. Base is free because main tree is elsewhere.
	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("vibecoding-merge-%d", os.Getpid()))
	_ = os.RemoveAll(tmp)
	if _, err := run(repoPath, "worktree", "add", tmp, baseResolved); err != nil {
		return fmt.Errorf("cannot merge without checking out %q (main tree is on %q): %w", baseResolved, cur, err)
	}
	defer func() { _ = RemoveWorktree(repoPath, tmp) }()
	if _, err := run(tmp, "merge", "--no-ff", "-m", "Merge "+feature+" via Vibecoding", feature); err != nil {
		return err
	}
	return nil
}

// DiffBetween returns unified file patches and totals for base...branch (triple-dot).
func DiffBetween(repoPath, base, branch string) (stats model.DiffStats, files []DiffFile, commits []DiffCommit, ahead, behind int, err error) {
	baseResolved, err := ResolveBaseBranch(repoPath, base)
	if err != nil {
		return stats, nil, nil, 0, 0, err
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return stats, nil, nil, 0, 0, fmt.Errorf("branch is required")
	}
	countOut, err := run(repoPath, "rev-list", "--left-right", "--count", baseResolved+"..."+branch)
	if err == nil {
		parts := strings.Fields(strings.TrimSpace(countOut))
		if len(parts) == 2 {
			behind, _ = strconv.Atoi(parts[0])
			ahead, _ = strconv.Atoi(parts[1])
		}
	}
	logOut, _ := run(repoPath, "log", "--format=%H%x09%s%x09%cI", baseResolved+"..."+branch)
	for _, line := range strings.Split(strings.TrimSpace(logOut), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 2 {
			continue
		}
		c := DiffCommit{SHA: parts[0], Subject: parts[1]}
		if len(parts) > 2 {
			c.Date = parts[2]
		}
		commits = append(commits, c)
	}
	numOut, err := run(repoPath, "diff", "--numstat", baseResolved+"..."+branch)
	if err != nil {
		return stats, nil, commits, ahead, behind, err
	}
	type fileMeta struct {
		path       string
		additions  int
		deletions  int
		status     string
	}
	metas := map[string]*fileMeta{}
	var order []string
	for _, line := range strings.Split(strings.TrimSpace(numOut), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		a, _ := strconv.Atoi(parts[0])
		d, _ := strconv.Atoi(parts[1])
		path := parts[2]
		if parts[0] == "-" { // binary
			a, d = 0, 0
		}
		stats.Additions += a
		stats.Deletions += d
		stats.FilesChanged++
		metas[path] = &fileMeta{path: path, additions: a, deletions: d, status: "M"}
		order = append(order, path)
	}
	nameOut, _ := run(repoPath, "diff", "--name-status", baseResolved+"..."+branch)
	for _, line := range strings.Split(strings.TrimSpace(nameOut), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		status, path := fields[0], fields[len(fields)-1]
		if m, ok := metas[path]; ok {
			if len(status) > 0 {
				m.status = string(status[0])
			}
		}
	}
	const maxPatch = 200 * 1024
	for _, path := range order {
		m := metas[path]
		f := DiffFile{
			Path:      m.path,
			Status:    m.status,
			Additions: m.additions,
			Deletions: m.deletions,
		}
		patch, perr := run(repoPath, "diff", baseResolved+"..."+branch, "--", path)
		if perr == nil {
			if len(patch) > maxPatch {
				f.Truncated = true
			} else {
				f.Patch = patch
			}
		}
		files = append(files, f)
	}
	return stats, files, commits, ahead, behind, nil
}

// DiffFile is one path in a base...branch diff.
type DiffFile struct {
	Path      string `json:"path"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Patch     string `json:"patch,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
}

// DiffCommit is a commit on branch not in base.
type DiffCommit struct {
	SHA     string `json:"sha"`
	Subject string `json:"subject"`
	Date    string `json:"date,omitempty"`
}

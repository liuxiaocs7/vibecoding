package gitx

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ymhhh/vibecoding/internal/model"
)

func ValidateRepo(path string) (map[string]any, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("path not found: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory")
	}
	gitDir := filepath.Join(abs, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		return nil, fmt.Errorf("not a git repository (missing .git)")
	}
	branch, err := run(abs, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil || strings.TrimSpace(branch) == "" || strings.TrimSpace(branch) == "HEAD" {
		if b2, err2 := run(abs, "symbolic-ref", "--short", "HEAD"); err2 == nil {
			branch = b2
		}
	}
	branch = strings.TrimSpace(branch)
	dirty, _ := IsDirty(abs)
	count := countFiles(abs)
	hasCommits := HasCommits(abs)
	defaultBranch := branch
	if hasCommits {
		if b, err := ResolveBaseBranch(abs, branch); err == nil {
			defaultBranch = b
		}
	}
	out := map[string]any{
		"ok":            true,
		"path":          abs,
		"currentBranch": branch,
		"defaultBranch": defaultBranch,
		"dirty":         dirty,
		"filesCount":    count,
		"hasCommits":    hasCommits,
	}
	if !hasCommits {
		out["warning"] = "repository has no commits yet; Auto-Dev will create an initial commit automatically"
	}
	return out, nil
}

// HasCommits reports whether HEAD resolves to a commit.
func HasCommits(dir string) bool {
	_, err := run(dir, "rev-parse", "--verify", "HEAD")
	return err == nil
}

// EnsureInitialCommit makes an empty repo usable as an Auto-Dev base.
// When there are no commits yet, it points HEAD at preferred (or main) and
// creates an empty initial commit. Returns the base branch and whether init ran.
func EnsureInitialCommit(dir, preferred string) (base string, initialized bool, err error) {
	if HasCommits(dir) {
		base, err = ResolveBaseBranch(dir, preferred)
		return base, false, err
	}
	branch := strings.TrimSpace(preferred)
	if branch == "" || branch == "HEAD" {
		if out, e := run(dir, "symbolic-ref", "--short", "HEAD"); e == nil {
			branch = strings.TrimSpace(out)
		}
	}
	if branch == "" || branch == "HEAD" {
		branch = "main"
	}
	if _, err := run(dir, "symbolic-ref", "HEAD", "refs/heads/"+branch); err != nil {
		return "", false, fmt.Errorf("set default branch %q: %w", branch, err)
	}
	cmd := exec.Command(
		"git",
		"-c", "user.name=VibeBot",
		"-c", "user.email=vibebot@local",
		"commit", "--allow-empty",
		"-m", "chore: initial commit (auto-created by Vibecoding)",
	)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", false, fmt.Errorf("git initial commit: %s", msg)
	}
	return branch, true, nil
}

func branchExists(dir, name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || name == "HEAD" {
		return false
	}
	_, err := run(dir, "show-ref", "--verify", "--quiet", "refs/heads/"+name)
	return err == nil
}

// ResolveBaseBranch picks a real local base branch for Auto-Dev / merge.
// preferred is the project-configured defaultBranch when set.
func ResolveBaseBranch(dir, preferred string) (string, error) {
	if !HasCommits(dir) {
		return "", fmt.Errorf("repository has no commits yet; create an initial commit on the default branch before Auto-Dev")
	}
	preferred = strings.TrimSpace(preferred)
	if preferred != "" && branchExists(dir, preferred) {
		return preferred, nil
	}
	if cur, err := CurrentBranch(dir); err == nil && branchExists(dir, cur) {
		return cur, nil
	}
	if out, err := run(dir, "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD"); err == nil {
		ref := strings.TrimSpace(out) // refs/remotes/origin/main
		if i := strings.LastIndex(ref, "/"); i >= 0 {
			name := ref[i+1:]
			if branchExists(dir, name) {
				return name, nil
			}
		}
	}
	for _, name := range []string{"main", "master", "develop", "trunk"} {
		if branchExists(dir, name) {
			return name, nil
		}
	}
	out, err := run(dir, "for-each-ref", "--format=%(refname:short)", "--count=1", "refs/heads/")
	if err == nil {
		if name := strings.TrimSpace(out); name != "" {
			return name, nil
		}
	}
	if preferred != "" {
		return "", fmt.Errorf("configured default branch %q does not exist locally (and no fallback branch found)", preferred)
	}
	return "", fmt.Errorf("no local branch found to use as Auto-Dev base")
}

func countFiles(root string) int {
	n := 0
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if name == ".git" || name == "node_modules" || name == "vendor" || name == "dist" || name == ".next" {
				return filepath.SkipDir
			}
			return nil
		}
		n++
		if n > 5000 {
			return filepath.SkipAll
		}
		return nil
	})
	return n
}

func run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return stdout.String(), nil
}

func IsDirty(dir string) (bool, error) {
	out, err := run(dir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// hasTrackedChanges reports staged/unstaged changes to tracked files (ignores untracked).
func hasTrackedChanges(dir string) (bool, error) {
	out, err := run(dir, "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func CheckoutBranch(dir, defaultBranch, newBranch string) error {
	// Ignore untracked files so empty repos with local files can still branch.
	dirty, err := hasTrackedChanges(dir)
	if err != nil {
		return err
	}
	if dirty {
		return fmt.Errorf("working tree is dirty in %s; commit or stash changes first", dir)
	}
	base, _, err := EnsureInitialCommit(dir, defaultBranch)
	if err != nil {
		return err
	}
	// Ensure we are on base branch tip.
	if _, err := run(dir, "checkout", base); err != nil {
		return fmt.Errorf("checkout base branch %q: %w", base, err)
	}
	if _, err := run(dir, "checkout", "-b", newBranch); err != nil {
		// maybe exists — checkout existing
		if _, err2 := run(dir, "checkout", newBranch); err2 != nil {
			return err
		}
	}
	return nil
}

func ApplyFileWrites(dir string, changes []model.SpecFileChange, repoName string) (int, error) {
	applied := 0
	for _, ch := range changes {
		if ch.RepoName != "" && ch.RepoName != repoName {
			continue
		}
		rel := filepath.Clean(ch.FilePath)
		if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			return applied, fmt.Errorf("unsafe file path: %s", ch.FilePath)
		}
		full := filepath.Join(dir, rel)
		switch ch.Action {
		case "delete":
			if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
				return applied, err
			}
			applied++
		case "create", "modify", "":
			if ch.ModifiedCode == "" && ch.Action != "delete" {
				// skip empty patches; coding phase should fill ModifiedCode
				continue
			}
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return applied, err
			}
			if err := os.WriteFile(full, []byte(ch.ModifiedCode), 0o644); err != nil {
				return applied, err
			}
			applied++
		}
	}
	return applied, nil
}

func ApplyUnifiedDiff(dir, diff string) error {
	if strings.TrimSpace(diff) == "" {
		return fmt.Errorf("empty diff")
	}
	cmd := exec.Command("git", "apply", "--whitespace=nowarn", "-")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(diff)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("git apply: %s", msg)
	}
	return nil
}

func CommitAll(dir, message string) error {
	if _, err := run(dir, "add", "-A"); err != nil {
		return err
	}
	dirty, err := IsDirty(dir)
	if err != nil {
		return err
	}
	if !dirty {
		return fmt.Errorf("nothing to commit")
	}
	cmd := exec.Command("git", "-c", "user.name=VibeBot", "-c", "user.email=vibebot@local", "commit", "-m", message)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("git commit: %s", msg)
	}
	return nil
}

func DiffStats(dir, baseBranch string) (model.DiffStats, error) {
	if base, err := ResolveBaseBranch(dir, baseBranch); err == nil {
		baseBranch = base
	} else if strings.TrimSpace(baseBranch) == "" {
		baseBranch = "HEAD~1"
	}
	out, err := run(dir, "diff", "--numstat", baseBranch+"...HEAD")
	if err != nil {
		// fallback: unstaged+staged vs HEAD~0 — use against merge-base failure: show working tree
		out, err = run(dir, "diff", "--numstat", "HEAD~1...HEAD")
		if err != nil {
			return model.DiffStats{}, nil
		}
	}
	stats := model.DiffStats{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		a, _ := strconv.Atoi(parts[0])
		d, _ := strconv.Atoi(parts[1])
		stats.Additions += a
		stats.Deletions += d
		stats.FilesChanged++
	}
	return stats, nil
}

func MergeBranch(dir, defaultBranch, featureBranch string) error {
	dirty, err := IsDirty(dir)
	if err != nil {
		return err
	}
	if dirty {
		return fmt.Errorf("working tree is dirty; cannot merge")
	}
	base, err := ResolveBaseBranch(dir, defaultBranch)
	if err != nil {
		return err
	}
	if _, err := run(dir, "checkout", base); err != nil {
		return fmt.Errorf("checkout base branch %q: %w", base, err)
	}
	if _, err := run(dir, "merge", "--no-ff", "-m", "Merge "+featureBranch+" via Vibecoding", featureBranch); err != nil {
		return err
	}
	return nil
}

func CurrentBranch(dir string) (string, error) {
	out, err := run(dir, "rev-parse", "--abbrev-ref", "HEAD")
	return strings.TrimSpace(out), err
}

func HeadSHA(dir string) (string, error) {
	out, err := run(dir, "rev-parse", "--short", "HEAD")
	return strings.TrimSpace(out), err
}

func ListTrackedFiles(dir string, limit int) ([]string, error) {
	out, err := run(dir, "ls-files")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		files = append(files, line)
		if limit > 0 && len(files) >= limit {
			break
		}
	}
	return files, nil
}

func ReadFile(dir, rel string) (string, error) {
	rel = filepath.Clean(rel)
	if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("unsafe path")
	}
	b, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

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

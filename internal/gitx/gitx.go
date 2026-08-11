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

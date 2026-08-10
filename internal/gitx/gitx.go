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
	return map[string]any{
		"ok":            true,
		"path":          abs,
		"currentBranch": branch,
		"dirty":         dirty,
		"filesCount":    count,
	}, nil
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

func CheckoutBranch(dir, defaultBranch, newBranch string) error {
	dirty, err := IsDirty(dir)
	if err != nil {
		return err
	}
	if dirty {
		return fmt.Errorf("working tree is dirty in %s; commit or stash changes first", dir)
	}
	if defaultBranch == "" {
		defaultBranch = "main"
	}
	// Ensure we are on default branch tip.
	if _, err := run(dir, "checkout", defaultBranch); err != nil {
		// try master
		if defaultBranch == "main" {
			if _, err2 := run(dir, "checkout", "master"); err2 != nil {
				return err
			}
		} else {
			return err
		}
	}
	// Delete local branch if exists from previous failed run? Prefer create new unique.
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
	if baseBranch == "" {
		baseBranch = "main"
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
	if defaultBranch == "" {
		defaultBranch = "main"
	}
	if _, err := run(dir, "checkout", defaultBranch); err != nil {
		return err
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

package gitx

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestResolveBaseBranchEmptyRepo(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	_, err := ResolveBaseBranch(dir, "main")
	if err == nil {
		t.Fatal("expected error for repo with no commits")
	}
}

func TestEnsureInitialCommitEmptyRepo(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")

	base, inited, err := EnsureInitialCommit(dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	if !inited {
		t.Fatal("expected initialized=true")
	}
	if base != "main" {
		t.Fatalf("base=%q", base)
	}
	if !HasCommits(dir) {
		t.Fatal("expected commits after init")
	}

	base2, inited2, err := EnsureInitialCommit(dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	if inited2 {
		t.Fatal("second call should not re-init")
	}
	if base2 != "main" {
		t.Fatalf("base2=%q", base2)
	}
}

func TestCheckoutBranchEmptyRepoAutoInit(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init") // unborn HEAD, often master/main depending on git version

	if err := CheckoutBranch(dir, "main", "ai-dev/issue-empty"); err != nil {
		t.Fatal(err)
	}
	cur, err := CurrentBranch(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cur != "ai-dev/issue-empty" {
		t.Fatalf("current=%q", cur)
	}
	if !HasCommits(dir) {
		t.Fatal("expected initial commit")
	}
}

func TestResolveBaseBranchUsesConfiguredAndFallback(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "develop")
	writeFile(t, filepath.Join(dir, "README.md"), "hi\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "init")

	base, err := ResolveBaseBranch(dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	if base != "develop" {
		t.Fatalf("want develop fallback, got %q", base)
	}

	base, err = ResolveBaseBranch(dir, "develop")
	if err != nil {
		t.Fatal(err)
	}
	if base != "develop" {
		t.Fatalf("want configured develop, got %q", base)
	}
}

func TestCheckoutBranchFromMaster(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "master")
	writeFile(t, filepath.Join(dir, "a.txt"), "a\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "init")

	if err := CheckoutBranch(dir, "main", "ai-dev/issue-test"); err != nil {
		t.Fatal(err)
	}
	cur, err := CurrentBranch(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cur != "ai-dev/issue-test" {
		t.Fatalf("current=%q", cur)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

package gitx

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func initRepo(t *testing.T, branch string) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", branch)
	runGit(t, dir, "config", "user.email", "t@t")
	runGit(t, dir, "config", "user.name", "t")
	writeFile(t, filepath.Join(dir, "README.md"), "hi\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "init")
	return dir
}

func TestAddWorktreeLeavesMainDirtyAndHEAD(t *testing.T) {
	dir := initRepo(t, "main")
	writeFile(t, filepath.Join(dir, "dirty.txt"), "dirty\n")
	headBefore, err := HeadSHA(dir)
	if err != nil {
		t.Fatal(err)
	}
	branchBefore, err := CurrentBranch(dir)
	if err != nil {
		t.Fatal(err)
	}
	wt := filepath.Join(t.TempDir(), "wt1")
	if err := AddWorktree(dir, wt, "main", "ai-dev/issue-1"); err != nil {
		t.Fatal(err)
	}
	headAfter, _ := HeadSHA(dir)
	branchAfter, _ := CurrentBranch(dir)
	if headAfter != headBefore || branchAfter != branchBefore {
		t.Fatalf("main tree changed: head %s→%s branch %s→%s", headBefore, headAfter, branchBefore, branchAfter)
	}
	dirty, _ := IsDirty(dir)
	if !dirty {
		t.Fatal("expected main tree to stay dirty")
	}
	cur, err := CurrentBranch(wt)
	if err != nil || cur != "ai-dev/issue-1" {
		t.Fatalf("worktree branch=%q err=%v", cur, err)
	}
}

func TestAddWorktreeParallelBranches(t *testing.T) {
	dir := initRepo(t, "main")
	wt1 := filepath.Join(t.TempDir(), "a")
	wt2 := filepath.Join(t.TempDir(), "b")
	if err := AddWorktree(dir, wt1, "main", "ai-dev/one"); err != nil {
		t.Fatal(err)
	}
	if err := AddWorktree(dir, wt2, "main", "ai-dev/two"); err != nil {
		t.Fatal(err)
	}
	b1, _ := CurrentBranch(wt1)
	b2, _ := CurrentBranch(wt2)
	if b1 != "ai-dev/one" || b2 != "ai-dev/two" {
		t.Fatalf("got %q and %q", b1, b2)
	}
}

func TestAddWorktreeReuseSamePath(t *testing.T) {
	dir := initRepo(t, "main")
	wt := filepath.Join(t.TempDir(), "reuse")
	if err := AddWorktree(dir, wt, "main", "ai-dev/rework"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(wt, "f.txt"), "x\n")
	runGit(t, wt, "add", ".")
	runGit(t, wt, "commit", "-m", "wip")
	if err := AddWorktree(dir, wt, "main", "ai-dev/rework"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(wt, "f.txt")); err != nil {
		t.Fatal("expected reused worktree to keep files")
	}
}

func TestRemoveWorktree(t *testing.T) {
	dir := initRepo(t, "main")
	wt := filepath.Join(t.TempDir(), "gone")
	if err := AddWorktree(dir, wt, "main", "ai-dev/gone"); err != nil {
		t.Fatal(err)
	}
	if err := RemoveWorktree(dir, wt); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Fatalf("worktree path still exists: %v", err)
	}
	out, err := run(dir, "worktree", "list", "--porcelain")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, wt) {
		t.Fatalf("worktree list still mentions path:\n%s", out)
	}
	// Missing path should be fine.
	if err := RemoveWorktree(dir, wt); err != nil {
		t.Fatal(err)
	}
}

func TestMergeBranchAtCleanOnBase(t *testing.T) {
	dir := initRepo(t, "main")
	wt := filepath.Join(t.TempDir(), "feat")
	if err := AddWorktree(dir, wt, "main", "ai-dev/feat"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(wt, "feat.txt"), "feat\n")
	runGit(t, wt, "add", ".")
	runGit(t, wt, "commit", "-m", "feat")
	// Main is on main and clean.
	if err := MergeBranchAt(dir, "main", "ai-dev/feat"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "feat.txt")); err != nil {
		t.Fatal("expected merge result on main")
	}
}

func TestMergeBranchAtDirtyOnBaseRefuses(t *testing.T) {
	dir := initRepo(t, "main")
	wt := filepath.Join(t.TempDir(), "feat")
	if err := AddWorktree(dir, wt, "main", "ai-dev/feat2"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(wt, "feat.txt"), "feat\n")
	runGit(t, wt, "add", ".")
	runGit(t, wt, "commit", "-m", "feat")
	writeFile(t, filepath.Join(dir, "dirty.txt"), "x\n")
	err := MergeBranchAt(dir, "main", "ai-dev/feat2")
	if err == nil {
		t.Fatal("expected refuse dirty base")
	}
}

func TestMergeBranchAtFromOtherBranch(t *testing.T) {
	dir := initRepo(t, "main")
	runGit(t, dir, "checkout", "-b", "topic")
	writeFile(t, filepath.Join(dir, "topic.txt"), "t\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "topic")
	wt := filepath.Join(t.TempDir(), "feat")
	if err := AddWorktree(dir, wt, "main", "ai-dev/feat3"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(wt, "feat.txt"), "feat\n")
	runGit(t, wt, "add", ".")
	runGit(t, wt, "commit", "-m", "feat")
	curBefore, _ := CurrentBranch(dir)
	if curBefore != "topic" {
		t.Fatalf("want topic, got %s", curBefore)
	}
	if err := MergeBranchAt(dir, "main", "ai-dev/feat3"); err != nil {
		t.Fatal(err)
	}
	curAfter, _ := CurrentBranch(dir)
	if curAfter != "topic" {
		t.Fatalf("main tree branch changed to %s", curAfter)
	}
	// Verify main branch tip has the file via show.
	out, err := run(dir, "show", "main:feat.txt")
	if err != nil || strings.TrimSpace(out) != "feat" {
		t.Fatalf("main branch missing merge: %v %q", err, out)
	}
}

func TestDiffBetween(t *testing.T) {
	dir := initRepo(t, "main")
	wt := filepath.Join(t.TempDir(), "feat")
	if err := AddWorktree(dir, wt, "main", "ai-dev/diff"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(wt, "new.go"), "package n\n")
	runGit(t, wt, "add", ".")
	runGit(t, wt, "commit", "-m", "add new.go")
	stats, files, commits, ahead, behind, err := DiffBetween(dir, "main", "ai-dev/diff")
	if err != nil {
		t.Fatal(err)
	}
	if ahead < 1 || behind != 0 {
		t.Fatalf("ahead=%d behind=%d", ahead, behind)
	}
	if stats.FilesChanged < 1 || len(files) < 1 || len(commits) < 1 {
		t.Fatalf("stats=%+v files=%d commits=%d", stats, len(files), len(commits))
	}
	if !strings.Contains(files[0].Patch, "new.go") && files[0].Path != "new.go" {
		t.Fatalf("unexpected file %+v", files[0])
	}
}

func TestRebaseOntoFastForwardBase(t *testing.T) {
	dir := initRepo(t, "main")
	wt := filepath.Join(t.TempDir(), "feat")
	if err := AddWorktree(dir, wt, "main", "ai-dev/rebase"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(wt, "feat.txt"), "feat\n")
	runGit(t, wt, "add", ".")
	runGit(t, wt, "commit", "-m", "feat")

	writeFile(t, filepath.Join(dir, "base.txt"), "from main\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "main ahead")

	_, _, _, ahead, behind, err := DiffBetween(dir, "main", "ai-dev/rebase")
	if err != nil {
		t.Fatal(err)
	}
	if behind < 1 || ahead < 1 {
		t.Fatalf("before rebase ahead=%d behind=%d", ahead, behind)
	}
	if err := RebaseOnto(wt, "main"); err != nil {
		t.Fatal(err)
	}
	_, _, _, ahead2, behind2, err := DiffBetween(dir, "main", "ai-dev/rebase")
	if err != nil {
		t.Fatal(err)
	}
	if behind2 != 0 {
		t.Fatalf("after rebase behind=%d ahead=%d", behind2, ahead2)
	}
	if _, err := os.Stat(filepath.Join(wt, "base.txt")); err != nil {
		t.Fatalf("expected base.txt in worktree: %v", err)
	}
}

func TestRebaseOntoConflictLeavesWorktree(t *testing.T) {
	dir := initRepo(t, "main")
	wt := filepath.Join(t.TempDir(), "feat")
	if err := AddWorktree(dir, wt, "main", "ai-dev/conflict"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(wt, "README.md"), "feature line\n")
	runGit(t, wt, "add", ".")
	runGit(t, wt, "commit", "-m", "feat edit")

	writeFile(t, filepath.Join(dir, "README.md"), "main line\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "main edit")

	err := RebaseOnto(wt, "main")
	if err == nil {
		t.Fatal("expected conflict")
	}
	var conflict *RebaseConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("got %T %v", err, err)
	}
	if len(conflict.Files) == 0 {
		t.Fatalf("expected conflict files, got %+v", conflict)
	}
	data, _ := os.ReadFile(filepath.Join(wt, "README.md"))
	if !strings.Contains(string(data), "<<<<<<") && !strings.Contains(string(data), ">>>>>>") && !strings.Contains(string(data), "======") {
		t.Fatalf("expected conflict markers in worktree file: %q", data)
	}
}

func TestPushOriginToBareRemote(t *testing.T) {
	dir := initRepo(t, "main")
	bare := t.TempDir()
	runGit(t, bare, "init", "--bare")
	if HasOrigin(dir) {
		t.Fatal("did not expect origin yet")
	}
	runGit(t, dir, "remote", "add", "origin", bare)
	if !HasOrigin(dir) {
		t.Fatal("expected origin")
	}
	runGit(t, dir, "checkout", "-b", "ai-dev/pub")
	writeFile(t, filepath.Join(dir, "pub.txt"), "ok\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "pub")
	if err := PushOrigin(dir, "ai-dev/pub"); err != nil {
		t.Fatal(err)
	}
	if err := PushOrigin(dir, ""); err == nil {
		t.Fatal("empty branch should fail")
	}
}

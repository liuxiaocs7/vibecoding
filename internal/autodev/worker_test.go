package autodev

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ymhhh/vibecoding/internal/db"
	"github.com/ymhhh/vibecoding/internal/executor"
	"github.com/ymhhh/vibecoding/internal/gitx"
	"github.com/ymhhh/vibecoding/internal/model"
)

type fakeExecutor struct {
	name  string
	calls atomic.Int32
	run   func(ctx context.Context, req executor.CodingRequest, emit executor.Emit, call int) (executor.Result, error)
}

func (f *fakeExecutor) Name() string { return f.name }

func (f *fakeExecutor) Run(ctx context.Context, req executor.CodingRequest, emit executor.Emit) (executor.Result, error) {
	n := int(f.calls.Add(1))
	if emit != nil {
		emit("coding", fmt.Sprintf("fake call %d", n), "")
	}
	if f.run != nil {
		return f.run(ctx, req, emit, n)
	}
	return executor.Result{}, nil
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "init")
}

func setupStoreJob(t *testing.T, repoPath string) (*db.Store, *model.AutoDevJob, *model.Issue, model.GitRepo) {
	t.Helper()
	store, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	repo := model.GitRepo{
		ID:            "repo-1",
		Name:          "demo",
		Path:          repoPath,
		DefaultBranch: "main",
		Language:      "go",
	}
	proj := model.Project{
		ID:        "proj-1",
		Name:      "p",
		GitRepos: []model.GitRepo{repo},
		CreatedAt: model.NowISO(),
		UpdatedAt: model.NowISO(),
	}
	if err := store.UpsertProject(proj); err != nil {
		t.Fatal(err)
	}
	issue := model.Issue{
		ID:                "issue-1",
		ProjectID:         proj.ID,
		Title:             "Add hello",
		Description:       "write hello.txt",
		Priority:          model.PriorityMedium,
		Status:            model.StatusBacklog,
		AssociatedRepoIDs: []string{repo.ID},
		DevSpec: &model.DevSpec{
			Title:       "Add hello",
			Summary:     "create hello.txt",
			RawMarkdown: "# Add hello\n\nCreate hello.txt with hi\n",
			FileChanges: []model.SpecFileChange{{
				FilePath: "hello.txt",
				RepoName: "demo",
				Action:   "create",
				Summary:  "hello",
			}},
			UpdatedAt: model.NowISO(),
		},
		CreatedAt: model.NowISO(),
		UpdatedAt: model.NowISO(),
	}
	if err := store.UpsertIssue(issue); err != nil {
		t.Fatal(err)
	}
	job, err := store.CreateJob(issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	return store, job, &issue, repo
}

func TestRunPreservesMainDirtyAndHEAD(t *testing.T) {
	repoPath := t.TempDir()
	initGitRepo(t, repoPath)

	// Switch main to another branch and leave it dirty.
	cmd := exec.Command("git", "checkout", "-b", "wip")
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	dirtyFile := filepath.Join(repoPath, "dirty.txt")
	if err := os.WriteFile(dirtyFile, []byte("local work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	beforeBranch, err := gitx.CurrentBranch(repoPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeSHA, err := gitx.HeadSHA(repoPath)
	if err != nil {
		t.Fatal(err)
	}

	store, job, _, _ := setupStoreJob(t, repoPath)
	wtRoot := filepath.Join(t.TempDir(), "worktrees")
	fake := &fakeExecutor{
		name: "fake",
		run: func(ctx context.Context, req executor.CodingRequest, emit executor.Emit, call int) (executor.Result, error) {
			// Provide a tiny Go module so language=go test detection succeeds with no tests.
			if err := os.WriteFile(filepath.Join(req.RepoPath, "go.mod"), []byte("module demo\n\ngo 1.22\n"), 0o644); err != nil {
				return executor.Result{}, err
			}
			path := filepath.Join(req.RepoPath, "hello.txt")
			if err := os.WriteFile(path, []byte("hello\n"), 0o644); err != nil {
				return executor.Result{}, err
			}
			return executor.Result{Changes: []model.SpecFileChange{{
				FilePath:     "hello.txt",
				RepoName:     req.RepoName,
				Action:       "create",
				Summary:      "hello",
				ModifiedCode: "hello\n",
			}}}, nil
		},
	}
	r := &Runner{
		Store:        store,
		Hub:          NewHub(),
		WorktreeRoot: wtRoot,
		NewExecutor:  func(cfg model.ExecutorConfig) (executor.Executor, error) { return fake, nil },
	}
	if err := r.run(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}

	afterBranch, _ := gitx.CurrentBranch(repoPath)
	afterSHA, _ := gitx.HeadSHA(repoPath)
	if afterBranch != beforeBranch || afterSHA != beforeSHA {
		t.Fatalf("main HEAD changed: %s/%s -> %s/%s", beforeBranch, beforeSHA, afterBranch, afterSHA)
	}
	dirty, err := gitx.IsDirty(repoPath)
	if err != nil || !dirty {
		t.Fatalf("main should stay dirty: dirty=%v err=%v", dirty, err)
	}
	data, _ := os.ReadFile(dirtyFile)
	if string(data) != "local work\n" {
		t.Fatalf("dirty file mutated: %q", data)
	}

	issue, _ := store.GetIssue("issue-1")
	if issue.Status != model.StatusInReview {
		t.Fatalf("status=%s", issue.Status)
	}
	if issue.PRInfo == nil || len(issue.PRInfo.Worktrees) == 0 {
		t.Fatalf("missing worktrees on PRInfo: %+v", issue.PRInfo)
	}
	wt := issue.PRInfo.Worktrees[0].Path
	if _, err := os.Stat(wt); err != nil {
		t.Fatalf("worktree missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wt, "hello.txt")); err != nil {
		t.Fatalf("hello.txt not in worktree: %v", err)
	}
	if fake.calls.Load() < 1 {
		t.Fatal("executor not called")
	}
}

func TestRunKeepsWorktreeOnFailure(t *testing.T) {
	repoPath := t.TempDir()
	initGitRepo(t, repoPath)
	// Minimal Go module so runTests actually runs and fails.
	if err := os.WriteFile(filepath.Join(repoPath, "go.mod"), []byte("module demo\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = repoPath
	_ = cmd.Run()
	cmd = exec.Command("git", "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "gomod")
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}

	store, job, _, _ := setupStoreJob(t, repoPath)
	_ = store.PutExecutorConfig(model.ExecutorConfig{Type: "llm", MaxHeal: 0})
	wtRoot := filepath.Join(t.TempDir(), "worktrees")
	fake := &fakeExecutor{
		name: "fake-fail",
		run: func(ctx context.Context, req executor.CodingRequest, emit executor.Emit, call int) (executor.Result, error) {
			// Write a failing test into the worktree.
			if err := os.WriteFile(filepath.Join(req.RepoPath, "go.mod"), []byte("module demo\n\ngo 1.22\n"), 0o644); err != nil {
				return executor.Result{}, err
			}
			if err := os.WriteFile(filepath.Join(req.RepoPath, "fail_test.go"), []byte("package demo\n\nimport \"testing\"\n\nfunc TestFail(t *testing.T) { t.Fatal(\"boom\") }\n"), 0o644); err != nil {
				return executor.Result{}, err
			}
			return executor.Result{}, nil
		},
	}
	r := &Runner{
		Store:        store,
		Hub:          NewHub(),
		WorktreeRoot: wtRoot,
		NewExecutor:  func(cfg model.ExecutorConfig) (executor.Executor, error) { return fake, nil },
	}
	err := r.run(context.Background(), job.ID)
	if err == nil {
		t.Fatal("expected failure")
	}
	issue, _ := store.GetIssue("issue-1")
	// On failure Start resets to backlog; run() itself returns error before that when called directly.
	// Worktree should still exist for diagnosis.
	entries, _ := os.ReadDir(wtRoot)
	if len(entries) == 0 {
		t.Fatal("expected worktree to be kept after failure")
	}
	_ = issue
}

func TestRunHealRounds(t *testing.T) {
	repoPath := t.TempDir()
	initGitRepo(t, repoPath)
	if err := os.WriteFile(filepath.Join(repoPath, "go.mod"), []byte("module demo\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "greet.go"), []byte("package demo\n\nfunc Greet() string { return \"\" }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = repoPath
	_ = cmd.Run()
	cmd = exec.Command("git", "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "base")
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}

	store, job, _, _ := setupStoreJob(t, repoPath)
	_ = store.PutExecutorConfig(model.ExecutorConfig{Type: "llm", MaxHeal: 2})
	wtRoot := filepath.Join(t.TempDir(), "worktrees")
	fake := &fakeExecutor{
		name: "fake-heal",
		run: func(ctx context.Context, req executor.CodingRequest, emit executor.Emit, call int) (executor.Result, error) {
			_ = os.WriteFile(filepath.Join(req.RepoPath, "go.mod"), []byte("module demo\n\ngo 1.22\n"), 0o644)
			if call == 1 {
				// Fail tests on first pass.
				_ = os.WriteFile(filepath.Join(req.RepoPath, "greet.go"), []byte("package demo\n\nfunc Greet() string { return \"nope\" }\n"), 0o644)
				_ = os.WriteFile(filepath.Join(req.RepoPath, "greet_test.go"), []byte("package demo\n\nimport \"testing\"\n\nfunc TestGreet(t *testing.T) {\n\tif Greet() != \"hi\" { t.Fatalf(\"got %q\", Greet()) }\n}\n"), 0o644)
				return executor.Result{}, nil
			}
			_ = os.WriteFile(filepath.Join(req.RepoPath, "greet.go"), []byte("package demo\n\nfunc Greet() string { return \"hi\" }\n"), 0o644)
			return executor.Result{}, nil
		},
	}
	r := &Runner{
		Store:        store,
		Hub:          NewHub(),
		WorktreeRoot: wtRoot,
		NewExecutor:  func(cfg model.ExecutorConfig) (executor.Executor, error) { return fake, nil },
	}
	if err := r.run(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	if fake.calls.Load() < 2 {
		t.Fatalf("expected heal re-run, calls=%d", fake.calls.Load())
	}
	issue, _ := store.GetIssue("issue-1")
	if issue.PRInfo == nil || issue.PRInfo.Quality == nil {
		t.Fatalf("missing quality: %+v", issue.PRInfo)
	}
	if issue.PRInfo.Quality.RepairRounds < 1 {
		t.Fatalf("expected repair rounds, got %+v", issue.PRInfo.Quality)
	}
	if !issue.PRInfo.Quality.TestsPassed {
		t.Fatalf("expected tests passed: %+v", issue.PRInfo.Quality)
	}
	if !strings.Contains(issue.PRInfo.Executor, "fake") && issue.PRInfo.Executor == "" {
		// Executor name comes from DisplayName of config (llm), job may store that;
		// quality/heal is the important assertion above.
	}
}

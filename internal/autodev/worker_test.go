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
		GitRepos:  []model.GitRepo{repo},
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
	wantWT := filepath.Join(wtRoot, "issue-1", "repo-1")
	if wt != wantWT {
		t.Fatalf("worktree path=%s want %s", wt, wantWT)
	}
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

func TestRunSecondPassReusesWorktree(t *testing.T) {
	repoPath := t.TempDir()
	initGitRepo(t, repoPath)
	store, job, _, _ := setupStoreJob(t, repoPath)
	wtRoot := filepath.Join(t.TempDir(), "worktrees")
	writeHello := func(ctx context.Context, req executor.CodingRequest, emit executor.Emit, call int) (executor.Result, error) {
		if err := os.WriteFile(filepath.Join(req.RepoPath, "hello.txt"), []byte("hello\n"), 0o644); err != nil {
			return executor.Result{}, err
		}
		return executor.Result{Changes: []model.SpecFileChange{{
			FilePath: "hello.txt", RepoName: req.RepoName, Action: "create", Summary: "hello", ModifiedCode: "hello\n",
		}}}, nil
	}
	r := &Runner{
		Store:        store,
		Hub:          NewHub(),
		WorktreeRoot: wtRoot,
		NewExecutor:  func(cfg model.ExecutorConfig) (executor.Executor, error) { return &fakeExecutor{name: "fake", run: writeHello}, nil },
	}
	if err := r.run(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	job2, err := store.CreateJob("issue-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.run(context.Background(), job2.ID); err != nil {
		t.Fatal(err)
	}
	wt := filepath.Join(wtRoot, "issue-1", "repo-1")
	if _, err := os.Stat(filepath.Join(wt, "hello.txt")); err != nil {
		t.Fatalf("expected second pass to keep worktree: %v", err)
	}
}

func TestRunRelocatesCommitsOffBase(t *testing.T) {
	repoPath := t.TempDir()
	initGitRepo(t, repoPath)
	cmd := exec.Command("git", "checkout", "-b", "wip")
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout wip: %v\n%s", err, out)
	}
	store, job, _, _ := setupStoreJob(t, repoPath)
	baseBefore, err := gitx.RefSHA(repoPath, "refs/heads/main")
	if err != nil {
		t.Fatal(err)
	}
	wtRoot := filepath.Join(t.TempDir(), "worktrees")
	fake := &fakeExecutor{
		name: "fake-on-base",
		run: func(ctx context.Context, req executor.CodingRequest, emit executor.Emit, call int) (executor.Result, error) {
			git := func(args ...string) error {
				c := exec.Command("git", args...)
				c.Dir = req.RepoPath
				c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
				out, err := c.CombinedOutput()
				if err != nil {
					return fmt.Errorf("git %v: %v\n%s", args, err, out)
				}
				return nil
			}
			if err := git("checkout", req.BaseBranch); err != nil {
				return executor.Result{}, err
			}
			if err := os.WriteFile(filepath.Join(req.RepoPath, "hello.txt"), []byte("hello\n"), 0o644); err != nil {
				return executor.Result{}, err
			}
			if err := git("add", "-A"); err != nil {
				return executor.Result{}, err
			}
			if err := git("commit", "-m", "feat: on base"); err != nil {
				return executor.Result{}, err
			}
			if err := git("checkout", "-B", req.Branch); err != nil {
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
	if err := r.run(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	baseAfter, err := gitx.RefSHA(repoPath, "refs/heads/main")
	if err != nil {
		t.Fatal(err)
	}
	if baseAfter != baseBefore {
		t.Fatalf("base branch moved: %s -> %s", baseBefore, baseAfter)
	}
	issue, _ := store.GetIssue("issue-1")
	if issue == nil || issue.PRInfo == nil || issue.PRInfo.BranchName == "" {
		t.Fatalf("missing PR branch: %+v", issue)
	}
	featSHA, err := gitx.RefSHA(repoPath, "refs/heads/"+issue.PRInfo.BranchName)
	if err != nil {
		t.Fatal(err)
	}
	if featSHA == baseBefore {
		t.Fatal("feature branch should keep the relocated commit")
	}
	if _, err := os.Stat(filepath.Join(wtRoot, "issue-1", "repo-1", "hello.txt")); err != nil {
		t.Fatalf("expected file on feature worktree: %v", err)
	}
	cur, _ := gitx.CurrentBranch(repoPath)
	if cur != "wip" {
		t.Fatalf("main working tree branch=%s", cur)
	}
}

func TestRunReworkReplacesStaleWorktree(t *testing.T) {
	repoPath := t.TempDir()
	initGitRepo(t, repoPath)
	store, job, _, _ := setupStoreJob(t, repoPath)
	wtRoot := filepath.Join(t.TempDir(), "worktrees")
	stale := filepath.Join(wtRoot, "issue-1", "repo-1")
	if err := os.MkdirAll(stale, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stale, "junk.txt"), []byte("leftover\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := &fakeExecutor{
		name: "fake-rework",
		run: func(ctx context.Context, req executor.CodingRequest, emit executor.Emit, call int) (executor.Result, error) {
			if err := os.WriteFile(filepath.Join(req.RepoPath, "hello.txt"), []byte("hello\n"), 0o644); err != nil {
				return executor.Result{}, err
			}
			return executor.Result{Changes: []model.SpecFileChange{{
				FilePath: "hello.txt", RepoName: req.RepoName, Action: "create", Summary: "hello", ModifiedCode: "hello\n",
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
	if _, err := os.Stat(filepath.Join(stale, "hello.txt")); err != nil {
		t.Fatalf("expected rework to recreate worktree: %v", err)
	}
	if _, err := os.Stat(filepath.Join(stale, "junk.txt")); !os.IsNotExist(err) {
		t.Fatal("expected leftover junk to be replaced")
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
	logs, err := store.ListJobLogs(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	var sawRepair bool
	for _, l := range logs {
		if strings.Contains(l.Message, "repair round") {
			sawRepair = true
			break
		}
	}
	if !sawRepair {
		t.Fatalf("expected repair round log, got %#v", logs)
	}
}

func TestRunPersistsAndResumesSession(t *testing.T) {
	repoPath := t.TempDir()
	initGitRepo(t, repoPath)
	store, job, _, _ := setupStoreJob(t, repoPath)
	issue, _ := store.GetIssue("issue-1")
	issue.AgentSessionID = "sess-prior"
	if err := store.UpsertIssue(*issue); err != nil {
		t.Fatal(err)
	}
	wtRoot := filepath.Join(t.TempDir(), "worktrees")
	var seenResume string
	fake := &fakeExecutor{
		name: "fake-resume",
		run: func(ctx context.Context, req executor.CodingRequest, emit executor.Emit, call int) (executor.Result, error) {
			seenResume = req.Resume
			if err := os.WriteFile(filepath.Join(req.RepoPath, "go.mod"), []byte("module demo\n\ngo 1.22\n"), 0o644); err != nil {
				return executor.Result{}, err
			}
			if err := os.WriteFile(filepath.Join(req.RepoPath, "hello.txt"), []byte("hello\n"), 0o644); err != nil {
				return executor.Result{}, err
			}
			return executor.Result{
				SessionID: "sess-next",
				Changes: []model.SpecFileChange{{
					FilePath: "hello.txt", RepoName: req.RepoName, Action: "create",
					Summary: "hello", ModifiedCode: "hello\n",
				}},
			}, nil
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
	if seenResume != "sess-prior" {
		t.Fatalf("resume=%q want sess-prior", seenResume)
	}
	saved, _ := store.GetIssue("issue-1")
	if saved.AgentSessionID != "sess-next" {
		t.Fatalf("agentSessionId=%q", saved.AgentSessionID)
	}
	logs, _ := store.ListJobLogs(job.ID)
	var sawResumeLog bool
	for _, l := range logs {
		if strings.Contains(l.Message, "sess-prior") {
			sawResumeLog = true
			break
		}
	}
	if !sawResumeLog {
		t.Fatalf("expected resume log, got %#v", logs)
	}
}

func TestRunUsesJobExecutorSnapshot(t *testing.T) {
	repoPath := t.TempDir()
	initGitRepo(t, repoPath)
	store, job, _, _ := setupStoreJob(t, repoPath)
	_ = store.PutExecutorConfig(model.ExecutorConfig{Type: "llm", MaxHeal: 0})
	snap := model.ExecutorConfig{Type: "agent", Preset: "custom", Command: "echo", Args: []string{"{prompt}"}, MaxHeal: 0}
	job.ExecutorConfig = &snap
	job.Executor = "agent:custom"
	if err := store.UpdateJob(job); err != nil {
		t.Fatal(err)
	}
	_ = store.PutExecutorConfig(model.ExecutorConfig{Type: "llm", MaxHeal: 2})

	var seenType string
	fake := &fakeExecutor{
		name: "from-snapshot",
		run: func(ctx context.Context, req executor.CodingRequest, emit executor.Emit, call int) (executor.Result, error) {
			if err := os.WriteFile(filepath.Join(req.RepoPath, "go.mod"), []byte("module demo\n\ngo 1.22\n"), 0o644); err != nil {
				return executor.Result{}, err
			}
			if err := os.WriteFile(filepath.Join(req.RepoPath, "hello.txt"), []byte("ok\n"), 0o644); err != nil {
				return executor.Result{}, err
			}
			return executor.Result{Changes: []model.SpecFileChange{{
				FilePath: "hello.txt", RepoName: req.RepoName, Action: "create", Summary: "ok", ModifiedCode: "ok\n",
			}}}, nil
		},
	}
	r := &Runner{
		Store:        store,
		Hub:          NewHub(),
		WorktreeRoot: filepath.Join(t.TempDir(), "worktrees"),
		NewExecutor: func(cfg model.ExecutorConfig) (executor.Executor, error) {
			seenType = cfg.Type + ":" + cfg.Preset
			return fake, nil
		},
	}
	if err := r.run(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	if seenType != "agent:custom" {
		t.Fatalf("executor cfg=%q want agent:custom (global was changed to llm)", seenType)
	}
}

func TestRunSetupCommandWritesMarker(t *testing.T) {
	repoPath := t.TempDir()
	initGitRepo(t, repoPath)
	store, job, _, _ := setupStoreJob(t, repoPath)
	proj, err := store.GetProject("proj-1")
	if err != nil || proj == nil {
		t.Fatal(err)
	}
	proj.GitRepos[0].SetupCommand = "echo setup-ok > SETUP_OK"
	if err := store.UpsertProject(*proj); err != nil {
		t.Fatal(err)
	}
	fake := &fakeExecutor{
		name: "fake",
		run: func(ctx context.Context, req executor.CodingRequest, emit executor.Emit, call int) (executor.Result, error) {
			if err := os.WriteFile(filepath.Join(req.RepoPath, "go.mod"), []byte("module demo\n\ngo 1.22\n"), 0o644); err != nil {
				return executor.Result{}, err
			}
			if err := os.WriteFile(filepath.Join(req.RepoPath, "hello.txt"), []byte("hi\n"), 0o644); err != nil {
				return executor.Result{}, err
			}
			return executor.Result{Changes: []model.SpecFileChange{{
				FilePath: "hello.txt", RepoName: req.RepoName, Action: "create", Summary: "hi", ModifiedCode: "hi\n",
			}}}, nil
		},
	}
	wtRoot := filepath.Join(t.TempDir(), "worktrees")
	r := &Runner{Store: store, Hub: NewHub(), WorktreeRoot: wtRoot, NewExecutor: func(cfg model.ExecutorConfig) (executor.Executor, error) { return fake, nil }}
	if err := r.run(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(wtRoot, "issue-1", "repo-1", "SETUP_OK")
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("setup marker missing: %v", err)
	}
	logs, _ := store.ListJobLogs(job.ID)
	var sawSetup bool
	for _, l := range logs {
		if l.Phase == "setup" {
			sawSetup = true
			break
		}
	}
	if !sawSetup {
		t.Fatalf("expected setup log, got %#v", logs)
	}
}

func TestRunSetupFailureKeepsWorktree(t *testing.T) {
	repoPath := t.TempDir()
	initGitRepo(t, repoPath)
	store, job, _, _ := setupStoreJob(t, repoPath)
	proj, _ := store.GetProject("proj-1")
	proj.GitRepos[0].SetupCommand = "false"
	_ = store.UpsertProject(*proj)
	fake := &fakeExecutor{name: "unused"}
	wtRoot := filepath.Join(t.TempDir(), "worktrees")
	r := &Runner{
		Store: store, Hub: NewHub(), WorktreeRoot: wtRoot,
		NewExecutor: func(cfg model.ExecutorConfig) (executor.Executor, error) {
			return fake, nil
		},
	}
	err := r.run(context.Background(), job.ID)
	if err == nil {
		t.Fatal("expected setup failure")
	}
	if _, err := os.Stat(filepath.Join(wtRoot, "issue-1", "repo-1")); err != nil {
		t.Fatalf("worktree should remain: %v", err)
	}
	if fake.calls.Load() != 0 {
		t.Fatalf("executor should not run after setup failure, calls=%d", fake.calls.Load())
	}
}

func TestConfiguredTestCommandFailure(t *testing.T) {
	repoPath := t.TempDir()
	initGitRepo(t, repoPath)
	store, job, _, _ := setupStoreJob(t, repoPath)
	proj, _ := store.GetProject("proj-1")
	proj.GitRepos[0].TestCommand = "echo fail-out; exit 1"
	_ = store.UpsertProject(*proj)
	fake := &fakeExecutor{
		name: "fake",
		run: func(ctx context.Context, req executor.CodingRequest, emit executor.Emit, call int) (executor.Result, error) {
			if err := os.WriteFile(filepath.Join(req.RepoPath, "hello.txt"), []byte("hi\n"), 0o644); err != nil {
				return executor.Result{}, err
			}
			return executor.Result{Changes: []model.SpecFileChange{{
				FilePath: "hello.txt", RepoName: req.RepoName, Action: "create", Summary: "hi",
			}}}, nil
		},
	}
	wtRoot := filepath.Join(t.TempDir(), "worktrees")
	r := &Runner{
		Store: store, Hub: NewHub(), WorktreeRoot: wtRoot,
		NewExecutor: func(cfg model.ExecutorConfig) (executor.Executor, error) { return fake, nil },
	}
	err := r.run(context.Background(), job.ID)
	if err == nil {
		t.Fatal("expected test failure")
	}
	logs, _ := store.ListJobLogs(job.ID)
	var sawConfigured bool
	for _, l := range logs {
		if l.Phase == "testing" && strings.Contains(l.Message, "configured:") {
			sawConfigured = true
			break
		}
	}
	if !sawConfigured {
		t.Fatalf("expected configured test log, got %#v", logs)
	}
}

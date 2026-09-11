package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ymhhh/vibecoding/internal/executor"
	"github.com/ymhhh/vibecoding/internal/model"
)

// fakeCodingExecutor mirrors autodev test doubles so API can drive a full Auto-Dev → diff → approve path.
type fakeCodingExecutor struct {
	name  string
	calls atomic.Int32
	run   func(ctx context.Context, req executor.CodingRequest, emit executor.Emit, call int) (executor.Result, error)
}

func (f *fakeCodingExecutor) Name() string { return f.name }

func (f *fakeCodingExecutor) Run(ctx context.Context, req executor.CodingRequest, emit executor.Emit) (executor.Result, error) {
	n := int(f.calls.Add(1))
	if emit != nil {
		emit("coding", fmt.Sprintf("fake call %d", n), "")
	}
	if f.run != nil {
		return f.run(ctx, req, emit, n)
	}
	return executor.Result{}, nil
}

func TestAutoDevPipelineRealDiffAndApprove(t *testing.T) {
	srv, store := testServer(t)
	repoDir := t.TempDir()
	initRepo(t, repoDir, "main")

	// Non-default branch + dirty main working tree (acceptance item 1 style).
	run := func(args ...string) {
		t.Helper()
		cmd := execCommand(t, repoDir, args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("checkout", "-b", "wip")
	if err := os.WriteFile(filepath.Join(repoDir, "dirty.txt"), []byte("local\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	repo := model.GitRepo{ID: "repo-1", Name: "demo", Path: repoDir, DefaultBranch: "main", Language: "go"}
	proj := model.Project{
		ID: "proj-1", Name: "p", GitRepos: []model.GitRepo{repo},
		CreatedAt: model.NowISO(), UpdatedAt: model.NowISO(),
	}
	if err := store.UpsertProject(proj); err != nil {
		t.Fatal(err)
	}
	issue := model.Issue{
		ID: "issue-pipe", ProjectID: proj.ID, Title: "Add hello", Description: "write hello.txt",
		Priority: model.PriorityMedium, Status: model.StatusBacklog,
		AssociatedRepoIDs: []string{repo.ID},
		DevSpec: &model.DevSpec{
			Title: "Add hello", Summary: "create hello.txt",
			RawMarkdown: "# Add hello\n\nCreate hello.txt\n",
			FileChanges: []model.SpecFileChange{{
				FilePath: "hello.txt", RepoName: "demo", Action: "create", Summary: "hello",
			}},
			UpdatedAt: model.NowISO(),
		},
		CreatedAt: model.NowISO(), UpdatedAt: model.NowISO(),
	}
	if err := store.UpsertIssue(issue); err != nil {
		t.Fatal(err)
	}
	job, err := store.CreateJob(issue.ID)
	if err != nil {
		t.Fatal(err)
	}

	wtRoot := filepath.Join(t.TempDir(), "worktrees")
	fake := &fakeCodingExecutor{
		name: "fake-llm",
		run: func(ctx context.Context, req executor.CodingRequest, emit executor.Emit, call int) (executor.Result, error) {
			if err := os.WriteFile(filepath.Join(req.RepoPath, "go.mod"), []byte("module demo\n\ngo 1.22\n"), 0o644); err != nil {
				return executor.Result{}, err
			}
			if err := os.WriteFile(filepath.Join(req.RepoPath, "hello.txt"), []byte("hello from pipeline\n"), 0o644); err != nil {
				return executor.Result{}, err
			}
			return executor.Result{Changes: []model.SpecFileChange{{
				FilePath: "hello.txt", RepoName: req.RepoName, Action: "create",
				Summary: "hello", ModifiedCode: "hello from pipeline\n",
			}}}, nil
		},
	}
	srv.Runner.WorktreeRoot = wtRoot
	srv.Runner.NewExecutor = func(cfg model.ExecutorConfig) (executor.Executor, error) { return fake, nil }

	if err := srv.Runner.RunSync(context.Background(), job.ID); err != nil {
		t.Fatalf("auto-dev run: %v", err)
	}

	logs, err := store.ListJobLogs(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	var hasCoding, hasTesting bool
	for _, l := range logs {
		if l.Phase == "coding" {
			hasCoding = true
		}
		if l.Phase == "testing" {
			hasTesting = true
		}
	}
	if !hasCoding || !hasTesting {
		t.Fatalf("expected coding+testing logs, got %#v", logs)
	}

	updated, err := store.GetIssue(issue.ID)
	if err != nil || updated == nil || updated.PRInfo == nil {
		t.Fatalf("issue after run: %+v err=%v", updated, err)
	}
	wantWT := filepath.Join(wtRoot, issue.ID, repo.ID)
	if len(updated.PRInfo.Worktrees) == 0 || updated.PRInfo.Worktrees[0].Path != wantWT {
		t.Fatalf("worktree path=%v want %s", updated.PRInfo.Worktrees, wantWT)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/issues/"+issue.ID+"/diff", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("diff status=%d %s", rr.Code, rr.Body.String())
	}
	var diff issueDiffResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &diff); err != nil {
		t.Fatal(err)
	}
	if len(diff.Repos) != 1 {
		t.Fatalf("repos=%+v", diff.Repos)
	}
	st := diff.Repos[0].Stats
	// Legacy mock Review used fixed 98/14; real git diff must not match that placeholder.
	if st.Additions == 98 && st.Deletions == 14 {
		t.Fatalf("got placeholder mock stats %+v", st)
	}
	if st.FilesChanged < 1 || st.Additions < 1 {
		t.Fatalf("expected real diff stats, got %+v files=%d", st, len(diff.Repos[0].Files))
	}
	foundHello := false
	for _, f := range diff.Repos[0].Files {
		if f.Path == "hello.txt" {
			foundHello = true
			break
		}
	}
	if !foundHello {
		t.Fatalf("expected hello.txt in diff files: %+v", diff.Repos[0].Files)
	}

	// Clean default branch for in-place merge: main tree is on wip+dirty, MergeBranchAt uses temp worktree.
	req = httptest.NewRequest(http.MethodPost, "/api/issues/"+issue.ID+"/approve-merge", bytes.NewReader(nil))
	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("approve status=%d %s", rr.Code, rr.Body.String())
	}
	if _, err := os.Stat(wantWT); !os.IsNotExist(err) {
		t.Fatalf("worktree should be removed after approve, err=%v", err)
	}
	// Feature content lives on main branch tip (main tree may still be on wip).
	show := execCommand(t, repoDir, "show", "main:hello.txt")
	out, err := show.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "hello from pipeline") {
		t.Fatalf("main:hello.txt after merge: %v %q", err, out)
	}
}

func execCommand(t *testing.T, dir string, args ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	return cmd
}

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ymhhh/vibecoding/internal/model"
)

func seedReadyIssue(t *testing.T, store interface {
	UpsertProject(model.Project) error
	UpsertIssue(model.Issue) error
}, repoPath string) model.Issue {
	t.Helper()
	repo := model.GitRepo{ID: "repo-1", Name: "demo", Path: repoPath, DefaultBranch: "main"}
	proj := model.Project{ID: "proj-start", Name: "p", GitRepos: []model.GitRepo{repo}, CreatedAt: model.NowISO(), UpdatedAt: model.NowISO()}
	if err := store.UpsertProject(proj); err != nil {
		t.Fatal(err)
	}
	issue := model.Issue{
		ID: "issue-start", ProjectID: proj.ID, Title: "t", Description: "d",
		Priority: model.PriorityMedium, Status: model.StatusBacklog,
		AssociatedRepoIDs: []string{repo.ID},
		DevSpec: &model.DevSpec{
			Title: "t", RawMarkdown: "# t\n", UpdatedAt: model.NowISO(),
		},
		CreatedAt: model.NowISO(), UpdatedAt: model.NowISO(),
	}
	if err := store.UpsertIssue(issue); err != nil {
		t.Fatal(err)
	}
	return issue
}

func TestAutoDevStartOmitsExecutorUsesGlobal(t *testing.T) {
	srv, store := testServer(t)
	dir := t.TempDir()
	initRepo(t, dir, "main")
	issue := seedReadyIssue(t, store, dir)

	req := httptest.NewRequest(http.MethodPost, "/api/auto-dev/start", strings.NewReader(`{"issueId":"`+issue.ID+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != 202 {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var job model.AutoDevJob
	if err := json.Unmarshal(rr.Body.Bytes(), &job); err != nil {
		t.Fatal(err)
	}
	if job.Executor != "llm" {
		t.Fatalf("executor=%q", job.Executor)
	}
	if job.ExecutorConfig == nil || job.ExecutorConfig.Type != "llm" {
		t.Fatalf("snapshot=%+v", job.ExecutorConfig)
	}
	// Cancel so the background runner does not leak into other tests.
	creq := httptest.NewRequest(http.MethodPost, "/api/auto-dev/jobs/"+job.ID+"/cancel", strings.NewReader("{}"))
	crr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(crr, creq)
}

func TestAutoDevStartRejectsMissingAgent(t *testing.T) {
	srv, store := testServer(t)
	dir := t.TempDir()
	initRepo(t, dir, "main")
	issue := seedReadyIssue(t, store, dir)

	body := `{"issueId":"` + issue.ID + `","executor":{"type":"agent","preset":"custom","command":"definitely-missing-vibecoding-bin","args":["{prompt}"]}}`
	req := httptest.NewRequest(http.MethodPost, "/api/auto-dev/start", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != 400 {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if active, _ := store.ActiveJobForRepoIssue(issue.ID); active != nil {
		t.Fatalf("job should not be created: %+v", active)
	}
}

func TestAutoDevStartOverrideSnapshotsAndClearsSession(t *testing.T) {
	srv, store := testServer(t)
	dir := t.TempDir()
	initRepo(t, dir, "main")
	issue := seedReadyIssue(t, store, dir)
	issue.AgentSessionID = "sess-old"
	issue.PRInfo = &model.PRInfo{Executor: "agent:claude", BranchName: "ai-dev/issue-start", Status: "open"}
	if err := store.UpsertIssue(issue); err != nil {
		t.Fatal(err)
	}
	_ = store.PutModelConfig(model.ModelConfig{OpenAIAPIKey: "sk-test-override", OpenAIModel: "gpt-test"})

	body := `{"issueId":"` + issue.ID + `","executor":{"type":"llm"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/auto-dev/start", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != 202 {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var job model.AutoDevJob
	if err := json.Unmarshal(rr.Body.Bytes(), &job); err != nil {
		t.Fatal(err)
	}
	if job.Executor != "llm" || job.ExecutorConfig == nil || job.ExecutorConfig.Type != "llm" {
		t.Fatalf("%+v", job)
	}
	saved, _ := store.GetIssue(issue.ID)
	if saved.AgentSessionID != "" {
		t.Fatalf("session should be cleared, got %q", saved.AgentSessionID)
	}
	creq := httptest.NewRequest(http.MethodPost, "/api/auto-dev/jobs/"+job.ID+"/cancel", strings.NewReader("{}"))
	srv.Handler().ServeHTTP(httptest.NewRecorder(), creq)
}

func TestAutoDevStartPersistsSnapshotOnGetJob(t *testing.T) {
	srv, store := testServer(t)
	dir := t.TempDir()
	initRepo(t, dir, "main")
	issue := seedReadyIssue(t, store, dir)

	req := httptest.NewRequest(http.MethodPost, "/api/auto-dev/start", strings.NewReader(`{"issueId":"`+issue.ID+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != 202 {
		t.Fatalf("status=%d %s", rr.Code, rr.Body.String())
	}
	var created model.AutoDevJob
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	got, err := store.GetJob(created.ID)
	if err != nil || got == nil || got.ExecutorConfig == nil {
		t.Fatalf("GetJob snapshot missing: %+v err=%v", got, err)
	}
	if got.ExecutorConfig.Type != "llm" {
		t.Fatalf("%+v", got.ExecutorConfig)
	}
	creq := httptest.NewRequest(http.MethodPost, "/api/auto-dev/jobs/"+created.ID+"/cancel", strings.NewReader("{}"))
	srv.Handler().ServeHTTP(httptest.NewRecorder(), creq)
}

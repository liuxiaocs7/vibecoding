package db

import (
	"path/filepath"
	"testing"

	"github.com/ymhhh/vibecoding/internal/model"
)

func TestFailOrphanJobs(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	proj := model.Project{
		ID:        "proj-1",
		Name:      "p",
		CreatedAt: model.NowISO(),
		UpdatedAt: model.NowISO(),
	}
	if err := store.UpsertProject(proj); err != nil {
		t.Fatal(err)
	}
	iss := model.Issue{
		ID:        "issue-1",
		ProjectID: proj.ID,
		Title:     "stuck",
		Status:    model.StatusInProgress,
		CreatedAt: model.NowISO(),
		UpdatedAt: model.NowISO(),
	}
	if err := store.UpsertIssue(iss); err != nil {
		t.Fatal(err)
	}
	job, err := store.CreateJob(iss.ID)
	if err != nil {
		t.Fatal(err)
	}
	job.Status = model.JobRunning
	job.Phase = "coding"
	if err := store.UpdateJob(job); err != nil {
		t.Fatal(err)
	}

	n, err := store.FailOrphanJobs()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("n=%d want 1", n)
	}

	gotJob, err := store.GetJob(job.ID)
	if err != nil || gotJob == nil {
		t.Fatalf("get job: %v %#v", err, gotJob)
	}
	if gotJob.Status != model.JobFailed {
		t.Fatalf("status=%s", gotJob.Status)
	}
	if gotJob.Error != "interrupted: process restarted" {
		t.Fatalf("error=%q", gotJob.Error)
	}

	gotIss, err := store.GetIssue(iss.ID)
	if err != nil || gotIss == nil {
		t.Fatalf("get issue: %v %#v", err, gotIss)
	}
	if gotIss.Status != model.StatusBacklog {
		t.Fatalf("issue status=%s want backlog", gotIss.Status)
	}

	n2, err := store.FailOrphanJobs()
	if err != nil {
		t.Fatal(err)
	}
	if n2 != 0 {
		t.Fatalf("second call n=%d", n2)
	}
}

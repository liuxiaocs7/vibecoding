package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ymhhh/vibecoding/internal/model"
)

func TestAcceptRequirementAndBacklogGates(t *testing.T) {
	srv, store := testServer(t)
	h := srv.Handler()

	iss := model.Issue{
		ID:                "ISSUE-REQ-1",
		ProjectID:         "proj-1",
		Title:             "demo",
		Status:            model.StatusRequirements,
		AssociatedRepoIDs: []string{"repo-1"},
		ReqDoc: &model.ReqDoc{
			RawMarkdown: "# req",
			UpdatedAt:   "2024-01-01T00:00:00Z",
		},
		DevSpec: &model.DevSpec{
			RawMarkdown: "# design",
			UpdatedAt:   "2024-01-02T00:00:00Z",
			FileChanges: []model.SpecFileChange{{FilePath: "a.go", RepoName: "svc", Action: "modify"}},
		},
		CreatedAt: model.NowISO(),
		UpdatedAt: model.NowISO(),
	}
	if err := store.UpsertIssue(iss); err != nil {
		t.Fatal(err)
	}

	// Move to backlog without accept → 400
	body, _ := json.Marshal(map[string]any{
		"id": iss.ID, "projectId": iss.ProjectID, "title": iss.Title,
		"status": "backlog", "associatedRepoIds": iss.AssociatedRepoIDs,
		"reqDoc": iss.ReqDoc, "devSpec": iss.DevSpec,
		"chatMessages": []any{}, "autoDevLogs": []any{},
		"createdAt": iss.CreatedAt, "updatedAt": model.NowISO(),
	})
	req := httptest.NewRequest(http.MethodPut, "/api/issues/"+iss.ID, bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 400 {
		t.Fatalf("expected 400 before accept, got %d %s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/issues/"+iss.ID+"/accept-requirement", bytes.NewReader([]byte("{}")))
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("accept status=%d body=%s", rr.Code, rr.Body.String())
	}
	var accepted model.Issue
	if err := json.Unmarshal(rr.Body.Bytes(), &accepted); err != nil {
		t.Fatal(err)
	}
	if accepted.ReqDoc == nil || accepted.ReqDoc.AcceptedAt == "" {
		t.Fatalf("acceptedAt missing: %+v", accepted.ReqDoc)
	}

	body, _ = json.Marshal(accepted)
	// force status backlog
	var m map[string]any
	_ = json.Unmarshal(body, &m)
	m["status"] = "backlog"
	body, _ = json.Marshal(m)
	req = httptest.NewRequest(http.MethodPut, "/api/issues/"+iss.ID, bytes.NewReader(body))
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("backlog after accept status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestGenerateSpecRequiresAcceptedReq(t *testing.T) {
	srv, store := testServer(t)
	h := srv.Handler()

	iss := model.Issue{
		ID:                "ISSUE-REQ-2",
		ProjectID:         "proj-1",
		Title:             "demo",
		Status:            model.StatusRequirements,
		AssociatedRepoIDs: []string{},
		ReqDoc: &model.ReqDoc{
			RawMarkdown: "# req",
			UpdatedAt:   model.NowISO(),
		},
		CreatedAt: model.NowISO(),
		UpdatedAt: model.NowISO(),
	}
	if err := store.UpsertIssue(iss); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"prompt": "design please"})
	req := httptest.NewRequest(http.MethodPost, "/api/issues/"+iss.ID+"/spec", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 400 {
		t.Fatalf("expected 400, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestReqDocFromBriefAndExport(t *testing.T) {
	srv, store := testServer(t)
	h := srv.Handler()

	iss := model.Issue{
		ID:          "ISSUE-REQ-MD",
		ProjectID:   "proj-1",
		Title:       "Brief Feature",
		Description: "Ship markdown requirements",
		Status:      model.StatusRequirements,
		CreatedAt:   model.NowISO(),
		UpdatedAt:   model.NowISO(),
	}
	if err := store.UpsertIssue(iss); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/issues/"+iss.ID+"/req-doc/from-brief", bytes.NewReader([]byte("{}")))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("from-brief status=%d body=%s", rr.Code, rr.Body.String())
	}
	var saved model.Issue
	if err := json.Unmarshal(rr.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.ReqDoc == nil || !strings.Contains(saved.ReqDoc.RawMarkdown, "# Brief Feature") {
		t.Fatalf("expected markdown reqDoc, got %+v", saved.ReqDoc)
	}
	if saved.DocPhase != model.DocPhaseRequirement {
		t.Fatalf("docPhase=%q", saved.DocPhase)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/issues/"+iss.ID+"/export-req", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("export-req status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "Ship markdown requirements") {
		t.Fatalf("export body=%s", rr.Body.String())
	}
	cd := rr.Header().Get("Content-Disposition")
	if !strings.Contains(cd, ".md") {
		t.Fatalf("Content-Disposition=%q", cd)
	}
}

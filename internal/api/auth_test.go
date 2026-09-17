package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/ymhhh/vibecoding/internal/db"
)

func TestAuthRequiredOnAPI(t *testing.T) {
	dir := t.TempDir()
	store, err := db.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	srv := &Server{Store: store, Token: "s3cret"}
	h := srv.Handler()

	// Health is public and reports authRequired.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if rr.Code != 200 {
		t.Fatalf("health status=%d", rr.Code)
	}
	if body := rr.Body.String(); !contains(body, `"authRequired":true`) {
		t.Fatalf("health body=%s", body)
	}

	// Projects without token → 401
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/projects", nil))
	if rr.Code != 401 {
		t.Fatalf("projects no auth status=%d body=%s", rr.Code, rr.Body.String())
	}

	// Bearer token → 200
	rr = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	req.Header.Set("Authorization", "Bearer s3cret")
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("bearer status=%d body=%s", rr.Code, rr.Body.String())
	}

	// Header token → 200
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	req.Header.Set("X-Vibecoding-Token", "s3cret")
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("header status=%d", rr.Code)
	}

	// Query token (EventSource) → 200
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/projects?token=s3cret", nil))
	if rr.Code != 200 {
		t.Fatalf("query status=%d", rr.Code)
	}
}

func TestAuthNotRequiredWhenTokenEmpty(t *testing.T) {
	dir := t.TempDir()
	store, err := db.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	srv := &Server{Store: store}
	h := srv.Handler()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/projects", nil))
	if rr.Code != 200 {
		t.Fatalf("status=%d", rr.Code)
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if !contains(rr.Body.String(), `"authRequired":false`) {
		t.Fatalf("body=%s", rr.Body.String())
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

package api

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/ymhhh/vibecoding/internal/model"
)

func (s *Server) handleListIssues(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("projectId")
	list, err := s.Store.ListIssues(projectID)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, list)
}

func (s *Server) handleGetIssue(w http.ResponseWriter, r *http.Request) {
	iss, err := s.Store.GetIssue(r.PathValue("id"))
	if err != nil || iss == nil {
		writeErr(w, 404, "issue not found")
		return
	}
	writeJSON(w, 200, iss)
}

func (s *Server) handleCreateIssue(w http.ResponseWriter, r *http.Request) {
	var iss model.Issue
	if err := decodeJSON(r, &iss); err != nil {
		writeErr(w, 400, "invalid JSON")
		return
	}
	if iss.ID == "" {
		iss.ID = "issue-" + uuid.NewString()[:8]
	}
	if iss.Status == "" {
		iss.Status = model.StatusRequirements
	}
	if err := s.Store.UpsertIssue(iss); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	saved, _ := s.Store.GetIssue(iss.ID)
	writeJSON(w, 201, saved)
}

func (s *Server) handleUpdateIssue(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, err := s.Store.GetIssue(id)
	if err != nil || existing == nil {
		writeErr(w, 404, "issue not found")
		return
	}
	var iss model.Issue
	if err := decodeJSON(r, &iss); err != nil {
		writeErr(w, 400, "invalid JSON")
		return
	}
	iss.ID = id
	iss.CreatedAt = existing.CreatedAt
	if err := validateStatusTransition(existing, &iss); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if err := s.Store.UpsertIssue(iss); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	saved, _ := s.Store.GetIssue(id)
	writeJSON(w, 200, saved)
}

func validateStatusTransition(old, neu *model.Issue) error {
	if old.Status == neu.Status {
		return nil
	}
	switch neu.Status {
	case model.StatusBacklog:
		if !neu.RequirementAccepted() {
			return fmt.Errorf("cannot move to backlog without an accepted requirement document")
		}
		if neu.DesignStale() {
			return fmt.Errorf("cannot move to backlog: design is stale after requirement changes; regenerate Dev Spec")
		}
		if !neu.SpecReadyForDev() {
			return fmt.Errorf("cannot move to backlog without a Markdown Dev Spec with file changes (all sub-requirements need specs when split)")
		}
		if len(neu.AssociatedRepoIDs) == 0 {
			return fmt.Errorf("cannot move to backlog without associated repositories")
		}
	case model.StatusInReview:
		if neu.PRInfo == nil && old.PRInfo == nil {
			return fmt.Errorf("cannot move to in_review without a completed auto-dev job")
		}
	}
	return nil
}

func (s *Server) handleDeleteIssue(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteIssue(r.PathValue("id")); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeErr(w, 404, "issue not found")
			return
		}
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

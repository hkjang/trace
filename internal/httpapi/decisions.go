package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hkjang/trace/internal/domain"
	"github.com/hkjang/trace/internal/store"
)

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	result, err := s.Store.Dashboard(r.Context(), userFrom(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (s *Server) handleAnalytics(w http.ResponseWriter, r *http.Request) {
	result, err := s.Store.Analytics(r.Context(), userFrom(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (s *Server) handleListDecisions(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	result, err := s.Store.ListDecisions(r.Context(), userFrom(r), r.URL.Query().Get("status"), r.URL.Query().Get("q"), limit, offset)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": result})
}
func (s *Server) handleCreateDecision(w http.ResponseWriter, r *http.Request) {
	var input domain.DecisionInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.Store.CreateDecision(r.Context(), userFrom(r), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}
func (s *Server) handleUpdateDecision(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, store.ErrNotFound)
		return
	}
	var input domain.DecisionPatch
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.Store.UpdateDecision(r.Context(), userFrom(r), id, input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (s *Server) handleGetDecision(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, store.ErrNotFound)
		return
	}
	item, err := s.Store.GetDecision(r.Context(), userFrom(r), id, nil)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (s *Server) handleReplayDecision(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, store.ErrNotFound)
		return
	}
	at, err := time.Parse(time.RFC3339, r.URL.Query().Get("at"))
	if err != nil {
		writeError(w, store.ErrValidation)
		return
	}
	item, err := s.Store.GetDecision(r.Context(), userFrom(r), id, &at)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"replayAt": at, "decision": item})
}
func (s *Server) handleAddEvidence(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, store.ErrNotFound)
		return
	}
	var input domain.EvidenceInput
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.Store.AddEvidence(r.Context(), userFrom(r), id, input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}
func (s *Server) handleAddOutcome(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, store.ErrNotFound)
		return
	}
	var input domain.Outcome
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.Store.AddOutcome(r.Context(), userFrom(r), id, input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}
func (s *Server) handleAddReflection(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, store.ErrNotFound)
		return
	}
	var input domain.Reflection
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.Store.AddReflection(r.Context(), userFrom(r), id, input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) handleSubmitApproval(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, store.ErrNotFound)
		return
	}
	var input struct {
		Note string `json:"note"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.Store.SubmitDecisionForApproval(r.Context(), userFrom(r), id, input.Note)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}
func (s *Server) handleListApprovals(w http.ResponseWriter, r *http.Request) {
	items, err := s.Store.ListApprovals(r.Context(), userFrom(r), r.URL.Query().Get("state"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
func (s *Server) handleReviewApproval(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, store.ErrNotFound)
		return
	}
	action := chi.URLParam(r, "action")
	var input struct {
		Note string `json:"note"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.Store.ReviewApproval(r.Context(), userFrom(r), id, action, input.Note)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

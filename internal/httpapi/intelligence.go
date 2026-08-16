package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hkjang/trace/internal/store"
)

func urlUUID(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		return uuid.Nil, store.ErrNotFound
	}
	return id, nil
}

func optionalRFC3339(raw string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, store.ErrValidation
	}
	return &value, nil
}

func (s *Server) handleDecisionVersions(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	items, err := s.Store.ListDecisionVersions(r.Context(), userFrom(r), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleReplayCompare(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	from, fromErr := time.Parse(time.RFC3339, r.URL.Query().Get("from"))
	to, toErr := time.Parse(time.RFC3339, r.URL.Query().Get("to"))
	if fromErr != nil || toErr != nil {
		writeError(w, store.ErrValidation)
		return
	}
	result, err := s.Store.CompareReplay(r.Context(), userFrom(r), id, from, to)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleAddConfidence(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var input struct {
		Confidence int        `json:"confidence"`
		Reason     string     `json:"reason"`
		RecordedAt *time.Time `json:"recordedAt"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	recordedAt := time.Now().UTC()
	if input.RecordedAt != nil {
		recordedAt = *input.RecordedAt
	}
	result, err := s.Store.AddConfidence(r.Context(), userFrom(r), id, input.Confidence, input.Reason, recordedAt)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) handleConfidenceHistory(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	decision, err := s.Store.GetDecision(r.Context(), userFrom(r), id, nil)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": decision.ConfidenceHistory})
}

func (s *Server) handleAddAssumption(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var input struct {
		Assumption string     `json:"assumption"`
		Status     string     `json:"status"`
		KnownAt    *time.Time `json:"knownAt"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	knownAt := time.Now().UTC()
	if input.KnownAt != nil {
		knownAt = *input.KnownAt
	}
	result, err := s.Store.AddAssumption(r.Context(), userFrom(r), id, input.Assumption, input.Status, knownAt)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) handleUpdateAssumption(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var input struct {
		Status     string     `json:"status"`
		Reason     string     `json:"reason"`
		EvidenceID *uuid.UUID `json:"evidenceId"`
		KnownAt    *time.Time `json:"knownAt"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	knownAt := time.Now().UTC()
	if input.KnownAt != nil {
		knownAt = *input.KnownAt
	}
	result, err := s.Store.UpdateAssumption(r.Context(), userFrom(r), id, input.Status, input.Reason, input.EvidenceID, knownAt)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleAddInvalidationCondition(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var input struct {
		Condition string     `json:"condition"`
		KnownAt   *time.Time `json:"knownAt"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	knownAt := time.Now().UTC()
	if input.KnownAt != nil {
		knownAt = *input.KnownAt
	}
	result, err := s.Store.AddInvalidationCondition(r.Context(), userFrom(r), id, input.Condition, knownAt)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) handleUpdateInvalidationCondition(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var input struct {
		Status     string     `json:"status"`
		Note       string     `json:"note"`
		EvidenceID *uuid.UUID `json:"evidenceId"`
		KnownAt    *time.Time `json:"knownAt"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	knownAt := time.Now().UTC()
	if input.KnownAt != nil {
		knownAt = *input.KnownAt
	}
	result, err := s.Store.UpdateInvalidationCondition(r.Context(), userFrom(r), id, input.Status, input.Note, input.EvidenceID, knownAt)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleCreateDecisionLink(w http.ResponseWriter, r *http.Request) {
	sourceID, err := urlUUID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var input struct {
		TargetDecisionID uuid.UUID  `json:"targetDecisionId"`
		RelationType     string     `json:"relationType"`
		Description      string     `json:"description"`
		EffectiveAt      *time.Time `json:"effectiveAt"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	effectiveAt := time.Now().UTC()
	if input.EffectiveAt != nil {
		effectiveAt = *input.EffectiveAt
	}
	result, err := s.Store.AddDecisionLink(r.Context(), userFrom(r), sourceID, input.TargetDecisionID, input.RelationType, input.Description, effectiveAt)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) handleDeleteDecisionLink(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.Store.DeleteDecisionLink(r.Context(), userFrom(r), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDecisionGraph(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	depth, _ := strconv.Atoi(r.URL.Query().Get("depth"))
	at, err := optionalRFC3339(r.URL.Query().Get("at"))
	if err != nil {
		writeError(w, err)
		return
	}
	result, err := s.Store.DecisionGraph(r.Context(), userFrom(r), &id, depth, r.URL.Query().Get("category"), at, 300)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleGlobalGraph(w http.ResponseWriter, r *http.Request) {
	depth, _ := strconv.Atoi(r.URL.Query().Get("depth"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	at, err := optionalRFC3339(r.URL.Query().Get("at"))
	if err != nil {
		writeError(w, err)
		return
	}
	result, err := s.Store.DecisionGraph(r.Context(), userFrom(r), nil, depth, r.URL.Query().Get("category"), at, limit)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleReviewQueue(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := s.Store.ReviewQueue(r.Context(), userFrom(r), limit)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleReviewDecision(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var input struct {
		Note         string     `json:"note"`
		Confidence   *int       `json:"confidence"`
		ReviewedAt   *time.Time `json:"reviewedAt"`
		NextReviewAt *time.Time `json:"nextReviewAt"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	reviewedAt := time.Now().UTC()
	if input.ReviewedAt != nil {
		reviewedAt = *input.ReviewedAt
	}
	result, err := s.Store.ReviewDecision(r.Context(), userFrom(r), id, input.Note, input.Confidence, reviewedAt, input.NextReviewAt)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) handleBiasProfile(w http.ResponseWriter, r *http.Request) {
	result, err := s.Store.Analytics(r.Context(), userFrom(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": result.Biases})
}

func (s *Server) handlePatternIntelligence(w http.ResponseWriter, r *http.Request) {
	result, err := s.Store.Analytics(r.Context(), userFrom(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": result.Patterns})
}

func (s *Server) handleDecisionProfile(w http.ResponseWriter, r *http.Request) {
	result, err := s.Store.Analytics(r.Context(), userFrom(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result.Profile)
}

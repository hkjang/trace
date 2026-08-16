package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hkjang/trace/internal/store"
)

func (s *Server) handleListPersonalKeys(w http.ResponseWriter, r *http.Request) {
	items, err := s.Store.ListPersonalKeys(r.Context(), userFrom(r).ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
func (s *Server) handleCreatePersonalKey(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name        string     `json:"name"`
		Kind        string     `json:"kind"`
		Value       string     `json:"value"`
		Permissions []string   `json:"permissions"`
		ExpiresAt   *time.Time `json:"expiresAt"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.Store.CreatePersonalKey(r.Context(), userFrom(r).ID, input.Name, input.Kind, input.Value, input.Permissions, input.ExpiresAt)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}
func (s *Server) handleRotatePersonalKey(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, store.ErrNotFound)
		return
	}
	var input struct {
		Value string `json:"value"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.Store.RotatePersonalKey(r.Context(), userFrom(r).ID, id, input.Value)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (s *Server) handlePersonalKeyPermissions(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, store.ErrNotFound)
		return
	}
	var input struct {
		Permissions []string `json:"permissions"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.Store.UpdatePersonalKeyPermissions(r.Context(), userFrom(r).ID, id, input.Permissions); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) handleRevokePersonalKey(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, store.ErrNotFound)
		return
	}
	if err := s.Store.RevokePersonalKey(r.Context(), userFrom(r).ID, id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) handleRotateDataKey(w http.ResponseWriter, r *http.Request) {
	user := userFrom(r)
	if err := s.Store.RotateUserDataKey(r.Context(), user, user); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListTokens(w http.ResponseWriter, r *http.Request) {
	items, err := s.Store.ListAPITokens(r.Context(), userFrom(r).ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
func (s *Server) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name      string     `json:"name"`
		Scopes    []string   `json:"scopes"`
		ExpiresAt *time.Time `json:"expiresAt"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, plain, err := s.Store.CreateAPIToken(r.Context(), userFrom(r).ID, input.Name, input.Scopes, input.ExpiresAt)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"token": item, "plainTextToken": plain})
}
func (s *Server) handleRevokeToken(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, store.ErrNotFound)
		return
	}
	if err := s.Store.RevokeAPIToken(r.Context(), userFrom(r).ID, id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

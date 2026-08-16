package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hkjang/trace/internal/domain"
	"github.com/hkjang/trace/internal/store"
)

func (s *Server) handleAdminSettings(w http.ResponseWriter, r *http.Request) {
	oidcSettings, err := s.Store.GetOIDCSettings(r.Context(), false)
	if err != nil {
		writeError(w, err)
		return
	}
	ai, err := s.Store.GetAISettings(r.Context(), false)
	if err != nil {
		writeError(w, err)
		return
	}
	workflow, _ := s.Store.GetWorkflowSettings(r.Context())
	branding, _ := s.Store.GetBrandingSettings(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"oidc": oidcSettings, "ai": ai, "workflow": workflow, "branding": branding, "limits": map[string]int{"maxTokens": 262144}})
}
func (s *Server) handleSaveOIDC(w http.ResponseWriter, r *http.Request) {
	var input domain.OIDCSettings
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.Store.SaveOIDCSettings(r.Context(), userFrom(r).ID, input); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"saved": true, "redirectUrl": input.BaseURL + "/api/v1/auth/oidc/callback"})
}
func (s *Server) handleSaveAI(w http.ResponseWriter, r *http.Request) {
	var input domain.AISettings
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.Store.SaveAISettings(r.Context(), userFrom(r).ID, input); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"saved": true})
}
func (s *Server) handleSaveWorkflow(w http.ResponseWriter, r *http.Request) {
	var input domain.WorkflowSettings
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.Store.SaveWorkflowSettings(r.Context(), userFrom(r).ID, input); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, input)
}
func (s *Server) handleSaveBranding(w http.ResponseWriter, r *http.Request) {
	var input domain.BrandingSettings
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.Store.SaveBrandingSettings(r.Context(), userFrom(r).ID, input); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, input)
}
func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	items, err := s.Store.ListUsers(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email       string `json:"email"`
		Username    string `json:"username"`
		DisplayName string `json:"displayName"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.Store.CreateUser(r.Context(), userFrom(r), input.Email, input.Username, input.DisplayName)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}
func (s *Server) handleUserStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, store.ErrNotFound)
		return
	}
	var input struct {
		Status string `json:"status"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.Store.SetUserStatus(r.Context(), userFrom(r), id, input.Status); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) handleUserRoles(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, store.ErrNotFound)
		return
	}
	var input struct {
		RoleIDs []uuid.UUID `json:"roleIds"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.Store.SetUserRoles(r.Context(), userFrom(r), id, input.RoleIDs); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) handleListRoles(w http.ResponseWriter, r *http.Request) {
	items, err := s.Store.ListRoles(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
func (s *Server) handleRolePermissions(w http.ResponseWriter, r *http.Request) {
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
	if err := s.Store.UpdateRolePermissions(r.Context(), userFrom(r), id, input.Permissions); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListTeams(w http.ResponseWriter, r *http.Request) {
	items, err := s.Store.ListTeams(r.Context(), userFrom(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
func (s *Server) handleAdminTeams(w http.ResponseWriter, r *http.Request) { s.handleListTeams(w, r) }
func (s *Server) handleCreateTeam(w http.ResponseWriter, r *http.Request) { s.saveTeam(w, r, nil) }
func (s *Server) handleUpdateTeam(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, store.ErrNotFound)
		return
	}
	s.saveTeam(w, r, &id)
}
func (s *Server) saveTeam(w http.ResponseWriter, r *http.Request, id *uuid.UUID) {
	var input struct {
		Name          string      `json:"name"`
		Description   string      `json:"description"`
		ManagerUserID *uuid.UUID  `json:"managerUserId"`
		MemberIDs     []uuid.UUID `json:"memberIds"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.Store.SaveTeam(r.Context(), userFrom(r), id, input.Name, input.Description, input.ManagerUserID, input.MemberIDs)
	if err != nil {
		writeError(w, err)
		return
	}
	status := http.StatusOK
	if id == nil {
		status = http.StatusCreated
	}
	writeJSON(w, status, item)
}

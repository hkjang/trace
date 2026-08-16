package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/hkjang/trace/internal/domain"
	"github.com/hkjang/trace/internal/store"
	"github.com/hkjang/trace/internal/version"
)

type contextKey string

const userContextKey contextKey = "trace-user"
const scopesContextKey contextKey = "trace-token-scopes"

type Server struct {
	Store      *store.Store
	Logger     *slog.Logger
	HTTPClient *http.Client
}

func New(s *store.Store, logger *slog.Logger) *Server {
	return &Server{Store: s, Logger: logger, HTTPClient: &http.Client{Timeout: 5 * time.Minute}}
}

func (s *Server) Router(static http.Handler) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer, s.securityHeaders, s.accessLog)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})
	r.Get("/api/v1/version", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, version.Current()) })
	r.Get("/api/v1/public/config", s.handlePublicConfig)
	r.Post("/api/v1/auth/login", s.handleLocalLogin)
	r.Get("/api/v1/auth/oidc/start", s.handleOIDCStart)
	r.Get("/api/v1/auth/oidc/callback", s.handleOIDCCallback)
	r.Group(func(api chi.Router) {
		api.Use(s.authenticate)
		api.Use(s.csrfGuard)
		api.Get("/api/v1/me", s.handleMe)
		api.Post("/api/v1/auth/logout", s.handleLogout)
		api.Get("/api/v1/dashboard", s.handleDashboard)
		api.Get("/api/v1/analytics/calibration", s.handleAnalytics)
		api.Get("/api/v1/teams", s.handleListTeams)
		api.Get("/api/v1/decisions", s.handleListDecisions)
		api.Post("/api/v1/decisions", s.require("decisions.write", s.handleCreateDecision))
		api.Get("/api/v1/decisions/{id}", s.handleGetDecision)
		api.Patch("/api/v1/decisions/{id}", s.require("decisions.write", s.handleUpdateDecision))
		api.Get("/api/v1/decisions/{id}/replay", s.handleReplayDecision)
		api.Post("/api/v1/decisions/{id}/evidence", s.require("decisions.write", s.handleAddEvidence))
		api.Post("/api/v1/decisions/{id}/outcomes", s.require("decisions.write", s.handleAddOutcome))
		api.Post("/api/v1/decisions/{id}/reflections", s.require("decisions.write", s.handleAddReflection))
		api.Post("/api/v1/decisions/{id}/approval", s.require("decisions.write", s.handleSubmitApproval))
		api.Get("/api/v1/approvals", s.require("decisions.approve", s.handleListApprovals))
		api.Post("/api/v1/approvals/{id}/{action}", s.require("decisions.approve", s.handleReviewApproval))
		api.Post("/api/v1/decisions/{id}/ai/stream", s.require("ai.use", s.handleAIStream))
		api.Get("/api/v1/personal/keys", s.handleListPersonalKeys)
		api.Post("/api/v1/personal/keys", s.require("keys.manage_own", s.handleCreatePersonalKey))
		api.Post("/api/v1/personal/keys/{id}/rotate", s.require("keys.manage_own", s.handleRotatePersonalKey))
		api.Patch("/api/v1/personal/keys/{id}/permissions", s.require("keys.manage_own", s.handlePersonalKeyPermissions))
		api.Delete("/api/v1/personal/keys/{id}", s.require("keys.manage_own", s.handleRevokePersonalKey))
		api.Post("/api/v1/personal/data-key/rotate", s.require("keys.manage_own", s.handleRotateDataKey))
		api.Get("/api/v1/personal/tokens", s.require("tokens.manage", s.handleListTokens))
		api.Post("/api/v1/personal/tokens", s.require("tokens.manage", s.handleCreateToken))
		api.Delete("/api/v1/personal/tokens/{id}", s.require("tokens.manage", s.handleRevokeToken))
		api.Route("/api/v1/admin", func(admin chi.Router) {
			admin.Use(s.requireMiddleware("admin.access"))
			admin.Get("/settings", s.handleAdminSettings)
			admin.Put("/settings/oidc", s.require("settings.manage", s.handleSaveOIDC))
			admin.Put("/settings/ai", s.require("settings.manage", s.handleSaveAI))
			admin.Put("/settings/workflow", s.require("settings.manage", s.handleSaveWorkflow))
			admin.Put("/settings/branding", s.require("settings.manage", s.handleSaveBranding))
			admin.Get("/users", s.require("users.manage", s.handleListUsers))
			admin.Post("/users", s.require("users.manage", s.handleCreateUser))
			admin.Patch("/users/{id}/status", s.require("users.manage", s.handleUserStatus))
			admin.Put("/users/{id}/roles", s.require("users.manage", s.handleUserRoles))
			admin.Get("/roles", s.require("roles.manage", s.handleListRoles))
			admin.Put("/roles/{id}/permissions", s.require("roles.manage", s.handleRolePermissions))
			admin.Get("/teams", s.require("users.manage", s.handleAdminTeams))
			admin.Post("/teams", s.require("users.manage", s.handleCreateTeam))
			admin.Put("/teams/{id}", s.require("users.manage", s.handleUpdateTeam))
		})
	})
	r.Handle("/mcp", s.mcpHandler())
	r.Handle("/*", static)
	return r
}

func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var user domain.User
		var scopes []string
		var err error
		if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			user, scopes, err = s.Store.AuthenticateAPIToken(r.Context(), strings.TrimSpace(strings.TrimPrefix(auth, "Bearer ")))
		} else if cookie, cookieErr := r.Cookie("trace_session"); cookieErr == nil {
			user, err = s.Store.UserFromSession(r.Context(), cookie.Value)
		} else {
			err = store.ErrUnauthorized
		}
		if err != nil {
			writeError(w, store.ErrUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), userContextKey, user)
		ctx = context.WithValue(ctx, scopesContextKey, scopes)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
func userFrom(r *http.Request) domain.User { return r.Context().Value(userContextKey).(domain.User) }
func (s *Server) require(permission string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !userFrom(r).Can(permission) {
			writeError(w, store.ErrForbidden)
			return
		}
		handler(w, r)
	}
}
func (s *Server) requireMiddleware(permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler { return http.HandlerFunc(s.require(permission, next.ServeHTTP)) }
}

func (s *Server) csrfGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		// Bearer tokens are not ambient browser credentials and are protected by
		// their explicit scopes. Cookie requests must prove same-origin script use.
		if scopes, _ := r.Context().Value(scopesContextKey).([]string); len(scopes) > 0 {
			next.ServeHTTP(w, r)
			return
		}
		if r.Header.Get("X-Trace-Request") != "1" {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]string{"code": "csrf", "message": "요청 출처를 확인할 수 없습니다."}})
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			parsed, err := url.Parse(origin)
			if err != nil || !strings.EqualFold(parsed.Host, r.Host) {
				writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]string{"code": "csrf", "message": "허용되지 않은 요청 출처입니다."}})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; font-src 'self'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}
func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.Logger.Info("http request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start), "request_id", middleware.GetReqID(r.Context()))
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_json", "message": err.Error()}})
		return false
	}
	return true
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "internal_error"
	message := "요청을 처리하지 못했습니다."
	switch {
	case errors.Is(err, store.ErrUnauthorized):
		status = http.StatusUnauthorized
		code = "unauthorized"
		message = "로그인이 필요합니다."
	case errors.Is(err, store.ErrForbidden):
		status = http.StatusForbidden
		code = "forbidden"
		message = "이 작업을 수행할 권한이 없습니다."
	case errors.Is(err, store.ErrNotFound):
		status = http.StatusNotFound
		code = "not_found"
		message = "요청한 항목을 찾을 수 없습니다."
	case errors.Is(err, store.ErrConflict):
		status = http.StatusConflict
		code = "conflict"
		message = "현재 상태와 충돌합니다."
	case errors.Is(err, store.ErrValidation):
		status = http.StatusUnprocessableEntity
		code = "validation"
		message = err.Error()
	}
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

package httpapi

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/hkjang/trace/internal/domain"
	"github.com/hkjang/trace/internal/store"
	"github.com/hkjang/trace/internal/version"
	"golang.org/x/oauth2"
)

func (s *Server) handlePublicConfig(w http.ResponseWriter, r *http.Request) {
	branding, err := s.Store.GetBrandingSettings(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	oidcSettings, err := s.Store.GetOIDCSettings(r.Context(), false)
	if err != nil {
		writeError(w, err)
		return
	}
	workflow, _ := s.Store.GetWorkflowSettings(r.Context())
	// autoLogin is published so the browser knows whether to try a silent
	// sign-in before showing the login screen.
	writeJSON(w, http.StatusOK, map[string]any{"branding": branding, "oidc": map[string]any{"enabled": oidcSettings.Enabled, "autoLogin": oidcSettings.Enabled && oidcSettings.AutoLogin, "loginUrl": "/api/v1/auth/oidc/start"}, "workflow": workflow, "version": version.Current()})
}

func (s *Server) handleLocalLogin(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Identity string `json:"identity"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	user, err := s.Store.AuthenticateLocal(r.Context(), input.Identity, input.Password)
	if err != nil {
		writeError(w, store.ErrUnauthorized)
		return
	}
	token, expires, err := s.Store.CreateSession(r.Context(), user.ID, "local", r.UserAgent(), r.RemoteAddr, 12*time.Hour)
	if err != nil {
		writeError(w, err)
		return
	}
	setSessionCookie(w, r, token, expires)
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("trace_session"); err == nil {
		_ = s.Store.DeleteSession(r.Context(), cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "trace_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"user": userFrom(r), "version": version.Current()})
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expires time.Time) {
	secure := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	http.SetCookie(w, &http.Cookie{Name: "trace_session", Value: token, Path: "/", Expires: expires, MaxAge: int(time.Until(expires).Seconds()), HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode})
}

func (s *Server) handleOIDCStart(w http.ResponseWriter, r *http.Request) {
	settings, err := s.Store.GetOIDCSettings(r.Context(), true)
	if err != nil || !settings.Enabled {
		writeError(w, store.ErrNotFound)
		return
	}
	provider, oauthConfig, err := oidcClient(r.Context(), settings)
	if err != nil {
		s.Logger.Error("OIDC discovery failed", "error", err)
		writeError(w, err)
		return
	}
	_ = provider
	state, _ := randomURLToken(32)
	nonce, _ := randomURLToken(24)
	verifier, _ := randomURLToken(32)
	pending := store.OAuthState{Nonce: nonce, Verifier: verifier, ReturnTo: safeReturnTo(r.URL.Query().Get("returnTo")), Silent: silentOIDCAttempt(r.URL.Query().Get("prompt"), settings)}
	if err := s.Store.CreateOAuthState(r.Context(), state, pending); err != nil {
		writeError(w, err)
		return
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	options := []oauth2.AuthCodeOption{oidc.Nonce(nonce), oauth2.SetAuthURLParam("code_challenge", challenge), oauth2.SetAuthURLParam("code_challenge_method", "S256")}
	if pending.Silent {
		// prompt=none asks the provider to answer from an existing session only.
		// It never renders UI: either a code comes straight back, or an error
		// such as login_required does.
		options = append(options, oauth2.SetAuthURLParam("prompt", "none"))
	}
	http.Redirect(w, r, oauthConfig.AuthCodeURL(state, options...), http.StatusFound)
}

// silentOIDCAttempt decides whether this start request becomes a prompt=none
// attempt. Refusing an unrequested silent attempt keeps the redirect surface
// tied to the administrator setting: anyone can append ?prompt=none to a URL,
// but only auto_login turns it on.
func silentOIDCAttempt(prompt string, settings domain.OIDCSettings) bool {
	return prompt == "none" && settings.Enabled && settings.AutoLogin
}

// safeReturnTo only accepts a same-origin path, so the login flow cannot be
// used as an open redirect. Anything else lands on the home page.
func safeReturnTo(value string) string {
	if !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") || strings.HasPrefix(value, "/\\") || strings.ContainsAny(value, "\r\n") {
		return "/"
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.IsAbs() || parsed.Host != "" {
		return "/"
	}
	return value
}

// providerErrorRedirect maps an error the provider sent to the callback onto
// the login page. prompt=none produces login_required whenever no session
// exists, which is an ordinary outcome: the browser is sent to /login?sso=none
// so it has a marker telling it not to retry, which is what stops a
// signed-out visitor from being bounced in a loop.
func providerErrorRedirect(oidcErr string, silent bool) string {
	if silent {
		return "/login?sso=none"
	}
	return "/login?error=" + url.QueryEscape(oidcErr)
}

func (s *Server) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	if oidcErr := r.URL.Query().Get("error"); oidcErr != "" {
		silent := false
		if state := r.URL.Query().Get("state"); state != "" {
			if pending, err := s.Store.ConsumeOAuthState(r.Context(), state); err == nil {
				silent = pending.Silent
			}
		}
		if !silent {
			s.Logger.Warn("OIDC provider returned an error", "error", oidcErr)
		}
		http.Redirect(w, r, providerErrorRedirect(oidcErr, silent), http.StatusFound)
		return
	}
	pending, err := s.Store.ConsumeOAuthState(r.Context(), r.URL.Query().Get("state"))
	if err != nil {
		http.Redirect(w, r, "/login?error=invalid_state", http.StatusFound)
		return
	}
	nonce, verifier, returnTo := pending.Nonce, pending.Verifier, safeReturnTo(pending.ReturnTo)
	settings, err := s.Store.GetOIDCSettings(r.Context(), true)
	if err != nil {
		writeError(w, err)
		return
	}
	provider, oauthConfig, err := oidcClient(r.Context(), settings)
	if err != nil {
		writeError(w, err)
		return
	}
	token, err := oauthConfig.Exchange(r.Context(), r.URL.Query().Get("code"), oauth2.SetAuthURLParam("code_verifier", verifier))
	if err != nil {
		http.Redirect(w, r, "/login?error=exchange_failed", http.StatusFound)
		return
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		http.Redirect(w, r, "/login?error=missing_id_token", http.StatusFound)
		return
	}
	idToken, err := provider.Verifier(&oidc.Config{ClientID: settings.ClientID}).Verify(r.Context(), rawIDToken)
	if err != nil || idToken.Nonce != nonce {
		http.Redirect(w, r, "/login?error=invalid_id_token", http.StatusFound)
		return
	}
	claims := map[string]any{}
	if err := idToken.Claims(&claims); err != nil {
		writeError(w, err)
		return
	}
	email := claimString(claims, settings.EmailClaim)
	username := claimString(claims, settings.UsernameClaim)
	display := claimString(claims, settings.DisplayClaim)
	user, err := s.Store.ResolveOIDCUser(r.Context(), settings.IssuerURL, idToken.Subject, email, username, display, claims, settings.AutoProvision)
	if err != nil {
		http.Redirect(w, r, "/login?error=provision_failed", http.StatusFound)
		return
	}
	session, expires, err := s.Store.CreateSession(r.Context(), user.ID, "oidc", r.UserAgent(), r.RemoteAddr, 12*time.Hour)
	if err != nil {
		writeError(w, err)
		return
	}
	setSessionCookie(w, r, session, expires)
	http.Redirect(w, r, returnTo, http.StatusFound)
}

func claimString(claims map[string]any, key string) string {
	if value, ok := claims[key].(string); ok {
		return value
	}
	return ""
}
func randomURLToken(size int) (string, error) { return storeRandomToken(size) }

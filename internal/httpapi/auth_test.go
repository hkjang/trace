package httpapi

import (
	"testing"

	"github.com/hkjang/trace/internal/domain"
)

func TestSilentOIDCAttemptRequiresAutoLogin(t *testing.T) {
	enabled := domain.OIDCSettings{Enabled: true, AutoLogin: true}
	if !silentOIDCAttempt("none", enabled) {
		t.Fatal("prompt=none with auto_login on must start a silent attempt")
	}
	// Anyone can append ?prompt=none to a URL; only the administrator setting
	// may turn it into a silent attempt.
	if silentOIDCAttempt("none", domain.OIDCSettings{Enabled: true, AutoLogin: false}) {
		t.Fatal("prompt=none must fall back to an ordinary login when auto_login is off")
	}
	if silentOIDCAttempt("none", domain.OIDCSettings{Enabled: false, AutoLogin: true}) {
		t.Fatal("prompt=none must not be honoured when OIDC is disabled")
	}
	if silentOIDCAttempt("", enabled) || silentOIDCAttempt("login", enabled) {
		t.Fatal("only prompt=none is a silent attempt")
	}
}

func TestProviderErrorRedirectMarksSilentRefusal(t *testing.T) {
	if got := providerErrorRedirect("login_required", true); got != "/login?sso=none" {
		t.Fatalf("silent refusal must land on /login?sso=none, got %q", got)
	}
	if got := providerErrorRedirect("access_denied", false); got != "/login?error=access_denied" {
		t.Fatalf("ordinary provider error should keep the error code, got %q", got)
	}
	if got := providerErrorRedirect("a b&c", false); got != "/login?error=a+b%26c" {
		t.Fatalf("provider error must be query-escaped, got %q", got)
	}
}

func TestSafeReturnToOnlyAcceptsLocalPaths(t *testing.T) {
	accepted := []string{"/", "/decisions/42", "/search?q=a%20b", "/insights#top"}
	for _, value := range accepted {
		if got := safeReturnTo(value); got != value {
			t.Fatalf("safeReturnTo(%q) = %q, want unchanged", value, got)
		}
	}
	rejected := []string{"", "https://evil.example/", "//evil.example/path", "/\\evil.example", "evil", "/x\r\nLocation: y", "javascript:alert(1)"}
	for _, value := range rejected {
		if got := safeReturnTo(value); got != "/" {
			t.Fatalf("safeReturnTo(%q) = %q, want /", value, got)
		}
	}
}

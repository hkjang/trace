package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerFallsBackToIndexForBrowserRoutes(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/decisions/00000000-0000-0000-0000-000000000000", nil)
	recorder := httptest.NewRecorder()
	Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `<div id="root">`) {
		t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("Cache-Control = %q", recorder.Header().Get("Cache-Control"))
	}
}

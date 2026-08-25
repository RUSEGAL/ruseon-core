package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/RUSEGAL/ruseon-core/v2/pkg/auth"
	"github.com/RUSEGAL/ruseon-core/v2/pkg/config"
)

func TestRouterCORS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &Handler{}                       // mock enough for router middleware test
	authenticator := &auth.LocalAuthenticator{} // mock

	tests := []struct {
		name          string
		allowed       []string
		origin        string
		expectAllowed bool
	}{
		{"Case 1: empty allowed, evil origin", []string{}, "https://evil.com", false},
		{"Case 2: match allowed origin", []string{"https://dashboard.example.com"}, "https://dashboard.example.com", true},
		{"Case 3: no match origin", []string{"https://dashboard.example.com"}, "https://evil.com", false},
		{"Case 4: wildcard allowed", []string{"*"}, "https://evil.com", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := SetupRouter(handler, authenticator, false, tc.allowed)
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("OPTIONS", "/api/cameras", nil)
			req.Header.Set("Origin", tc.origin)
			router.ServeHTTP(w, req)

			if len(tc.allowed) > 0 && w.Code != http.StatusNoContent {
				t.Errorf("expected 204 for OPTIONS, got %d", w.Code)
			}

			originHeader := w.Header().Get("Access-Control-Allow-Origin")
			if tc.expectAllowed {
				if originHeader != tc.origin {
					t.Errorf("expected Allow-Origin %q, got %q", tc.origin, originHeader)
				}
				if w.Header().Get("Access-Control-Allow-Credentials") != "true" {
					t.Errorf("expected Allow-Credentials true")
				}
				if w.Header().Get("Vary") != "Origin" {
					t.Errorf("expected Vary: Origin")
				}
			} else if originHeader != "" {
				t.Errorf("expected no Allow-Origin header, got %q", originHeader)
			}
		})
	}
}

func TestWSOriginCheck(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.CORSAllowedOrigins = []string{"https://dashboard.example.com"}
	h := &Handler{cfg: cfg}

	// 1. Matching origin
	req1, _ := http.NewRequest("GET", "/stream/ws/cam1", nil)
	req1.Header.Set("Origin", "https://dashboard.example.com")
	if !h.checkWSOrigin(req1) {
		t.Errorf("expected origin to be allowed")
	}

	// 2. Non-matching origin
	req2, _ := http.NewRequest("GET", "/stream/ws/cam1", nil)
	req2.Header.Set("Origin", "https://evil.com")
	if h.checkWSOrigin(req2) {
		t.Errorf("expected evil origin to be rejected")
	}

	// 3. No origin header (non-browser client)
	req3, _ := http.NewRequest("GET", "/stream/ws/cam1", nil)
	if !h.checkWSOrigin(req3) {
		t.Errorf("expected empty origin to be allowed")
	}

	// 4. Wildcard origin
	cfgWildcard := &config.Config{}
	cfgWildcard.Server.CORSAllowedOrigins = []string{"*"}
	hWildcard := &Handler{cfg: cfgWildcard}
	req4, _ := http.NewRequest("GET", "/stream/ws/cam1", nil)
	req4.Header.Set("Origin", "https://anything.com")
	if !hWildcard.checkWSOrigin(req4) {
		t.Errorf("expected wildcard origin to be allowed")
	}
}

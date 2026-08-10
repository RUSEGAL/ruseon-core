package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/RUSEGAL/ruseon-core/pkg/auth"
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
			} else {
				if originHeader != "" {
					t.Errorf("expected no Allow-Origin header, got %q", originHeader)
				}
			}
		})
	}
}

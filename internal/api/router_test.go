package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/RUSEGAL/REA-Stream-Engine/internal/config"
	"github.com/RUSEGAL/REA-Stream-Engine/internal/storage"
	"github.com/RUSEGAL/REA-Stream-Engine/internal/stream"
)

func TestRouterMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tempDir := t.TempDir()
	store, _ := storage.NewStorage(filepath.Join(tempDir, "db"))
	defer store.Close()

	cfg := &config.Config{}
	cfg.Auth.Username = "admin"
	cfg.Auth.Password = "password"
	cfg.Auth.Secret = "secret"

	manager := stream.NewManager()
	handler := NewHandler(manager, cfg, store)

	// debug=true turns on statsviz and pprof
	router := SetupRouter(handler, true)

	// 1. Test CORS Options
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("OPTIONS", "/api/cameras", nil)
	router.ServeHTTP(w1, req1)
	if w1.Code != http.StatusNoContent { // 204
		t.Errorf("expected 204 for OPTIONS, got %d", w1.Code)
	}
	if w1.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected CORS header")
	}

	// 2. Test Auth Middleware - No Token
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/cameras", nil)
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing token, got %d", w2.Code)
	}

	// 3. Test Auth Middleware - Invalid Token
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("GET", "/api/cameras", nil)
	req3.Header.Set("Authorization", "Bearer invalidtoken")
	router.ServeHTTP(w3, req3)
	if w3.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid token, got %d", w3.Code)
	}

	// 4. Test Auth Middleware - Valid Token (Query param)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"username": "admin"})
	tokenString, _ := token.SignedString([]byte("secret"))
	w4 := httptest.NewRecorder()
	req4, _ := http.NewRequest("GET", "/api/cameras?token="+tokenString, nil)
	router.ServeHTTP(w4, req4)
	if w4.Code != http.StatusOK {
		t.Errorf("expected 200 for valid token, got %d", w4.Code)
	}

	// 5. Test Auth Middleware - Valid Token (Header)
	w5 := httptest.NewRecorder()
	req5, _ := http.NewRequest("GET", "/api/cameras", nil)
	req5.Header.Set("Authorization", "Bearer "+tokenString)
	router.ServeHTTP(w5, req5)
	if w5.Code != http.StatusOK {
		t.Errorf("expected 200 for valid token header, got %d", w5.Code)
	}

	// 6. Test Logging path skipping (/api/logs/stream)
	// Just make sure it returns 200 (or streams) and doesn't crash the logger
	w6 := httptest.NewRecorder()
	req6, _ := http.NewRequest("GET", "/api/logs/stream", nil)
	req6.Header.Set("Authorization", "Bearer "+tokenString)
	// We run it briefly in a goroutine because it blocks for SSE
	// But we just want to test routing
	// Wait, StreamLogs is a blocking call. We should cancel context.
	ctx, cancel := context.WithCancel(req6.Context())
	cancel() // Cancel immediately
	req6 = req6.WithContext(ctx)
	router.ServeHTTP(w6, req6)
	// Actually req.Context() will be canceled
}

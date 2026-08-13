package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/RUSEGAL/ruseon-core/internal/models"
	"github.com/RUSEGAL/ruseon-core/internal/stream"
	"github.com/RUSEGAL/ruseon-core/pkg/auth"
	"github.com/RUSEGAL/ruseon-core/pkg/config"
	"github.com/RUSEGAL/ruseon-core/pkg/storage"
)

func TestRouterMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tempDir := t.TempDir()
	store, _ := storage.NewStorage(filepath.Join(tempDir, "db"))
	defer store.Close()

	cfg := &config.Config{}
	cfg.Auth.Secret = "secret"

	manager := stream.NewManager()
	handler := NewHandler(manager, cfg, store)
	authenticator := auth.NewLocalAuthenticator(cfg)

	// debug=true turns on statsviz and pprof
	router := SetupRouter(handler, authenticator, true, nil)

	// 1. Test Auth Middleware - No Token
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
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"username": "admin", "role": string(models.RoleAdmin)})
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

func TestRouterMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tempDir := t.TempDir()
	store, _ := storage.NewStorage(filepath.Join(tempDir, "db"))
	defer store.Close()

	cfg := &config.Config{}
	manager := stream.NewManager()
	handler := NewHandler(manager, cfg, store)
	authenticator := auth.NewLocalAuthenticator(cfg)

	router := SetupRouter(handler, authenticator, false, nil)

	// Test /metrics endpoint
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/metrics", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for /metrics, got %d", w.Code)
	}

	// Verify response contains Prometheus formatting
	if !strings.Contains(w.Body.String(), "go_goroutines") {
		t.Errorf("expected metrics output to contain go_goroutines, got: %s", w.Body.String())
	}
}

func TestRBAC(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tempDir := t.TempDir()
	store, _ := storage.NewStorage(filepath.Join(tempDir, "db"))
	defer store.Close()

	cfg := &config.Config{}
	cfg.Auth.Secret = "secret"
	manager := stream.NewManager()
	handler := NewHandler(manager, cfg, store)
	authenticator := auth.NewLocalAuthenticator(cfg)

	router := SetupRouter(handler, authenticator, false, nil)

	getToken := func(role models.Role) string {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"username": "test", "role": string(role)})
		tokenString, _ := token.SignedString([]byte("secret"))
		return tokenString
	}

	// 1. Viewer trying to POST /api/cameras (Should be 403 Forbidden)
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("POST", "/api/cameras", nil)
	req1.Header.Set("Authorization", "Bearer "+getToken(models.RoleViewer))
	router.ServeHTTP(w1, req1)
	if w1.Code != http.StatusForbidden {
		t.Errorf("expected 403 for Viewer trying to write, got %d", w1.Code)
	}

	// 2. Operator trying to GET /api/system/backup/export (Should be 403 Forbidden)
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/system/backup/export", nil)
	req2.Header.Set("Authorization", "Bearer "+getToken(models.RoleOperator))
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusForbidden {
		t.Errorf("expected 403 for Operator trying to backup, got %d", w2.Code)
	}

	// 3. Admin trying to GET /api/system/backup/export (Should NOT be 403)
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("GET", "/api/system/backup/export", nil)
	req3.Header.Set("Authorization", "Bearer "+getToken(models.RoleAdmin))
	router.ServeHTTP(w3, req3)
	if w3.Code == http.StatusForbidden || w3.Code == http.StatusUnauthorized {
		t.Errorf("expected Admin to pass RBAC, got %d", w3.Code)
	}
}

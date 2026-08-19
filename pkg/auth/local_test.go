package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/RUSEGAL/ruseon-core/internal/models"
	"github.com/RUSEGAL/ruseon-core/pkg/config"
	"github.com/RUSEGAL/ruseon-core/pkg/registry"
	"github.com/RUSEGAL/ruseon-core/pkg/storage"
)

func setupTestAuth(t *testing.T) (*LocalAuthenticator, *storage.Storage) {
	gin.SetMode(gin.TestMode)
	store, err := storage.NewStorage(t.TempDir())
	require.NoError(t, err)
	registry.RegisterStateStore(store)

	cfg := &config.Config{}
	cfg.Auth.Secret = "test-secret-key-1234567890-secure"

	auth := NewLocalAuthenticator(cfg)
	return auth, store
}

func TestLocalAuthenticator_New_InitialAdmin(t *testing.T) {
	_, store := setupTestAuth(t)
	defer store.Close()

	hasUsers, err := store.HasUsers()
	require.NoError(t, err)
	assert.True(t, hasUsers)

	admin, err := store.GetUser("admin")
	require.NoError(t, err)
	require.NotNil(t, admin)
	assert.Equal(t, "admin", admin.Username)
	assert.Equal(t, models.RoleAdmin, admin.Role)
	assert.NotEmpty(t, admin.PasswordHash)
}

func TestLocalAuthenticator_Login(t *testing.T) {
	auth, store := setupTestAuth(t)
	defer store.Close()

	// Create known user
	hash, err := bcrypt.GenerateFromPassword([]byte("testpassword123"), bcrypt.DefaultCost)
	require.NoError(t, err)
	err = store.SaveUser(&models.User{
		Username:     "operator1",
		PasswordHash: string(hash),
		Role:         models.RoleOperator,
	})
	require.NoError(t, err)

	router := gin.New()
	router.POST("/api/login", auth.Login)

	t.Run("successful login", func(t *testing.T) {
		body, _ := json.Marshal(Request{Username: "operator1", Password: "testpassword123"})
		req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.NotEmpty(t, resp["token"])
	})

	t.Run("invalid password", func(t *testing.T) {
		body, _ := json.Marshal(Request{Username: "operator1", Password: "wrongpassword"})
		req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("non-existent user", func(t *testing.T) {
		body, _ := json.Marshal(Request{Username: "nobody", Password: "testpassword123"})
		req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("empty username", func(t *testing.T) {
		body, _ := json.Marshal(Request{Username: "", Password: "testpassword123"})
		req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("invalid json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestLocalAuthenticator_Middleware(t *testing.T) {
	auth, store := setupTestAuth(t)
	defer store.Close()

	hash, err := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.DefaultCost)
	require.NoError(t, err)
	err = store.SaveUser(&models.User{
		Username:     "viewer1",
		PasswordHash: string(hash),
		Role:         models.RoleViewer,
	})
	require.NoError(t, err)

	router := gin.New()
	router.Use(auth.Middleware())
	router.GET("/api/protected", func(c *gin.Context) {
		user, _ := c.Get("username")
		role, _ := c.Get("role")
		c.JSON(http.StatusOK, gin.H{"user": user, "role": role})
	})

	// Helper to generate valid token
	makeToken := func(username string, role models.Role, exp time.Duration) string {
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"username": username,
			"role":     string(role),
			"exp":      time.Now().Add(exp).Unix(),
		})
		str, _ := tok.SignedString([]byte(auth.cfg.Auth.Secret))
		return str
	}

	t.Run("valid Bearer token", func(t *testing.T) {
		tok := makeToken("viewer1", models.RoleViewer, time.Hour)
		req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("reject token in query param", func(t *testing.T) {
		tok := makeToken("viewer1", models.RoleViewer, time.Hour)
		req := httptest.NewRequest(http.MethodGet, "/api/protected?token="+tok, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("missing token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("expired token", func(t *testing.T) {
		tok := makeToken("viewer1", models.RoleViewer, -time.Hour)
		req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("reject stream token on main API", func(t *testing.T) {
		streamTok, err := auth.GenerateStreamToken("cam-1")
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
		req.Header.Set("Authorization", "Bearer "+streamTok)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("revoked user", func(t *testing.T) {
		tok := makeToken("deleted_user", models.RoleViewer, time.Hour)
		req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("role changed", func(t *testing.T) {
		tok := makeToken("viewer1", models.RoleAdmin, time.Hour) // Token claims admin, but db has viewer
		req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestRequireRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/admin-only", func(c *gin.Context) {
		role := c.Query("role")
		if role != "" {
			c.Set("role", role)
		}
		c.Next()
	}, RequireRole(models.RoleAdmin), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	t.Run("authorized role", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin-only?role=admin", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("insufficient permissions", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin-only?role=viewer", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("missing role in context", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

func TestLocalAuthenticator_StreamMiddleware(t *testing.T) {
	auth, store := setupTestAuth(t)
	defer store.Close()

	router := gin.New()
	router.GET("/stream/hls/:id/index.m3u8", auth.StreamMiddleware(), func(c *gin.Context) {
		c.String(http.StatusOK, "#EXTM3U")
	})

	tokCam1, err := auth.GenerateStreamToken("cam-1")
	require.NoError(t, err)

	t.Run("valid stream token matches camera id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/stream/hls/cam-1/index.m3u8?token="+tokCam1, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "#EXTM3U", w.Body.String())
	})

	t.Run("token camera id mismatch", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/stream/hls/cam-2/index.m3u8?token="+tokCam1, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("missing stream token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/stream/hls/cam-1/index.m3u8", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("main api token rejected on stream endpoint", func(t *testing.T) {
		apiTok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"username": "admin",
			"role":     "admin",
			"exp":      time.Now().Add(time.Hour).Unix(),
		})
		apiTokStr, _ := apiTok.SignedString([]byte(auth.cfg.Auth.Secret))

		req := httptest.NewRequest(http.MethodGet, "/stream/hls/cam-1/index.m3u8?token="+apiTokStr, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("stream token claims contains iat and nbf", func(t *testing.T) {
		token, err := jwt.Parse(tokCam1, func(_ *jwt.Token) (interface{}, error) {
			return []byte(auth.cfg.Auth.Secret), nil
		})
		require.NoError(t, err)
		claims, ok := token.Claims.(jwt.MapClaims)
		require.True(t, ok)
		assert.Equal(t, "cam-1", claims["stream_id"])
		assert.NotNil(t, claims["iat"])
		assert.NotNil(t, claims["nbf"])
		assert.NotNil(t, claims["exp"])
	})
}

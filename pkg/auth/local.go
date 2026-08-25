// Package auth provides authentication, token issuance, and role-based access
// control (RBAC) middleware for RUSEON Core.
//
// It implements JWT-based local authentication with bcrypt password verification,
// short-lived stream token generation for secure media playback, and continuous
// state verification against the underlying StateStore.
package auth

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"

	"github.com/RUSEGAL/ruseon-core/v2/internal/models"
	"github.com/RUSEGAL/ruseon-core/v2/pkg/config"
	"github.com/RUSEGAL/ruseon-core/v2/pkg/registry"
)

// LocalAuthenticator implements registry.Authenticator using local bcrypt-hashed
// credentials stored in StateStore and HMAC-SHA256 signed JSON Web Tokens (JWT).
//
// All methods are thread-safe and designed for concurrent invocation by HTTP handlers.
type LocalAuthenticator struct {
	cfg *config.Config
}

func generateRandomPassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		panic("failed to generate random bytes: " + err.Error())
	}
	for i := 0; i < length; i++ {
		b[i] = charset[b[i]%byte(len(charset))]
	}
	return string(b)
}

// NewLocalAuthenticator creates a new LocalAuthenticator instance using the provided configuration.
//
// On the very first startup when no users exist in the StateStore, it automatically generates
// a secure, random 16-character initial administrator password, saves the admin user to the database,
// and prints the one-time credentials to stdout for initial system setup.
func NewLocalAuthenticator(cfg *config.Config) *LocalAuthenticator {
	auth := &LocalAuthenticator{
		cfg: cfg,
	}

	if registry.CurrentStateStore != nil {
		hasUsers, err := registry.CurrentStateStore.HasUsers()
		if err == nil && !hasUsers {
			password := generateRandomPassword(16)
			hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err == nil {
				user := &models.User{
					Username:     "admin",
					PasswordHash: string(hash),
					Role:         models.RoleAdmin,
				}
				err = registry.CurrentStateStore.SaveUser(user)
				if err == nil {
					fmt.Println("\n=======================================================")
					fmt.Printf("[SECURITY] INITIAL ADMIN PASSWORD: %s\n", password)
					fmt.Println("[SECURITY] Username: admin")
					fmt.Println("[SECURITY] Please save this password. It will not be shown again.")
					fmt.Println("=======================================================")
					log.Info().Msg("Generated initial admin password and saved to BadgerDB")
					log.Info().Str("audit", "true").Str("action", "user_created").Str("username", "admin").Msg("Initial admin user created")
				} else {
					log.Error().Err(err).Msg("Failed to save initial admin password to BadgerDB")
				}
			} else {
				log.Error().Err(err).Msg("Failed to hash initial admin password")
			}
		}
	}

	return auth
}

// Request represents the incoming JSON login payload containing user credentials.
type Request struct {
	// Username is the unique account identifier.
	Username string `json:"username"`
	// Password is the plaintext password to be verified against the stored bcrypt hash.
	Password string `json:"password"`
}

// Login is a Gin handler that authenticates user credentials and issues a signed JWT access token.
//
// On successful authentication, it returns an HTTP 200 OK response with a token valid for 1 hour.
// If credentials are invalid, it responds with HTTP 401 Unauthorized and writes an audit log event.
func (a *LocalAuthenticator) Login(c *gin.Context) {
	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	if req.Username == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	user, err := registry.CurrentStateStore.GetUser(req.Username)
	if err != nil || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		log.Warn().Str("audit", "true").Str("action", "login_failed").Str("username", req.Username).Msg("Failed login attempt")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	log.Info().Str("audit", "true").Str("action", "login_success").Str("username", req.Username).Str("role", string(user.Role)).Msg("User logged in")

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": user.Username,
		"role":     user.Role,
		"exp":      time.Now().Add(time.Hour * 1).Unix(), // 1 hour token
	})

	tokenString, err := token.SignedString([]byte(a.cfg.Auth.Secret))
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate JWT token")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": tokenString})
}

// Middleware returns a Gin middleware handler that validates the "Authorization: Bearer <token>" header.
//
// The middleware enforces:
//   - Valid HMAC-SHA256 signature and expiration time.
//   - Rejection of short-lived stream tokens (which cannot be used for general REST API access).
//   - Real-time revocation/role verification against StateStore to ensure deleted users or demoted roles cannot act.
//   - Injects "username" and "role" values into the Gin context for downstream handlers.
func (a *LocalAuthenticator) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(a.cfg.Auth.Secret), nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		// Ensure this is not a short-lived stream token
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			if _, isStreamToken := claims["stream_id"]; isStreamToken {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Stream tokens are not valid for API access"})
				return
			}
			
			username, _ := claims["username"].(string)
			tokenRole, _ := claims["role"].(string)

			// Revocation check: verify user still exists and role hasn't changed
			if registry.CurrentStateStore != nil {
				user, err := registry.CurrentStateStore.GetUser(username)
				if err != nil || user == nil {
					c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "User no longer exists"})
					return
				}
				if string(user.Role) != tokenRole {
					c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "User role has changed, please login again"})
					return
				}
			}

			c.Set("username", username)
			c.Set("role", tokenRole)
		}

		c.Next()
	}
}

// RequireRole creates a Gin middleware that ensures the authenticated user possesses
// at least one of the specified allowed roles.
//
// If the user's role does not match, the request is aborted with HTTP 403 Forbidden
// and an audit log entry is recorded.
func RequireRole(allowedRoles ...models.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("role")
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Role not found"})
			return
		}

		roleStr := fmt.Sprintf("%v", userRole)
		for _, r := range allowedRoles {
			if roleStr == string(r) {
				c.Next()
				return
			}
		}

		log.Warn().Str("audit", "true").Str("action", "access_denied").Str("path", c.Request.URL.Path).Str("role", roleStr).Msg("Access denied due to insufficient permissions")
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
	}
}

// GenerateStreamToken creates a short-lived (60 seconds) signed JWT token scoped exclusively
// for accessing media playback endpoints (HLS, WebRTC, WebSocket) of the specified cameraID.
//
// Returns the signed token string or an error if JWT signing fails.
func (a *LocalAuthenticator) GenerateStreamToken(cameraID string) (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"stream_id": cameraID,
		"iat":       now.Unix(),
		"nbf":       now.Unix(),
		"exp":       now.Add(time.Second * 60).Unix(), // 60 seconds TTL
	})
	return token.SignedString([]byte(a.cfg.Auth.Secret))
}

// StreamMiddleware returns a Gin middleware that validates short-lived stream tokens provided
// via the "?token=..." query parameter on video streaming endpoints.
//
// It verifies that:
//   - The token is present, signed, and not expired.
//   - The token contains a "stream_id" claim matching the requested URL camera parameter ":id".
//   - General API access tokens are rejected to prevent token leakage from media players.
func (a *LocalAuthenticator) StreamMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := c.Query("token")
		if tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing stream token in query"})
			return
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(a.cfg.Auth.Secret), nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired stream token"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			return
		}

		// Verify this is a stream token
		streamID, hasStreamID := claims["stream_id"].(string)
		if !hasStreamID {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Main API tokens are not allowed here. Please use a short-lived stream token."})
			return
		}

		requestedID := c.Param("id")
		if streamID != requestedID {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Stream token does not match the requested camera"})
			return
		}

		c.Next()
	}
}

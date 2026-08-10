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

	"github.com/RUSEGAL/ruseon-core/pkg/config"
	"github.com/RUSEGAL/ruseon-core/pkg/registry"
)

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
				err = registry.CurrentStateStore.SaveUser("admin", string(hash))
				if err == nil {
					fmt.Println("\n=======================================================")
					fmt.Printf("[SECURITY] INITIAL ADMIN PASSWORD: %s\n", password)
					fmt.Println("[SECURITY] Username: admin")
					fmt.Println("[SECURITY] Please save this password. It will not be shown again.")
					fmt.Println("=======================================================")
					log.Info().Msg("Generated initial admin password and saved to BadgerDB")
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

// Request модель запроса логина
type Request struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

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

	hash, err := registry.CurrentStateStore.GetUser(req.Username)
	if err != nil || hash == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": req.Username,
		"exp":      time.Now().Add(time.Hour * 24 * 7).Unix(), // 7 дней
	})

	tokenString, err := token.SignedString([]byte(a.cfg.Auth.Secret))
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate JWT token")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": tokenString})
}

func (a *LocalAuthenticator) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		tokenString := ""

		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
		} else {
			tokenString = c.Query("token")
		}

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
		}

		c.Next()
	}
}

// GenerateStreamToken создает короткоживущий токен для доступа к потоку камеры
func (a *LocalAuthenticator) GenerateStreamToken(cameraID string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"stream_id": cameraID,
		"exp":       time.Now().Add(time.Second * 60).Unix(), // 60 секунд
	})
	return token.SignedString([]byte(a.cfg.Auth.Secret))
}

// StreamMiddleware проверяет короткоживущие токены для видео-потоков
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

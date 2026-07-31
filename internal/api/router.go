package api

import (
	"net/http"

	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// SetupRouter инициализирует маршруты Gin.
func SetupRouter(h *Handler, debug bool) *gin.Engine {
	if !debug {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())

	// Zerolog middleware for Gin
	r.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)

		// Игнорируем логирование самого лог-стрима, чтобы не было бесконечного цикла
		if c.Request.URL.Path == "/api/logs/stream" {
			return
		}

		status := c.Writer.Status()
		path := c.Request.URL.Path
		
		l := log.Info()
		
		if status < 400 && c.Request.Method == "GET" && (path == "/api/cameras" || path == "/api/stats" || path == "/api/tags" || path == "/health") {
			l = log.Debug()
		} else if status >= 400 && status < 500 {
			l = log.Warn()
		} else if status >= 500 {
			l = log.Error()
		}

		l.Str("method", c.Request.Method).
			Str("path", path).
			Int("status", status).
			Dur("duration", duration).
			Str("ip", c.ClientIP()).
			Msg("HTTP Request")
	})

	// CORS Middleware
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Роуты без авторизации
	r.GET("/health", h.HealthCheck)
	r.POST("/api/login", h.Login)

	// API с авторизацией
	api := r.Group("/api")
	api.Use(func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token != "Bearer demo-token-123" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		c.Next()
	})
	
	api.GET("/cameras", h.GetCameras)
	api.POST("/cameras", h.AddCamera)
	api.PUT("/cameras/:id", h.EditCamera)
	api.DELETE("/cameras/:id", h.DeleteCamera)
	api.GET("/stats", h.GetServerStats)
	api.GET("/logs/stream", h.StreamLogs)
	
	api.GET("/tags", h.GetTags)
	api.POST("/tags", h.AddTag)
	api.PUT("/tags/:id", h.EditTag)
	api.DELETE("/tags/:id", h.DeleteTag)

	// HLS стриминг
	r.GET("/stream/hls/:id/index.m3u8", h.GetHLSPlaylist)
	r.GET("/stream/hls/:id/:segment", h.GetHLSSegment)

	// Статика фронтенда
	r.Static("/assets", "./web/dist/assets")
	r.NoRoute(func(c *gin.Context) {
		c.File("./web/dist/index.html")
	})

	return r
}

package api

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"time"
	"mime"

	"gritprofmediaserver/web"

	"github.com/arl/statsviz"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog/log"
)

func init() {
	// Fix Windows missing MIME types for statsviz and frontend
	_ = mime.AddExtensionType(".woff", "font/woff")
	_ = mime.AddExtensionType(".woff2", "font/woff2")
	_ = mime.AddExtensionType(".js", "application/javascript")
	_ = mime.AddExtensionType(".css", "text/css")
}

// SetupRouter инициализирует маршруты Gin.
func SetupRouter(h *Handler, debug bool) *gin.Engine {
	if !debug {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())

	if debug {
		pprof.Register(r)

		srv, err := statsviz.NewServer()
		if err == nil {
			r.GET("/debug/statsviz/*filepath", func(c *gin.Context) {
				if c.Param("filepath") == "/ws" {
					srv.Ws()(c.Writer, c.Request)
					return
				}
				srv.Index()(c.Writer, c.Request)
			})
		} else {
			log.Error().Err(err).Msg("Failed to initialize statsviz")
		}
	}

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
			return []byte(h.cfg.Auth.Secret), nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		c.Next()
	})
	
	api.GET("/cameras", h.GetCameras)
	api.POST("/cameras", h.AddCamera)
	api.PUT("/cameras/:id", h.EditCamera)
	api.DELETE("/cameras/:id", h.DeleteCamera)
	api.GET("/cameras/:id/archive", h.GetCameraArchive)
	api.GET("/cameras/:id/export", h.ExportCameraArchive)
	api.GET("/stats", h.GetServerStats)
	api.GET("/logs/stream", h.StreamLogs)
	
	// Backup
	api.GET("/system/backup/export", h.ExportBackupJSON)
	api.POST("/system/backup/import", h.ImportBackupJSON)
	
	api.GET("/tags", h.GetTags)
	api.POST("/tags", h.AddTag)
	api.PUT("/tags/:id", h.EditTag)
	api.DELETE("/tags/:id", h.DeleteTag)

	// HLS стриминг (Live)
	r.GET("/stream/hls/:id/index.m3u8", h.GetHLSPlaylist)
	r.GET("/stream/hls/:id/:segment", h.GetHLSSegment)

	// HLS стриминг (Archive)
	r.GET("/hls/:id/archive.m3u8", h.GetArchiveHLSPlaylist)
	r.GET("/hls/:id/segment.ts", h.GetArchiveHLSSegment)

	// Статика фронтенда
	assetsFS, err := fs.Sub(web.FrontendFS, "dist/assets")
	if err == nil {
		r.StaticFS("/assets", http.FS(assetsFS))
	}

	distFS, err := fs.Sub(web.FrontendFS, "dist")
	r.NoRoute(func(c *gin.Context) {
		if err != nil {
			c.String(http.StatusNotFound, "Frontend not embedded")
			return
		}
		file, err2 := distFS.Open("index.html")
		if err2 != nil {
			c.String(http.StatusNotFound, "index.html not found")
			return
		}
		defer file.Close()
		stat, _ := file.Stat()
		http.ServeContent(c.Writer, c.Request, "index.html", stat.ModTime(), file.(io.ReadSeeker))
	})

	return r
}

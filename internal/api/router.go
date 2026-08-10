package api

import (
	"io"
	"io/fs"
	"net/http"
	"time"
	"mime"

	"github.com/RUSEGAL/ruseon-core/web"

	"github.com/arl/statsviz"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/RUSEGAL/ruseon-core/pkg/registry"
)

func init() {
	// Fix Windows missing MIME types for statsviz and frontend
	_ = mime.AddExtensionType(".woff", "font/woff")
	_ = mime.AddExtensionType(".woff2", "font/woff2")
	_ = mime.AddExtensionType(".js", "application/javascript")
	_ = mime.AddExtensionType(".css", "text/css")
}

// SetupRouter инициализирует маршруты Gin.
func SetupRouter(h *Handler, auth registry.Authenticator, debug bool) *gin.Engine {
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
		
		switch {
		case status < 400 && c.Request.Method == "GET" && (path == "/api/cameras" || path == "/api/stats" || path == "/api/tags" || path == "/health"):
			l = log.Debug()
		case status >= 400 && status < 500:
			l = log.Warn()
		case status >= 500:
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
	r.POST("/api/login", auth.Login)

	// Метрики Prometheus
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API с авторизацией
	api := r.Group("/api")
	api.Use(auth.Middleware())
	
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

	api.GET("/folders", h.GetFolders)
	api.POST("/folders", h.AddFolder)
	api.PUT("/folders/:id", h.EditFolder)
	api.DELETE("/folders/:id", h.DeleteFolder)

	// Опциональная авторизация для видео-потоков
	authMiddleware := auth.Middleware()
	streamAuth := func(c *gin.Context) {
		id := c.Param("id")
		cam, err := registry.CurrentStateStore.GetCamera(id)
		if err == nil && cam != nil && cam.TokenAuth {
			authMiddleware(c)
		} else {
			c.Next()
		}
	}

	// HLS стриминг (Live)
	r.GET("/stream/hls/:id/index.m3u8", streamAuth, h.GetHLSPlaylist)
	r.GET("/stream/hls/:id/stream.m3u8", streamAuth, h.GetHLSVideoPlaylist)
	r.GET("/stream/hls/:id/subs.m3u8", streamAuth, h.GetHLSSubsPlaylist)
	r.GET("/stream/hls/:id/:segment", streamAuth, h.GetHLSSegment)

	// WebRTC (WHEP)
	r.POST("/stream/webrtc/whep/:id", streamAuth, h.PostWHEP)

	// HLS стриминг (Archive)
	r.GET("/hls/:id/archive.m3u8", streamAuth, h.GetArchiveHLSPlaylist)
	r.GET("/hls/:id/segment.ts", streamAuth, h.GetArchiveHLSSegment)

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

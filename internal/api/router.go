package api

import (
	"io"
	"io/fs"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/RUSEGAL/ruseon-core/v2/web"

	"github.com/arl/statsviz"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	// init swagger docs
	_ "github.com/RUSEGAL/ruseon-core/v2/internal/api/docs"

	"github.com/RUSEGAL/ruseon-core/v2/internal/models"
	pkgauth "github.com/RUSEGAL/ruseon-core/v2/pkg/auth"
	"github.com/RUSEGAL/ruseon-core/v2/pkg/registry"
)

func init() {
	// Fix Windows missing MIME types for statsviz and frontend
	_ = mime.AddExtensionType(".woff", "font/woff")
	_ = mime.AddExtensionType(".woff2", "font/woff2")
	_ = mime.AddExtensionType(".js", "application/javascript")
	_ = mime.AddExtensionType(".css", "text/css")
	_ = mime.AddExtensionType(".svg", "image/svg+xml")
	_ = mime.AddExtensionType(".ico", "image/x-icon")
	_ = mime.AddExtensionType(".wasm", "application/wasm")
}

// SetupRouter initializes and configures the Gin HTTP engine with CORS middleware, zerolog access logging,
// Prometheus metrics, Swagger UI endpoints, Pprof/Statsviz debugging tools, REST API routes, and embedded SPA static file serving.
func SetupRouter(h *Handler, auth registry.Authenticator, debug bool, corsOrigins []string) *gin.Engine {
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
		path := c.Request.URL.Path

		// Fast-bypass для медиапотоков (HLS/WebRTC/Logs): исключаем миллионы аллокаций логгера в секунду
		if strings.HasPrefix(path, "/stream/") || path == "/api/logs/stream" {
			c.Next()
			return
		}

		start := time.Now()
		c.Next()
		duration := time.Since(start)

		status := c.Writer.Status()
		
		l := log.Info()
		
		switch {
		case status < 400 && c.Request.Method == "GET" && (path == "/api/cameras" || path == "/api/stats" || path == "/api/tags" || path == "/livez" || path == "/readyz"):
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
		if len(corsOrigins) == 0 {
			// CORS disabled
			c.Next()
			return
		}

		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			allowed := false
			for _, o := range corsOrigins {
				if o == origin || o == "*" {
					allowed = true
					break
				}
			}

			if allowed {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
				c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
				c.Writer.Header().Add("Vary", "Origin")
			}
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Роуты без авторизации
	r.GET("/livez", h.LivenessCheck)
	r.GET("/readyz", h.ReadinessCheck)
	r.POST("/api/login", auth.Login)
	r.GET("/models/:filename", h.GetModel)

	// Метрики Prometheus
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Swagger Docs
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// API с авторизацией
	api := r.Group("/api")
	api.Use(auth.Middleware())
	
	// Admin and Operator roles can view/manage streams and stats. Viewer can only view streams. Service is for automation.
	apiRead := api.Group("", pkgauth.RequireRole(models.RoleAdmin, models.RoleOperator, models.RoleViewer, models.RoleService))
	apiRead.GET("/cameras", h.GetCameras)
	apiRead.GET("/cameras/:id/archive", h.GetCameraArchive)
	apiRead.GET("/cameras/:id/export", h.ExportCameraArchive)
	apiRead.GET("/cameras/:id/stream-token", h.GetStreamToken)
	apiRead.GET("/stats", h.GetServerStats)
	apiRead.GET("/tags", h.GetTags)
	apiRead.GET("/folders", h.GetFolders)

	// Admin and Operator can edit cameras
	apiWrite := api.Group("", pkgauth.RequireRole(models.RoleAdmin, models.RoleOperator))
	apiWrite.POST("/cameras", h.AddCamera)
	apiWrite.PUT("/cameras/:id", h.EditCamera)
	apiWrite.DELETE("/cameras/:id", h.DeleteCamera)

	// Admin only for system level operations
	apiAdmin := api.Group("", pkgauth.RequireRole(models.RoleAdmin))
	apiAdmin.GET("/logs/stream", h.StreamLogs)
	apiAdmin.GET("/system/backup/export", h.ExportBackupJSON)
	apiAdmin.POST("/system/backup/import", h.ImportBackupJSON)
	apiAdmin.POST("/tags", h.AddTag)
	apiAdmin.PUT("/tags/:id", h.EditTag)
	apiAdmin.DELETE("/tags/:id", h.DeleteTag)
	apiAdmin.POST("/folders", h.AddFolder)
	apiAdmin.PUT("/folders/:id", h.EditFolder)
	apiAdmin.DELETE("/folders/:id", h.DeleteFolder)
	apiAdmin.GET("/users", h.GetUsers)
	apiAdmin.POST("/users", h.AddUser)
	apiAdmin.PUT("/users/:username", h.EditUser)
	apiAdmin.DELETE("/users/:username", h.DeleteUser)

	// Опциональная авторизация для видео-потоков
	streamMiddleware := auth.StreamMiddleware()
	streamAuth := func(c *gin.Context) {
		id := c.Param("id")
		if st, ok := h.manager.GetStream(id); ok && st.IsTokenAuth() {
			streamMiddleware(c)
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

	// WebCodecs (WebSocket Binary NALU Stream)
	r.GET("/stream/ws/:id", streamAuth, h.StreamWS)

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

		// Try to serve static root file from distFS (e.g. /favicon.svg, /favicon.ico, /icons.svg)
		reqPath := strings.TrimPrefix(c.Request.URL.Path, "/")
		if reqPath != "" && !strings.Contains(reqPath, "..") {
			if f, errOpen := distFS.Open(reqPath); errOpen == nil {
				defer f.Close()
				if stat, errStat := f.Stat(); errStat == nil && !stat.IsDir() {
					if rs, ok := f.(io.ReadSeeker); ok {
						http.ServeContent(c.Writer, c.Request, stat.Name(), stat.ModTime(), rs)
						return
					}
				}
			}
		}

		// Fallback to index.html for client-side SPA routing
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

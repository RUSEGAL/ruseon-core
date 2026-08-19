package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/sync/singleflight"

	"github.com/RUSEGAL/ruseon-core/internal/archive"
	"github.com/RUSEGAL/ruseon-core/internal/models"
	"github.com/RUSEGAL/ruseon-core/internal/stream"
	"github.com/RUSEGAL/ruseon-core/internal/webrtc"
	"github.com/RUSEGAL/ruseon-core/pkg/config"
	"github.com/RUSEGAL/ruseon-core/pkg/logger"
	"github.com/RUSEGAL/ruseon-core/pkg/metrics"
	"github.com/RUSEGAL/ruseon-core/pkg/registry"
)

type ClientInfo struct {
	IP       string `json:"ip"`
	StreamID string `json:"streamId"`
}

type ClientTracker struct {
	mu      sync.Mutex
	clients map[string]map[string]time.Time
}

func (c *ClientTracker) Mark(ip, streamID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.clients[ip] == nil {
		c.clients[ip] = make(map[string]time.Time)
	}
	c.clients[ip][streamID] = time.Now()
}

func (c *ClientTracker) GetActiveClients(timeout time.Duration) []ClientInfo {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	var active []ClientInfo
	for ip, streams := range c.clients {
		for streamID, lastSeen := range streams {
			if now.Sub(lastSeen) <= timeout {
				active = append(active, ClientInfo{IP: ip, StreamID: streamID})
			} else {
				delete(streams, streamID)
			}
		}
		if len(streams) == 0 {
			delete(c.clients, ip)
		}
	}
	return active
}

type cachedCamerasResponse struct {
	data      []byte
	timestamp time.Time
}

// Handler хранит зависимости для API.
type Handler struct {
	manager      *stream.Manager
	cfg          *config.Config
	store        registry.StateStore
	startTime    time.Time
	tracker      *ClientTracker
	camerasCache atomic.Pointer[cachedCamerasResponse]
	camerasSf    singleflight.Group
	cacheTTL     time.Duration
	webrtcEngine *webrtc.Engine
}

// NewHandler создает новый обработчик API.
func NewHandler(manager *stream.Manager, cfg *config.Config, store registry.StateStore, webrtcEngine ...*webrtc.Engine) *Handler {
	var engine *webrtc.Engine
	if len(webrtcEngine) > 0 && webrtcEngine[0] != nil {
		engine = webrtcEngine[0]
	} else if cfg != nil {
		engine, _ = webrtc.NewEngine(cfg)
	}

	return &Handler{
		manager:   manager,
		cfg:       cfg,
		store:     store,
		startTime: time.Now(),
		tracker: &ClientTracker{
			clients: make(map[string]map[string]time.Time),
		},
		cacheTTL:     250 * time.Millisecond,
		webrtcEngine: engine,
	}
}

// InvalidateCamerasCache сбрасывает кэш списка камер.
func (h *Handler) InvalidateCamerasCache() {
	h.camerasCache.Store(nil)
}

// LivenessCheck responds to liveness probes (e.g. /livez).
func (h *Handler) LivenessCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

// ReadinessCheck responds to readiness probes (e.g. /readyz) verifying the health of database, storage, and streaming subsystems.
func (h *Handler) ReadinessCheck(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	components := make(map[string]string)
	isReady := true
	var errMsgs []string

	// 1. Check StateStore / Database health via Ping
	store := h.store
	if store == nil {
		store = registry.CurrentStateStore
	}
	if store == nil {
		components["database"] = "uninitialized"
		isReady = false
		errMsgs = append(errMsgs, "database store not initialized")
	} else if err := store.Ping(ctx); err != nil {
		components["database"] = fmt.Sprintf("unavailable: %v", err)
		isReady = false
		errMsgs = append(errMsgs, fmt.Sprintf("database unavailable: %v", err))
	} else {
		components["database"] = "ok"
	}

	// 2. Check BlobStore / Storage subsystem health
	if registry.CurrentBlobStore == nil {
		components["storage"] = "uninitialized"
		isReady = false
		errMsgs = append(errMsgs, "storage blobstore not initialized")
	} else if _, err := registry.CurrentBlobStore.Stat("."); err != nil {
		// If "." is not supported or returns error, check if recordings dir is accessible/creatable
		if mkdirErr := registry.CurrentBlobStore.MkdirAll("recordings"); mkdirErr != nil {
			components["storage"] = fmt.Sprintf("unavailable: %v", err)
			isReady = false
			errMsgs = append(errMsgs, fmt.Sprintf("storage unavailable: %v", err))
		} else {
			components["storage"] = "ok"
		}
	} else {
		components["storage"] = "ok"
	}

	// 3. Check StreamManager / Media subsystem health
	if h.manager == nil {
		components["stream_manager"] = "uninitialized"
		isReady = false
		errMsgs = append(errMsgs, "stream manager not initialized")
	} else if err := h.manager.Ready(ctx); err != nil {
		components["stream_manager"] = fmt.Sprintf("unavailable: %v", err)
		isReady = false
		errMsgs = append(errMsgs, fmt.Sprintf("stream manager unavailable: %v", err))
	} else {
		components["stream_manager"] = "ok"
	}

	if !isReady {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":     "not_ready",
			"error":      strings.Join(errMsgs, "; "),
			"components": components,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "ready",
		"components": components,
	})
}

// StreamLogs streams server logs to clients via SSE.
func (h *Handler) StreamLogs(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	ch := logger.GlobalBroadcaster.Subscribe()
	defer logger.GlobalBroadcaster.Unsubscribe(ch)

	c.Writer.Flush()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case msg := <-ch:
			// zerolog appends a newline by default, strip it so SSE format is correct
			if len(msg) > 0 && msg[len(msg)-1] == '\n' {
				msg = msg[:len(msg)-1]
			}
			_, _ = c.Writer.Write([]byte("data: "))
			_, _ = c.Writer.Write(msg)
			_, _ = c.Writer.Write([]byte("\n\n"))
			c.Writer.Flush()
		}
	}
}

// CameraInfo описывает текущее состояние камеры и статистику для API.
type CameraInfo struct {
	ID             string                 `json:"id"`
	URL            string                 `json:"url"`
	State          models.CameraState     `json:"state"`
	Record         bool                   `json:"record"`
	RetentionDays  int                    `json:"retentionDays"`
	Tags           []string               `json:"tags"`
	FolderID       string                 `json:"folderId"`
	Comment        string                 `json:"comment"`
	SimPhone       string                 `json:"simPhone"`
	SimICCID       string                 `json:"simICCID"`
	TrafficLimit   uint64                 `json:"trafficLimit"`
	TrafficUsed    uint64                 `json:"trafficUsed"`
	Uptime         uint64                 `json:"uptime"`
	BytesReceived  uint64                 `json:"bytesReceived"`
	BytesSent      uint64                 `json:"bytesSent"`
	Frames         uint64                 `json:"frames"`
	KeyFrames      uint64                 `json:"keyFrames"`
	Codec          string                 `json:"codec"`
	LastFrameTime  int64                  `json:"lastFrameTime"`
	LastKeyTime    int64                  `json:"lastKeyTime"`
	LastError      string                 `json:"lastError"`
	Reconnects     uint64                 `json:"reconnects"`
	Bitrate        float64                `json:"bitrate"`
	LazyHLS        bool                   `json:"lazyHLS"`
	TokenAuth      bool                   `json:"tokenAuth"`
	Disabled       bool                   `json:"disabled"`
	DisableReason  string                 `json:"disableReason"`
	DisableHistory []config.DisableRecord `json:"disableHistory"`
	RecordHistory  []config.DisableRecord `json:"recordHistory"`
}

// @Summary Get all cameras
// @Description Returns a list of all registered cameras along with their real-time statistics
// @Tags cameras
// @Produce json
// @Success 200 {array} CameraInfo
// @Router /api/cameras [get]
func (h *Handler) GetCameras(c *gin.Context) {
	// 1. Fast path: check TTL cache
	if cached := h.camerasCache.Load(); cached != nil {
		if time.Since(cached.timestamp) < h.cacheTTL {
			c.Data(http.StatusOK, "application/json; charset=utf-8", cached.data)
			return
		}
	}

	// 2. Slow path: singleflight to avoid stampede
	val, err, _ := h.camerasSf.Do("get_cameras", func() (interface{}, error) {
		// Double check after acquiring singleflight lock
		if cached := h.camerasCache.Load(); cached != nil {
			if time.Since(cached.timestamp) < h.cacheTTL {
				return cached.data, nil
			}
		}

		streams := h.manager.GetStreams()
		streamMap := make(map[string]*stream.Stream, len(streams))
		for _, st := range streams {
			streamMap[st.ID] = st
		}

		result := make([]CameraInfo, 0)
		cams, err := h.store.ListCameras()
		if err != nil {
			cams = []config.CameraConfig{}
		}

		for _, cam := range cams {
			stats := streamMap[cam.ID]

			state := models.StateOffline
			var uptime, bytesReceived, bytesSent, frames, keyFrames, reconnects uint64
			var lastFrameTime, lastKeyTime int64
			var bitrate float64
			var lastError string
			codec := "-"
			if stats != nil {
				s := stats.GetStats()
				state = s.State
				// #nosec G115 -- uptime in seconds is non-negative
				uptime = uint64(s.Uptime)
				bytesReceived = s.BytesReceived
				bytesSent = s.BytesSent
				frames = s.Frames
				keyFrames = s.KeyFrames
				codec = s.Codec
				lastFrameTime = s.LastFrameTime
				lastKeyTime = s.LastKeyTime
				lastError = s.LastError
				reconnects = s.Reconnects
				bitrate = s.Bitrate
			}

			result = append(result, CameraInfo{
				ID:             cam.ID,
				URL:            cam.URL,
				State:          state,
				Record:         cam.Record,
				RetentionDays:  cam.RetentionDays,
				Tags:           cam.Tags,
				FolderID:       cam.FolderID,
				Comment:        cam.Comment,
				SimPhone:       cam.SimPhone,
				SimICCID:       cam.SimICCID,
				TrafficLimit:   cam.TrafficLimit,
				TrafficUsed:    cam.TrafficUsed,
				Disabled:       cam.Disabled,
				DisableReason:  cam.DisableReason,
				DisableHistory: cam.DisableHistory,
				RecordHistory:  cam.RecordHistory,
				Uptime:         uptime,
				BytesReceived:  bytesReceived,
				BytesSent:      bytesSent,
				Frames:         frames,
				KeyFrames:      keyFrames,
				Codec:          codec,
				LastFrameTime:  lastFrameTime,
				LastKeyTime:    lastKeyTime,
				LastError:      lastError,
				Reconnects:     reconnects,
				Bitrate:        bitrate,
				LazyHLS:        cam.LazyHLS,
				TokenAuth:      cam.TokenAuth,
			})
		}

		data, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}

		h.camerasCache.Store(&cachedCamerasResponse{
			data:      data,
			timestamp: time.Now(),
		})

		return data, nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to serialize cameras: " + err.Error()})
		return
	}

	c.Data(http.StatusOK, "application/json; charset=utf-8", val.([]byte))
}

// @Summary Add a new camera
// @Description Dynamically registers a new camera and starts its stream if not disabled
// @Tags cameras
// @Accept json
// @Produce json
// @Param camera body config.CameraConfig true "Camera Configuration"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/cameras [post]
func (h *Handler) AddCamera(c *gin.Context) {
	var cam config.CameraConfig
	if err := c.ShouldBindJSON(&cam); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	cam.ID = strings.TrimSpace(cam.ID)
	if cam.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Camera ID cannot be empty"})
		return
	}

	// Проверяем, не существует ли уже камера с таким ID в хранилище
	if existing, _ := h.store.GetCamera(cam.ID); existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("Camera with ID '%s' already exists", cam.ID)})
		return
	}

	if err := h.store.SaveCamera(&cam); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save camera to DB"})
		return
	}

	// Атомарно синхронизируем рантайм-менеджер потоков
	h.manager.UpsertStream(cam.ID, cam.URL, cam.Record, cam.LazyHLS, cam.Transport, cam.Disabled)

	log.Info().Str("audit", "true").Str("action", "camera_added").Str("camera_id", cam.ID).Str("user", c.GetString("username")).Msg("Camera added")
	h.InvalidateCamerasCache()
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// @Summary Delete a camera
// @Description Stops the stream and completely removes the camera configuration
// @Tags cameras
// @Produce json
// @Param id path string true "Camera ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/cameras/{id} [delete]
func (h *Handler) DeleteCamera(c *gin.Context) {
	id := c.Param("id")

	// Проверяем, существует ли камера в хранилище
	if _, err := h.store.GetCamera(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Camera '%s' not found", id)})
		return
	}

	if err := h.store.DeleteCamera(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete camera"})
		return
	}

	// Останавливаем поток и удаляем из менеджера
	h.manager.RemoveStream(id)

	log.Info().Str("audit", "true").Str("action", "camera_deleted").Str("camera_id", id).Str("user", c.GetString("username")).Msg("Camera deleted")
	h.InvalidateCamerasCache()
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// @Summary Edit a camera
// @Description Updates camera settings dynamically without full restart
// @Tags cameras
// @Accept json
// @Produce json
// @Param id path string true "Camera ID"
// @Param camera body config.CameraConfig true "Updated Camera Configuration"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/cameras/{id} [put]
func (h *Handler) EditCamera(c *gin.Context) {
	id := c.Param("id")
	var req config.CameraConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	// Гарантируем соответствие ID в URL и теле запроса
	req.ID = id

	err := h.store.UpdateCameraTx(id, func(cam *config.CameraConfig) bool {
		cam.URL = req.URL
		cam.RetentionDays = req.RetentionDays
		cam.Tags = req.Tags
		cam.FolderID = req.FolderID
		cam.Comment = req.Comment
		cam.SimPhone = req.SimPhone
		cam.SimICCID = req.SimICCID
		cam.LazyHLS = req.LazyHLS
		cam.TokenAuth = req.TokenAuth
		cam.Transport = req.Transport

		// Отслеживание изменения статуса записи
		if cam.Record != req.Record {
			action := "enable"
			if !req.Record {
				action = "disable"
			}
			recordEvent := config.DisableRecord{
				Timestamp: time.Now().Format(time.RFC3339),
				Action:    action,
			}
			cam.RecordHistory = append(cam.RecordHistory, recordEvent)
		}

		// Отслеживание изменения статуса отключения камеры
		if cam.Disabled != req.Disabled {
			action := "enable"
			if req.Disabled {
				action = "disable"
			}
			record := config.DisableRecord{
				Timestamp: time.Now().Format(time.RFC3339),
				Action:    action,
				Reason:    req.DisableReason,
			}
			cam.DisableHistory = append(cam.DisableHistory, record)
		}

		cam.Disabled = req.Disabled
		cam.DisableReason = req.DisableReason
		cam.Record = req.Record
		return true
	})

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Camera '%s' not found or failed to update", id)})
		return
	}

	// Атомарно обновляем рантайм-поток
	h.manager.UpsertStream(id, req.URL, req.Record, req.LazyHLS, req.Transport, req.Disabled)

	log.Info().Str("audit", "true").Str("action", "camera_edited").Str("camera_id", id).Str("user", c.GetString("username")).Msg("Camera edited")
	h.InvalidateCamerasCache()
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// GetHLSPlaylist возвращает M3U8 плейлист для конкретной камеры.
func (h *Handler) GetHLSPlaylist(c *gin.Context) {
	id := c.Param("id")
	metrics.HLSRequestsTotal.WithLabelValues(id).Inc()
	h.tracker.Mark(c.ClientIP(), id)
	st, ok := h.manager.GetStream(id)
	if !ok {
		c.String(http.StatusNotFound, "Stream not found")
		return
	}

	st.WakeUpHLSMuxer()

	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	masterPlaylist := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID="subs",NAME="Metadata",DEFAULT=YES,AUTOSELECT=YES,URI="subs.m3u8"
#EXT-X-STREAM-INF:BANDWIDTH=5000000,SUBTITLES="subs"
stream.m3u8
`
	c.String(http.StatusOK, masterPlaylist)

	st.AddBytesSent(uint64(len(masterPlaylist)))
}

// GetHLSVideoPlaylist возвращает плейлист видео сегментов для HLS.
func (h *Handler) GetHLSVideoPlaylist(c *gin.Context) {
	id := c.Param("id")
	metrics.HLSRequestsTotal.WithLabelValues(id).Inc()
	h.tracker.Mark(c.ClientIP(), id)
	st, ok := h.manager.GetStream(id)
	if !ok {
		c.String(http.StatusNotFound, "Stream not found")
		return
	}

	muxer := st.WakeUpHLSMuxer()
	playlist := muxer.GetPlaylist()

	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.String(http.StatusOK, playlist)

	st.AddBytesSent(uint64(len(playlist)))
}

// GetHLSSubsPlaylist возвращает M3U8 плейлист субтитров для конкретной камеры.
func (h *Handler) GetHLSSubsPlaylist(c *gin.Context) {
	id := c.Param("id")
	metrics.HLSRequestsTotal.WithLabelValues(id).Inc()
	h.tracker.Mark(c.ClientIP(), id)
	st, ok := h.manager.GetStream(id)
	if !ok {
		c.String(http.StatusNotFound, "Stream not found")
		return
	}

	muxer := st.WakeUpHLSMuxer()
	playlist := muxer.GetSubsPlaylist()

	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.String(http.StatusOK, playlist)
}

// GetHLSSegment возвращает TS-сегмент или VTT файл для конкретной камеры.
func (h *Handler) GetHLSSegment(c *gin.Context) {
	id := c.Param("id")
	h.tracker.Mark(c.ClientIP(), id)
	metrics.HLSRequestsTotal.WithLabelValues(id).Inc()
	segment := c.Param("segment")

	st, ok := h.manager.GetStream(id)
	if !ok {
		c.String(http.StatusNotFound, "Stream not found")
		return
	}

	muxer := st.WakeUpHLSMuxer()
	seg, mimeType := muxer.AcquireSegment(segment)
	if seg == nil {
		c.String(http.StatusNotFound, "Segment not found")
		return
	}
	defer seg.Release()

	var data []byte
	if mimeType == "text/vtt" {
		data = seg.VTTData
	} else {
		data = seg.Data
	}

	c.Header("Content-Type", mimeType)
	c.Data(http.StatusOK, mimeType, data)

	st.AddBytesSent(uint64(len(data)))
}

// PostWHEP обрабатывает WebRTC SDP Offer и возвращает SDP Answer (WHEP протокол).
func (h *Handler) PostWHEP(c *gin.Context) {
	id := c.Param("id")
	h.tracker.Mark(c.ClientIP(), id)

	st, ok := h.manager.GetStream(id)
	if !ok {
		c.String(http.StatusNotFound, "Stream not found")
		return
	}

	body, err := c.GetRawData()
	if err != nil {
		c.String(http.StatusBadRequest, "Failed to read SDP offer")
		return
	}
	offerSDP := string(body)

	whepHandler := webrtc.NewWHEPHandler(id, st.GetRingBuffer(), st.GetMetadataBroadcaster(), h.cfg, h.webrtcEngine)
	answerSDP, err := whepHandler.HandleOffer(c.Request.Context(), offerSDP)
	if err != nil {
		log.Error().Err(err).Str("stream", id).Msg("WHEP HandleOffer failed")
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	c.Header("Content-Type", "application/sdp")
	c.String(http.StatusCreated, answerSDP)
}

// GetStreamToken возвращает короткоживущий токен для HLS и WebRTC.
func (h *Handler) GetStreamToken(c *gin.Context) {
	id := c.Param("id")
	token, err := registry.CurrentAuthenticator.GenerateStreamToken(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate stream token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"stream_token": token})
}

// GetServerStats
func (h *Handler) GetServerStats(c *gin.Context) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	streams := h.manager.GetStreams()
	var totalBytes uint64
	var totalBytesSent uint64
	var totalFrames uint64
	var onlineCameras int

	for _, st := range streams {
		stats := st.GetStats()
		totalBytes += stats.BytesReceived
		totalBytesSent += stats.BytesSent
		totalFrames += stats.Frames
		if stats.State == models.StateOnline {
			onlineCameras++
		}
	}

	disabledCameras := 0
	disabledReasons := map[string]int{
		"technical": 0,
		"requested": 0,
		"payment":   0,
	}

	cams, _ := h.store.ListCameras()
	for _, cam := range cams {
		if cam.Disabled {
			disabledCameras++
			if cam.DisableReason != "" {
				disabledReasons[cam.DisableReason]++
			}
		}
	}

	activeClients := h.tracker.GetActiveClients(15 * time.Second)

	c.JSON(http.StatusOK, gin.H{
		"uptime":          int(time.Since(h.startTime).Seconds()),
		"memoryUsed":      m.Alloc,
		"sysMemory":       m.Sys,
		"heapAlloc":       m.HeapAlloc,
		"heapSys":         m.HeapSys,
		"heapObjects":     m.HeapObjects,
		"numGC":           m.NumGC,
		"numCPU":          runtime.NumCPU(),
		"goroutines":      runtime.NumGoroutine(),
		"totalCameras":    len(cams),
		"onlineCameras":   onlineCameras,
		"disabledCameras": disabledCameras,
		"disabledReasons": disabledReasons,
		"totalBytes":      totalBytes,
		"totalBytesSent":  totalBytesSent,
		"totalFrames":     totalFrames,
		"activeClients":   len(activeClients),
		"clients":         activeClients,
	})
}

// GetTags возвращает глобальный список тегов
func (h *Handler) GetTags(c *gin.Context) {
	tags, err := h.store.ListTags()
	if err != nil {
		tags = []config.TagConfig{}
	}
	c.JSON(http.StatusOK, tags)
}

// AddTag добавляет новый тег
func (h *Handler) AddTag(c *gin.Context) {
	var tag config.TagConfig
	if err := c.ShouldBindJSON(&tag); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}
	if err := h.store.SaveTag(&tag); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save tag to db"})
		return
	}
	c.JSON(http.StatusOK, tag)
}

// EditTag изменяет тег
func (h *Handler) EditTag(c *gin.Context) {
	id := c.Param("id")
	var req config.TagConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	t, err := h.store.GetTag(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
		return
	}

	t.Name = req.Name
	t.Color = req.Color

	if err := h.store.SaveTag(t); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save tag"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// DeleteTag удаляет тег по ID
func (h *Handler) DeleteTag(c *gin.Context) {
	id := c.Param("id")

	if err := h.store.DeleteTag(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete tag"})
		return
	}

	// Очищаем удаленный тег у всех камер
	cams, _ := h.store.ListCameras()
	for _, camMeta := range cams {
		_ = h.store.UpdateCameraTx(camMeta.ID, func(cam *config.CameraConfig) bool {
			var newCamTags []string
			changed := false
			for _, tID := range cam.Tags {
				if tID != id {
					newCamTags = append(newCamTags, tID)
				} else {
					changed = true
				}
			}
			if changed {
				cam.Tags = newCamTags
				return true
			}
			return false
		})
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// GetFolders возвращает глобальный список папок
func (h *Handler) GetFolders(c *gin.Context) {
	folders, err := h.store.ListFolders()
	if err != nil {
		folders = []config.FolderConfig{}
	}
	c.JSON(http.StatusOK, folders)
}

// AddFolder добавляет новую папку
func (h *Handler) AddFolder(c *gin.Context) {
	var folder config.FolderConfig
	if err := c.ShouldBindJSON(&folder); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}
	if err := h.store.SaveFolder(&folder); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save folder to db"})
		return
	}
	c.JSON(http.StatusOK, folder)
}

// EditFolder изменяет папку
func (h *Handler) EditFolder(c *gin.Context) {
	id := c.Param("id")
	var req config.FolderConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	f, err := h.store.GetFolder(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Folder not found"})
		return
	}

	f.Name = req.Name

	if err := h.store.SaveFolder(f); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save folder"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// DeleteFolder удаляет папку по ID
func (h *Handler) DeleteFolder(c *gin.Context) {
	id := c.Param("id")

	if err := h.store.DeleteFolder(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete folder"})
		return
	}

	// Очищаем удаленную папку у всех камер
	cams, _ := h.store.ListCameras()
	for _, camMeta := range cams {
		if camMeta.FolderID == id {
			_ = h.store.UpdateCameraTx(camMeta.ID, func(cam *config.CameraConfig) bool {
				cam.FolderID = ""
				return true
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// GetCameraArchive возвращает список доступных отрезков архива для камеры
func (h *Handler) GetCameraArchive(c *gin.Context) {
	id := c.Param("id")
	// Проверяем существование камеры
	if _, err := h.store.GetCamera(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Camera not found"})
		return
	}

	intervals, err := archive.GetCameraArchive("recordings", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read archive"})
		return
	}

	log.Info().Str("audit", "true").Str("action", "archive_viewed").Str("camera_id", id).Str("user", c.GetString("username")).Msg("Camera archive viewed")
	c.JSON(http.StatusOK, intervals)
}

// GetArchiveHLSPlaylist отдает M3U8 манифест для конкретного файла архива
func (h *Handler) GetArchiveHLSPlaylist(c *gin.Context) {
	id := c.Param("id")
	metrics.HLSRequestsTotal.WithLabelValues(id).Inc()
	filename := c.Query("file")

	if filename == "" {
		c.String(http.StatusBadRequest, "file parameter is required")
		return
	}

	playlist, err := archive.GenerateHLSPlaylist("recordings", id, filename)
	if err != nil {
		c.String(http.StatusInternalServerError, "failed to generate playlist: %v", err)
		return
	}

	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.Header("Cache-Control", "no-cache")
	log.Info().Str("audit", "true").Str("action", "archive_playlist_viewed").Str("camera_id", id).Str("user", c.GetString("username")).Msg("Camera archive HLS playlist viewed")
	c.String(http.StatusOK, playlist)
}

// CleanupArchiveTrigger вручную вызывает сборщик мусора архива
func (h *Handler) CleanupArchiveTrigger(_ *gin.Context) {
	// (Опционально)
}

// ExportBackupJSON отдает дамп конфигурации (камеры и теги) в виде JSON-файла.
func (h *Handler) ExportBackupJSON(c *gin.Context) {
	data, err := h.store.ExportJSON()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate backup: " + err.Error()})
		return
	}

	filename := fmt.Sprintf("config_backup_%s.json", time.Now().Format("2006-01-02_15-04-05"))
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, "application/json", data)
}

const maxBackupSize = 50 << 20 // 50 MB

// ImportBackupJSON принимает JSON-файл дампа и восстанавливает конфигурации камер и тегов.
func (h *Handler) ImportBackupJSON(c *gin.Context) {
	file, err := c.FormFile("backup")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No backup file provided"})
		return
	}

	if file.Size > maxBackupSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Backup file exceeds maximum allowed size (50MB)"})
		return
	}

	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open uploaded file"})
		return
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, maxBackupSize))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read uploaded file"})
		return
	}

	if err := h.store.ImportJSON(data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to import backup: " + err.Error()})
		return
	}

	// Перезапускаем все потоки с новыми настройками
	if err := h.manager.SyncWithStorage(h.store); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sync streams: " + err.Error()})
		return
	}

	h.InvalidateCamerasCache()
	c.JSON(http.StatusOK, gin.H{"message": "Backup imported and streams restarted successfully."})
}

// GetArchiveHLSSegment отдает TS сегмент архива "на лету"
func (h *Handler) GetArchiveHLSSegment(c *gin.Context) {
	id := c.Param("id")
	metrics.HLSRequestsTotal.WithLabelValues(id).Inc()
	filename := c.Query("file")
	seqStr := c.Query("seq")

	if filename == "" || seqStr == "" {
		c.String(http.StatusBadRequest, "file and seq parameters are required")
		return
	}

	seq, err := strconv.Atoi(seqStr)
	if err != nil {
		c.String(http.StatusBadRequest, "invalid seq")
		return
	}

	segment, err := archive.GenerateHLSSegment("recordings", id, filename, seq)
	if err != nil {
		c.String(http.StatusInternalServerError, "failed to generate segment: %v", err)
		return
	}

	c.Header("Content-Type", "video/MP2T")
	// Кешируем сегмент надолго, так как архив неизменен (кроме ongoing)
	if !strings.HasSuffix(filename, "_ongoing.mp4") {
		c.Header("Cache-Control", "public, max-age=86400")
	}
	c.Data(http.StatusOK, "video/MP2T", segment)
}

// ExportCameraArchive скачивает фрагмент архива в виде MP4 файла
func (h *Handler) ExportCameraArchive(c *gin.Context) {
	id := c.Param("id")
	filename := c.Query("file")
	startSeqStr := c.Query("startSeq")
	endSeqStr := c.Query("endSeq")

	if filename == "" {
		c.String(http.StatusBadRequest, "file parameter is required")
		return
	}

	startSeq := -1
	endSeq := -1

	if startSeqStr != "" {
		s, err := strconv.Atoi(startSeqStr)
		if err == nil {
			startSeq = s
		}
	}
	if endSeqStr != "" {
		e, err := strconv.Atoi(endSeqStr)
		if err == nil {
			endSeq = e
		}
	}

	log.Info().Str("audit", "true").Str("action", "archive_exported").Str("camera_id", id).Str("user", c.GetString("username")).Msg("Camera archive exported")

	c.Writer.Header().Set("Content-Type", "video/mp4")

	downloadFilename := fmt.Sprintf("export_%s", filename)
	if startSeqStr != "" && endSeqStr != "" {
		baseName := strings.TrimSuffix(filename, ".mp4")
		downloadFilename = fmt.Sprintf("export_%s_part_%s_to_%s.mp4", baseName, startSeqStr, endSeqStr)
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, downloadFilename))

	if err := archive.ExportMP4("recordings", id, filename, startSeq, endSeq, c.Writer); err != nil {
		log.Error().Err(err).Msg("Failed to export mp4")
	}
}

// GetUsers возвращает список всех пользователей.
func (h *Handler) GetUsers(c *gin.Context) {
	if registry.CurrentStateStore == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Registry not initialized"})
		return
	}
	users, err := registry.CurrentStateStore.ListUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Очищаем хэши паролей перед отправкой на фронт
	for i := range users {
		users[i].PasswordHash = ""
	}
	c.JSON(http.StatusOK, users)
}

// AddUser добавляет нового пользователя.
func (h *Handler) AddUser(c *gin.Context) {
	var input struct {
		Username string      `json:"username"`
		Password string      `json:"password"`
		Role     models.Role `json:"role"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if input.Username == "" || input.Password == "" || input.Role == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username, password and role are required"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	user := &models.User{
		Username:     input.Username,
		PasswordHash: string(hash),
		Role:         input.Role,
	}

	if err := registry.CurrentStateStore.SaveUser(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Info().Str("audit", "true").Str("action", "user_added").Str("target_username", user.Username).Str("user", c.GetString("username")).Msg("User added")
	c.JSON(http.StatusCreated, user)
}

// EditUser обновляет пользователя (пароль и/или роль).
func (h *Handler) EditUser(c *gin.Context) {
	username := c.Param("username")

	// Запрещаем изменять роль последнего/себя или хотя бы "admin" в простом виде?
	// Пока оставим просто как есть.

	var input struct {
		Password string      `json:"password"`
		Role     models.Role `json:"role"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := registry.CurrentStateStore.GetUser(username)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if input.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}
		user.PasswordHash = string(hash)
	}

	if input.Role != "" {
		user.Role = input.Role
	}

	if err := registry.CurrentStateStore.SaveUser(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Info().Str("audit", "true").Str("action", "user_edited").Str("target_username", user.Username).Str("user", c.GetString("username")).Msg("User edited")
	c.JSON(http.StatusOK, user)
}

// DeleteUser удаляет пользователя.
func (h *Handler) DeleteUser(c *gin.Context) {
	username := c.Param("username")

	if username == "admin" {
		// Опциональная защита, чтобы случайно не удалить админа
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete default admin"})
		return
	}

	if err := registry.CurrentStateStore.DeleteUser(username); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Info().Str("audit", "true").Str("action", "user_deleted").Str("target_username", username).Str("user", c.GetString("username")).Msg("User deleted")
	c.JSON(http.StatusOK, gin.H{"message": "User deleted"})
}

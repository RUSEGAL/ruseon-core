package api

import (
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"gritprofmediaserver/internal/config"
	"gritprofmediaserver/internal/stream"
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

// Handler хранит зависимости для API.
type Handler struct {
	manager   *stream.Manager
	cfg       *config.Config
	startTime time.Time
	tracker   *ClientTracker
}

// NewHandler создает новый обработчик API.
func NewHandler(manager *stream.Manager, cfg *config.Config) *Handler {
	return &Handler{
		manager:   manager,
		cfg:       cfg,
		startTime: time.Now(),
		tracker: &ClientTracker{
			clients: make(map[string]map[string]time.Time),
		},
	}
}

// HealthCheck отвечает на запрос для проверки работоспособности сервиса.
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

// GetCameras возвращает список доступных камер и их статус.
func (h *Handler) GetCameras(c *gin.Context) {
	streams := h.manager.GetStreams()
	
	type CameraInfo struct {
		ID            string   `json:"id"`
		URL           string   `json:"url"`
		Connected     bool     `json:"connected"`
		Record        bool     `json:"record"`
		RetentionDays int      `json:"retentionDays"`
		Tags          []string `json:"tags"`
		Comment       string   `json:"comment"`
		SimPhone      string   `json:"simPhone"`
		SimICCID      string   `json:"simICCID"`
		TrafficLimit  uint64   `json:"trafficLimit"`
		TrafficUsed   uint64   `json:"trafficUsed"`
		Uptime        uint64   `json:"uptime"`
		BytesReceived uint64   `json:"bytesReceived"`
		BytesSent     uint64   `json:"bytesSent"`
		Frames        uint64   `json:"frames"`
		KeyFrames     uint64   `json:"keyFrames"`
		Codec         string   `json:"codec"`
		Disabled      bool                   `json:"disabled"`
		DisableReason string                 `json:"disableReason"`
		DisableHistory []config.DisableRecord `json:"disableHistory"`
	}

	var result []CameraInfo
	for _, cam := range h.cfg.Cameras {
		var stats *stream.Stream
		for _, st := range streams {
			if st.ID == cam.ID {
				stats = st
				break
			}
		}

		connected := false
		var uptime, bytesReceived, bytesSent, frames, keyFrames uint64
		codec := "-"
		if stats != nil {
			s := stats.GetStats()
			connected = s.Connected
			uptime = uint64(s.Uptime)
			bytesReceived = s.BytesReceived
			bytesSent = s.BytesSent
			frames = s.Frames
			keyFrames = s.KeyFrames
			codec = s.Codec
		}

		result = append(result, CameraInfo{
			ID:            cam.ID,
			URL:           cam.URL,
			Connected:     connected,
			Record:        cam.Record,
			RetentionDays: cam.RetentionDays,
			Tags:          cam.Tags,
			Comment:       cam.Comment,
			SimPhone:      cam.SimPhone,
			SimICCID:      cam.SimICCID,
			TrafficLimit:  cam.TrafficLimit,
			TrafficUsed:   cam.TrafficUsed,
			Disabled:      cam.Disabled,
			DisableReason: cam.DisableReason,
			DisableHistory: cam.DisableHistory,
			Uptime:        uptime,
			BytesReceived: bytesReceived,
			BytesSent:     bytesSent,
			Frames:        frames,
			KeyFrames:     keyFrames,
			Codec:         codec,
		})
	}

	c.JSON(http.StatusOK, result)
}

// AuthRequest модель запроса логина
type AuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Login проверяет учетные данные и возвращает токен (для простоты возвращаем фиксированный токен, если совпадает с конфигом)
func (h *Handler) Login(c *gin.Context) {
	var req AuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	if req.Username == h.cfg.Auth.Username && req.Password == h.cfg.Auth.Password && req.Username != "" {
		// Для MVP просто возвращаем фиксированный токен-заглушку. 
		// В идеале здесь должен быть JWT или безопасная сессия.
		c.JSON(http.StatusOK, gin.H{"token": "demo-token-123"})
	} else {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
	}
}

// AddCamera добавляет новую камеру
func (h *Handler) AddCamera(c *gin.Context) {
	var cam config.CameraConfig
	if err := c.ShouldBindJSON(&cam); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	// Обновляем конфиг
	h.cfg.Cameras = append(h.cfg.Cameras, cam)
	if err := h.cfg.Save("config.yaml"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save config"})
		return
	}

	// Запускаем поток, если он не отключен
	if !cam.Disabled {
		if err := h.manager.AddStream(cam.ID, cam.URL, cam.Record); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// DeleteCamera удаляет камеру
func (h *Handler) DeleteCamera(c *gin.Context) {
	id := c.Param("id")

	// Находим и удаляем из конфига
	found := false
	var newCams []config.CameraConfig
	for _, cam := range h.cfg.Cameras {
		if cam.ID == id {
			found = true
		} else {
			newCams = append(newCams, cam)
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Camera not found"})
		return
	}

	h.cfg.Cameras = newCams
	if err := h.cfg.Save("config.yaml"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save config"})
		return
	}

	// Останавливаем поток
	h.manager.RemoveStream(id)

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// EditCamera изменяет параметры существующей камеры
func (h *Handler) EditCamera(c *gin.Context) {
	id := c.Param("id")
	var req config.CameraConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	found := false
	for i, cam := range h.cfg.Cameras {
		if cam.ID == id {
			cam.URL = req.URL
			cam.Record = req.Record
			cam.RetentionDays = req.RetentionDays
			cam.Tags = req.Tags
			cam.Comment = req.Comment
			cam.SimPhone = req.SimPhone
			cam.SimICCID = req.SimICCID

			// Отслеживание изменения статуса
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

			h.cfg.Cameras[i] = cam
			found = true
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Camera not found"})
		return
	}

	if err := h.cfg.Save("config.yaml"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save config"})
		return
	}

	// Перезапускаем поток если он включен
	h.manager.RemoveStream(id)
	if !req.Disabled {
		h.manager.AddStream(req.ID, req.URL, req.Record)
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// GetHLSPlaylist возвращает M3U8 плейлист для конкретной камеры.
func (h *Handler) GetHLSPlaylist(c *gin.Context) {
	id := c.Param("id")
	h.tracker.Mark(c.ClientIP(), id)
	st, ok := h.manager.GetStream(id)
	if !ok {
		c.String(http.StatusNotFound, "Stream not found")
		return
	}

	muxer := st.GetHLSMuxer()
	playlist := muxer.GetPlaylist()

	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.String(http.StatusOK, playlist)

	st.AddBytesSent(uint64(len(playlist)))
}

// GetHLSSegment возвращает TS-сегмент для конкретной камеры.
func (h *Handler) GetHLSSegment(c *gin.Context) {
	id := c.Param("id")
	h.tracker.Mark(c.ClientIP(), id)
	segment := c.Param("segment")

	st, ok := h.manager.GetStream(id)
	if !ok {
		c.String(http.StatusNotFound, "Stream not found")
		return
	}

	muxer := st.GetHLSMuxer()
	data := muxer.GetSegment(segment)
	if data == nil {
		c.String(http.StatusNotFound, "Segment not found")
		return
	}

	c.Header("Content-Type", "video/mp2t")
	c.Data(http.StatusOK, "video/mp2t", data)

	st.AddBytesSent(uint64(len(data)))
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
		if stats.Connected {
			onlineCameras++
		}
	}

	disabledCameras := 0
	disabledReasons := map[string]int{
		"technical": 0,
		"requested": 0,
		"payment":   0,
	}

	for _, cam := range h.cfg.Cameras {
		if cam.Disabled {
			disabledCameras++
			if cam.DisableReason != "" {
				disabledReasons[cam.DisableReason]++
			}
		}
	}

	activeClients := h.tracker.GetActiveClients(15 * time.Second)

	c.JSON(http.StatusOK, gin.H{
		"uptime":         int(time.Since(h.startTime).Seconds()),
		"memoryUsed":     m.Alloc,
		"sysMemory":      m.Sys,
		"heapAlloc":      m.HeapAlloc,
		"heapSys":        m.HeapSys,
		"heapObjects":    m.HeapObjects,
		"numGC":          m.NumGC,
		"numCPU":          runtime.NumCPU(),
		"goroutines":      runtime.NumGoroutine(),
		"totalCameras":    len(h.cfg.Cameras),
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
	if h.cfg.GlobalTags == nil {
		c.JSON(http.StatusOK, []config.TagConfig{})
		return
	}
	c.JSON(http.StatusOK, h.cfg.GlobalTags)
}

// AddTag добавляет новый тег
func (h *Handler) AddTag(c *gin.Context) {
	var tag config.TagConfig
	if err := c.ShouldBindJSON(&tag); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}
	h.cfg.GlobalTags = append(h.cfg.GlobalTags, tag)
	if err := h.cfg.Save("config.yaml"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save config"})
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
	
	found := false
	for i, t := range h.cfg.GlobalTags {
		if t.ID == id {
			h.cfg.GlobalTags[i] = req
			found = true
			break
		}
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
		return
	}
	if err := h.cfg.Save("config.yaml"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save config"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// DeleteTag удаляет тег по ID
func (h *Handler) DeleteTag(c *gin.Context) {
	id := c.Param("id")
	var newTags []config.TagConfig
	for _, t := range h.cfg.GlobalTags {
		if t.ID != id {
			newTags = append(newTags, t)
		}
	}
	h.cfg.GlobalTags = newTags
	
	// Очищаем удаленный тег у всех камер
	for i := range h.cfg.Cameras {
		var newCamTags []string
		for _, tID := range h.cfg.Cameras[i].Tags {
			if tID != id {
				newCamTags = append(newCamTags, tID)
			}
		}
		h.cfg.Cameras[i].Tags = newCamTags
	}
	
	if err := h.cfg.Save("config.yaml"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save config"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}


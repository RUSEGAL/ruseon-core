package api

import (
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog/log"

	"github.com/RUSEGAL/REA-Stream-Engine/internal/archive"
	"github.com/RUSEGAL/REA-Stream-Engine/internal/config"
	"github.com/RUSEGAL/REA-Stream-Engine/internal/logger"
	"github.com/RUSEGAL/REA-Stream-Engine/internal/storage"
	"github.com/RUSEGAL/REA-Stream-Engine/internal/stream"
	"strconv"
	"strings"
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
	store     *storage.Storage
	startTime time.Time
	tracker   *ClientTracker
}

// NewHandler создает новый обработчик API.
func NewHandler(manager *stream.Manager, cfg *config.Config, store *storage.Storage) *Handler {
	return &Handler{
		manager:   manager,
		cfg:       cfg,
		store:     store,
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
		LazyHLS       bool     `json:"lazyHLS"`
		Disabled      bool                   `json:"disabled"`
		DisableReason string                 `json:"disableReason"`
		DisableHistory []config.DisableRecord `json:"disableHistory"`
		RecordHistory  []config.DisableRecord `json:"recordHistory"`
	}

	result := make([]CameraInfo, 0)
	cams, err := h.store.ListCameras()
	if err != nil {
		cams = []config.CameraConfig{}
	}
	for _, cam := range cams {
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
			RecordHistory: cam.RecordHistory,
			Uptime:        uptime,
			BytesReceived: bytesReceived,
			BytesSent:     bytesSent,
			Frames:        frames,
			KeyFrames:     keyFrames,
			Codec:         codec,
			LazyHLS:       cam.LazyHLS,
		})
	}

	c.JSON(http.StatusOK, result)
}

// AuthRequest модель запроса логина
type AuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Login проверяет учетные данные и возвращает JWT токен
func (h *Handler) Login(c *gin.Context) {
	var req AuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	if req.Username == h.cfg.Auth.Username && req.Password == h.cfg.Auth.Password && req.Username != "" {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"username": req.Username,
			"exp":      time.Now().Add(time.Hour * 24 * 7).Unix(), // 7 дней
		})

		tokenString, err := token.SignedString([]byte(h.cfg.Auth.Secret))
		if err != nil {
			log.Error().Err(err).Msg("Failed to generate JWT token")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"token": tokenString})
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

	if err := h.store.SaveCamera(&cam); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save camera to DB"})
		return
	}

	// Запускаем поток, если он не отключен
	if !cam.Disabled {
		if err := h.manager.AddStream(cam.ID, cam.URL, cam.Record, cam.LazyHLS, cam.Transport); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// DeleteCamera удаляет камеру
func (h *Handler) DeleteCamera(c *gin.Context) {
	id := c.Param("id")

	if err := h.store.DeleteCamera(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete camera"})
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

	err := h.store.UpdateCameraTx(id, func(cam *config.CameraConfig) bool {
		cam.URL = req.URL
		cam.RetentionDays = req.RetentionDays
		cam.Tags = req.Tags
		cam.Comment = req.Comment
		cam.SimPhone = req.SimPhone
		cam.SimICCID = req.SimICCID
		cam.LazyHLS = req.LazyHLS

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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update camera"})
		return
	}

	// Перезапускаем поток если он включен
	h.manager.RemoveStream(id)
	if !req.Disabled {
		_ = h.manager.AddStream(req.ID, req.URL, req.Record, req.LazyHLS, req.Transport)
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

	muxer := st.WakeUpHLSMuxer()
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

	muxer := st.WakeUpHLSMuxer()
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
		"uptime":         int(time.Since(h.startTime).Seconds()),
		"memoryUsed":     m.Alloc,
		"sysMemory":      m.Sys,
		"heapAlloc":      m.HeapAlloc,
		"heapSys":        m.HeapSys,
		"heapObjects":    m.HeapObjects,
		"numGC":          m.NumGC,
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

	c.JSON(http.StatusOK, intervals)
}

// GetArchiveHLSPlaylist отдает M3U8 манифест для конкретного файла архива
func (h *Handler) GetArchiveHLSPlaylist(c *gin.Context) {
	id := c.Param("id")
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
	c.String(http.StatusOK, playlist)
}

// CleanupArchiveTrigger вручную вызывает сборщик мусора архива
func (h *Handler) CleanupArchiveTrigger(c *gin.Context) {
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

// ImportBackupJSON принимает JSON-файл дампа и восстанавливает конфигурации камер и тегов.
func (h *Handler) ImportBackupJSON(c *gin.Context) {
	file, err := c.FormFile("backup")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No backup file provided"})
		return
	}
	
	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open uploaded file"})
		return
	}
	defer f.Close()
	
	data := make([]byte, file.Size)
	if _, err := f.Read(data); err != nil {
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
	
	c.JSON(http.StatusOK, gin.H{"message": "Backup imported and streams restarted successfully."})
}

// GetArchiveHLSSegment отдает TS сегмент архива "на лету"
func (h *Handler) GetArchiveHLSSegment(c *gin.Context) {
	id := c.Param("id")
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

	c.Header("Content-Type", "video/mp4")
	
	downloadFilename := fmt.Sprintf("export_%s", filename)
	if startSeqStr != "" && endSeqStr != "" {
		baseName := strings.TrimSuffix(filename, ".mp4")
		downloadFilename = fmt.Sprintf("export_%s_part_%s_to_%s.mp4", baseName, startSeqStr, endSeqStr)
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, downloadFilename))
	
	err := archive.ExportMP4("recordings", id, filename, startSeq, endSeq, c.Writer)
	if err != nil {
		log.Error().Err(err).Msg("Failed to export MP4")
		// Заголовки уже могут быть отправлены, но мы логируем ошибку
	}
}


package api

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

var (
	modelDownloadMu sync.Mutex
	upstreamModels  = map[string][]string{
		"yolo11m.onnx": {
			"https://huggingface.co/Xuban/yolo_weights_database/resolve/main/yolo11m.onnx",
			"https://huggingface.co/banu4prasad/YOLO11m_BDD100k/resolve/main/yolo11m.onnx",
		},
		"yolo11s.onnx": {
			"https://huggingface.co/Xuban/yolo_weights_database/resolve/main/yolo11s.onnx",
		},
		"yolo11n.onnx": {
			"https://github.com/ultralytics/assets/releases/download/v8.3.0/yolo11n.onnx",
			"https://huggingface.co/Xuban/yolo_weights_database/resolve/main/yolo11n.onnx",
		},
	}
)

// GetModel serves or transparently proxies & caches AI ONNX models locally.
// This completely bypasses browser CORS restrictions on external CDNs.
func (h *Handler) GetModel(c *gin.Context) {
	filename := c.Param("filename")
	upstreamURLs, ok := upstreamModels[filename]
	if !ok {
		c.String(http.StatusNotFound, "Model not found")
		return
	}

	modelsDir := filepath.Join("data", "models")
	_ = os.MkdirAll(modelsDir, 0755)
	modelPath := filepath.Join(modelsDir, filename)

	// 1. If model file already exists on disk and valid (> 4MB)
	if stat, err := os.Stat(modelPath); err == nil && stat.Size() > 4*1024*1024 {
		c.Header("Content-Type", "application/octet-stream")
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		c.Header("Access-Control-Allow-Origin", "*")
		c.File(modelPath)
		return
	}

	// 2. Download from upstream on Go backend (Go client is not restricted by browser CORS)
	modelDownloadMu.Lock()
	defer modelDownloadMu.Unlock()

	// Re-check under lock
	if stat, err := os.Stat(modelPath); err == nil && stat.Size() > 4*1024*1024 {
		c.Header("Content-Type", "application/octet-stream")
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		c.Header("Access-Control-Allow-Origin", "*")
		c.File(modelPath)
		return
	}

	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	var resp *http.Response
	var finalURL string

	for _, u := range upstreamURLs {
		log.Info().Str("filename", filename).Str("url", u).Msg("Attempting AI model download from upstream...")
		req, err := http.NewRequestWithContext(c.Request.Context(), "GET", u, nil)
		if err != nil {
			continue
		}

		r, err := client.Do(req)
		if err == nil && r.StatusCode == http.StatusOK {
			resp = r
			finalURL = u
			break
		}
		if r != nil {
			_ = r.Body.Close()
		}
	}

	if resp == nil {
		log.Error().Str("filename", filename).Msg("Failed to download model from all configured upstream CDNs")
		c.String(http.StatusBadGateway, "Failed to download model from upstream CDNs")
		return
	}
	defer resp.Body.Close()

	log.Info().Str("filename", filename).Str("url", finalURL).Msg("Streaming AI model and saving to local disk cache...")

	tmpFile := modelPath + ".tmp"
	f, err := os.Create(tmpFile)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to create local model file: %v", err)
		return
	}

	c.Header("Content-Type", "application/octet-stream")
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Header("Access-Control-Allow-Origin", "*")
	if resp.ContentLength > 0 {
		c.Header("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
	}

	// Stream directly to HTTP response while writing to disk
	tee := io.TeeReader(resp.Body, f)
	_, copyErr := io.Copy(c.Writer, tee)
	_ = f.Close()

	if copyErr == nil {
		_ = os.Rename(tmpFile, modelPath)
		log.Info().Str("filename", filename).Msg("AI model downloaded and cached on disk successfully")
	} else {
		_ = os.Remove(tmpFile)
		log.Warn().Err(copyErr).Msg("Model download interrupted")
	}
}

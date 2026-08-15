package api

import (
	"fmt"
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
	upstreamModels  = map[string]string{
		"yolo11n.onnx": "https://github.com/ultralytics/assets/releases/download/v8.3.0/yolo11n.onnx",
		"yolo11s.onnx": "https://github.com/ultralytics/assets/releases/download/v8.3.0/yolo11s.onnx",
		"yolo11m.onnx": "https://github.com/ultralytics/assets/releases/download/v8.3.0/yolo11m.onnx",
	}
)

// GetModel serves or transparently proxies & caches AI ONNX models locally.
// This completely bypasses browser CORS restrictions on external GitHub Releases CDN.
func (h *Handler) GetModel(c *gin.Context) {
	filename := c.Param("filename")
	upstreamURL, ok := upstreamModels[filename]
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

	log.Info().Str("filename", filename).Str("url", upstreamURL).Msg("Downloading AI model on backend from upstream release...")

	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), "GET", upstreamURL, nil)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to create request: %v", err)
		return
	}

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		log.Error().Err(err).Int("status", status).Msg("Failed to download model from upstream CDN")
		c.String(http.StatusBadGateway, fmt.Sprintf("Failed to fetch model from upstream (status: %d)", status))
		return
	}
	defer resp.Body.Close()

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

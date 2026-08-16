package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"
)

var (
	modelDownloadGroup singleflight.Group
	upstreamModels     = map[string][]string{
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
// Uses singleflight to coalesce concurrent requests per model file without head-of-line blocking.
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

	// 1. Fast path: Serve from local disk cache if already downloaded (> 4MB)
	if stat, err := os.Stat(modelPath); err == nil && stat.Size() > 4*1024*1024 {
		c.Header("Content-Type", "application/octet-stream")
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		c.Header("Access-Control-Allow-Origin", "*")
		c.File(modelPath)
		return
	}

	// 2. Slow path: Coalesce concurrent download requests for this filename
	_, err, _ := modelDownloadGroup.Do(filename, func() (interface{}, error) {
		// Re-check disk under singleflight
		if stat, err := os.Stat(modelPath); err == nil && stat.Size() > 4*1024*1024 {
			return nil, nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		client := &http.Client{Timeout: 5 * time.Minute}
		var resp *http.Response
		var finalURL string

		for _, u := range upstreamURLs {
			log.Info().Str("filename", filename).Str("url", u).Msg("Attempting AI model download from upstream...")
			req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
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
			return nil, fmt.Errorf("failed to download model %s from upstream CDNs", filename)
		}
		defer resp.Body.Close()

		log.Info().Str("filename", filename).Str("url", finalURL).Msg("Saving AI model to local disk cache...")

		tmpFile := modelPath + ".tmp"
		f, err := os.Create(tmpFile)
		if err != nil {
			return nil, fmt.Errorf("failed to create local model file: %w", err)
		}

		// Buffer streaming write to disk (32KB chunks)
		_, copyErr := io.Copy(f, resp.Body)
		_ = f.Close()

		if copyErr != nil {
			_ = os.Remove(tmpFile)
			return nil, fmt.Errorf("model download interrupted: %w", copyErr)
		}

		if err := os.Rename(tmpFile, modelPath); err != nil {
			_ = os.Remove(tmpFile)
			return nil, fmt.Errorf("failed to persist model file: %w", err)
		}

		log.Info().Str("filename", filename).Msg("AI model downloaded and cached on disk successfully")
		return nil, nil
	})

	if err != nil {
		log.Error().Err(err).Str("filename", filename).Msg("Model download failed")
		c.String(http.StatusBadGateway, "Failed to download model: %v", err)
		return
	}

	// 3. Serve the cached file
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Header("Access-Control-Allow-Origin", "*")
	c.File(modelPath)
}

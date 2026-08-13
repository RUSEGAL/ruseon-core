package recorder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/RUSEGAL/ruseon-core/pkg/config"
	"github.com/RUSEGAL/ruseon-core/pkg/registry"
)

// StartCleanupTask запускает фоновую горутину для очистки старых записей.
func StartCleanupTask(ctx context.Context, recordDir string, cfg *config.Config, store registry.StateStore) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().Interface("panic", r).Msg("Recovered from panic in StartCleanupTask")
			}
		}()
		ticker := time.NewTicker(1 * time.Hour)
		diskTicker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		defer diskTicker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cleanupOldFiles(recordDir, cfg, store)
			case <-diskTicker.C:
				checkDiskUsage(recordDir)
			}
		}
	}()
}

func cleanupOldFiles(recordDir string, cfg *config.Config, store registry.StateStore) {
	globalRetention := cfg.Server.RecordRetentionDays
	
	// Собираем мапу retention для камер
	camRetention := make(map[string]int)
	cams, _ := store.ListCameras()
	for _, cam := range cams {
		if cam.RetentionDays > 0 {
			camRetention[cam.ID] = cam.RetentionDays
		} else {
			camRetention[cam.ID] = globalRetention
		}
	}

	entries, err := registry.CurrentBlobStore.ReadDir(recordDir)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list record directory")
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		camID := entry.Name()
		camDir := filepath.Join(recordDir, camID)
		files, err := registry.CurrentBlobStore.ReadDir(camDir)
		if err != nil {
			continue
		}

		retention := globalRetention
		if val, ok := camRetention[camID]; ok {
			retention = val
		}

		if retention <= 0 {
			continue
		}

		cutoff := time.Now().AddDate(0, 0, -retention)
		
		for _, fileInfo := range files {
			if !fileInfo.IsDir() && filepath.Ext(fileInfo.Name()) == ".mp4" {
				info, err := fileInfo.Info()
				if err != nil {
					continue
				}
				if info.ModTime().Before(cutoff) {
					filePath := filepath.Join(camDir, fileInfo.Name())
					log.Info().Str("file", filePath).Msg("Removing old record file")
					if err := registry.CurrentBlobStore.Delete(filePath); err != nil {
						log.Warn().Err(err).Str("file", filePath).Msg("Failed to delete record file")
					}
				}
			}
		}
	}
}

func checkDiskUsage(dir string) {
	// Получаем абсолютный путь для корректной проверки
	absDir, err := filepath.Abs(dir)
	if err != nil {
		absDir = dir
	}
	
	if _, err := os.Stat(absDir); os.IsNotExist(err) {
		return // Ignore if folder doesn't exist yet
	}

	usage, err := disk.Usage(absDir)
	if err != nil {
		log.Warn().Err(err).Str("path", absDir).Msg("Failed to check disk usage")
		return
	}
	if usage.UsedPercent > 90.0 {
		log.Warn().Float64("used_percent", usage.UsedPercent).Msg("Storage warning: disk is almost full")
		if registry.CurrentEventBus != nil {
			registry.CurrentEventBus.Publish("storage_warning", "system", map[string]string{
				"used_percent": fmt.Sprintf("%.2f", usage.UsedPercent),
				"path":         absDir,
			})
		}
	}
}

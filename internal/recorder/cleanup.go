package recorder

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/RUSEGAL/REA-Stream-Engine/internal/config"
	"github.com/RUSEGAL/REA-Stream-Engine/internal/storage"
)

// StartCleanupTask запускает фоновую задачу для удаления старых записей.
func StartCleanupTask(ctx context.Context, recordDir string, cfg *config.Config, store *storage.Storage) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().Interface("panic", r).Msg("Recovered from panic in StartCleanupTask")
			}
		}()
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cleanupOldFiles(recordDir, cfg, store)
			}
		}
	}()
}

func cleanupOldFiles(recordDir string, cfg *config.Config, store *storage.Storage) {
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

	err := filepath.Walk(recordDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && filepath.Ext(path) == ".mp4" {
			// Папка камеры обычно является родительской для файла.
			// Путь выглядит так: recordings/test/2026-07-29...mp4
			dir := filepath.Dir(path)
			camID := filepath.Base(dir)
			
			retention := globalRetention
			if val, ok := camRetention[camID]; ok {
				retention = val
			}

			if retention <= 0 {
				return nil // храним вечно
			}

			cutoff := time.Now().AddDate(0, 0, -retention)
			if info.ModTime().Before(cutoff) {
				log.Info().Str("file", path).Msg("Removing old record file")
				os.Remove(path)
			}
		}
		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		log.Error().Err(err).Msg("Failed to cleanup old records")
	}
}

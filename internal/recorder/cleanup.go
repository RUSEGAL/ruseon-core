package recorder

import (
	"context"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
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

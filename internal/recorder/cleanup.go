package recorder

import (
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	"gritprofmediaserver/internal/config"
)

// StartCleanupTask запускает фоновую задачу для удаления старых записей.
func StartCleanupTask(recordDir string, cfg *config.Config) {
	go func() {
		for {
			cleanupOldFiles(recordDir, cfg)
			time.Sleep(1 * time.Hour)
		}
	}()
}

func cleanupOldFiles(recordDir string, cfg *config.Config) {
	globalRetention := cfg.Server.RecordRetentionDays
	
	// Собираем мапу retention для камер
	camRetention := make(map[string]int)
	for _, cam := range cfg.Cameras {
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

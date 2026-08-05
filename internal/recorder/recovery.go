package recorder

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	
	"github.com/RUSEGAL/REA-Stream-Engine/pkg/registry"
)

// RecoverCrashedFiles сканирует директорию с архивами при старте сервера и переименовывает
// незавершенные файлы (закончившиеся на _ongoing.mp4) в нормальный формат, используя
// время последнего изменения файла как время окончания записи.
// Это решает проблему резких сбоев питания: так как мы пишем fMP4 (fragmented MP4),
// сам файл внутри валиден и проигрывается, нужно только дать ему корректное имя.
func RecoverCrashedFiles(recordDir string) {
	log.Info().Msg("Scanning for crashed/ongoing MP4 recordings to recover...")

	entries, err := registry.CurrentBlobStore.ReadDir(recordDir)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Warn().Err(err).Msg("Failed to read recordings directory")
		}
		return
	}

	recoveredCount := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		streamDir := filepath.Join(recordDir, entry.Name())
		files, err := registry.CurrentBlobStore.ReadDir(streamDir)
		if err != nil {
			continue
		}

		for _, fileInfo := range files {
			if !fileInfo.IsDir() && strings.HasSuffix(fileInfo.Name(), "_ongoing.mp4") {
				oldPath := filepath.Join(streamDir, fileInfo.Name())

				// Получаем время обрыва (когда в файл последний раз производилась запись)
				info, err := fileInfo.Info()
				if err != nil {
					continue
				}

				// Извлекаем стартовое время из названия (пример: 2026-07-31_15-04-05_ongoing.mp4)
				parts := strings.Split(fileInfo.Name(), "_ongoing.mp4")
				if len(parts) != 2 {
					continue
				}
				startTimeStr := parts[0]

				// Формируем новое имя с датой модификации
				endTimeStr := info.ModTime().Format("15-04-05")
				newPath := filepath.Join(streamDir, fmt.Sprintf("%s_to_%s.mp4", startTimeStr, endTimeStr))

				// Переименовываем
				if err := registry.CurrentBlobStore.Rename(oldPath, newPath); err == nil {
					log.Info().Str("stream", entry.Name()).Str("recovered_file", newPath).Msg("Recovered crashed MP4 file")
					recoveredCount++
				}
			}
		}
	}

	if recoveredCount > 0 {
		log.Info().Int("count", recoveredCount).Msg("Successfully recovered MP4 files after power failure")
	} else {
		log.Info().Msg("No crashed recordings found.")
	}
}

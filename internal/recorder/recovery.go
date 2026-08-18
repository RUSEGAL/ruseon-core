package recorder

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	
	"github.com/RUSEGAL/ruseon-core/pkg/registry"
)

// ValidateFMP4File проверяет структурную целостность fMP4 файла:
// 1. Размер файла не менее 32 байт.
// 2. Наличие валидного блока инициализации (moov).
// 3. Наличие хотя бы одного медиа-фрагмента (moof + mdat).
func ValidateFMP4File(path string) (bool, error) {
	file, err := registry.CurrentBlobStore.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()

	stat, err := registry.CurrentBlobStore.Stat(path)
	if err != nil {
		return false, err
	}
	if stat.Size() < 32 {
		return false, fmt.Errorf("fMP4 file size too small: %d bytes", stat.Size())
	}

	var hasMoov, hasMoof, hasMdat bool
	for {
		var header [8]byte
		if _, err := io.ReadFull(file, header[:]); err != nil {
			break
		}
		size := binary.BigEndian.Uint32(header[0:4])
		boxType := string(header[4:8])

		if size < 8 {
			break
		}

		switch boxType {
		case "moov":
			hasMoov = true
		case "moof":
			hasMoof = true
		case "mdat":
			hasMdat = true
		}

		// #nosec G115 -- box size fits into int64
		remaining := int64(size) - 8
		if _, err := file.Seek(remaining, io.SeekCurrent); err != nil {
			break
		}
	}

	if !hasMoov {
		return false, fmt.Errorf("missing fMP4 moov initialization box")
	}
	if !hasMoof || !hasMdat {
		return false, fmt.Errorf("missing fMP4 media fragments (moof/mdat)")
	}

	return true, nil
}

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

				// Валидируем fMP4 перед включением в архив
				if valid, valErr := ValidateFMP4File(oldPath); !valid {
					log.Warn().Err(valErr).Str("stream", entry.Name()).Str("file", oldPath).
						Msg("Skipping corrupted or empty ongoing recording during crash recovery")

					// Изолируем битый файл
					corruptedPath := filepath.Join(streamDir, strings.TrimSuffix(fileInfo.Name(), ".mp4")+".corrupted")
					_ = registry.CurrentBlobStore.Rename(oldPath, corruptedPath)
					continue
				}

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
					if registry.CurrentEventBus != nil {
						registry.CurrentEventBus.Publish("archive_segment_ready", entry.Name(), map[string]string{"file": newPath})
					}
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

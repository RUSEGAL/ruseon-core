package archive

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/RUSEGAL/REA-Stream-Engine/pkg/registry"
)

// RecordInterval представляет один записанный отрезок видео
type RecordInterval struct {
	StartTime time.Time `json:"start"`
	EndTime   time.Time `json:"end"`
	Filename  string    `json:"filename"`
}

// GetCameraArchive сканирует директорию камеры и возвращает доступные отрезки
func GetCameraArchive(recordDir, cameraID string) ([]RecordInterval, error) {
	dir := filepath.Join(recordDir, cameraID)
	entries, err := registry.CurrentBlobStore.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []RecordInterval{}, nil
		}
		return nil, err
	}

	var intervals []RecordInterval

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".mp4") {
			continue
		}
		if strings.HasSuffix(name, "_ongoing.mp4") {
			// Текущая запись: 2026-07-31_15-04-05_ongoing.mp4
			base := strings.TrimSuffix(name, "_ongoing.mp4")
			start, err := time.ParseInLocation("2006-01-02_15-04-05", base, time.Local)
			if err == nil {
				info, _ := entry.Info()
				endTime := time.Now()
				if info != nil {
					endTime = info.ModTime() // Последнее обновление файла
				}
				intervals = append(intervals, RecordInterval{
					StartTime: start,
					EndTime:   endTime,
					Filename:  name,
				})
			}
		} else if strings.Contains(name, "_to_") {
			// Завершенная запись: 2026-07-31_15-04-05_to_16-04-05.mp4
			base := strings.TrimSuffix(name, ".mp4")
			parts := strings.Split(base, "_to_")
			if len(parts) == 2 {
				start, err1 := time.ParseInLocation("2006-01-02_15-04-05", parts[0], time.Local)
				end, err2 := time.ParseInLocation("15-04-05", parts[1], time.Local)
				if err1 == nil && err2 == nil {
					// Конец содержит только часы, минуты, секунды. Берем дату из start.
					endFull := time.Date(start.Year(), start.Month(), start.Day(), end.Hour(), end.Minute(), end.Second(), 0, time.Local)
					// Если время конца меньше времени начала, значит запись перевалила за полночь
					if endFull.Before(start) {
						endFull = endFull.AddDate(0, 0, 1)
					}
					
					intervals = append(intervals, RecordInterval{
						StartTime: start,
						EndTime:   endFull,
						Filename:  name,
					})
				}
			}
		}
	}

	return intervals, nil
}

package backup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/RUSEGAL/REA-Stream-Engine/pkg/registry"
)

// Worker управляет фоновым процессом создания бэкапов.
type Worker struct {
	store        registry.StateStore
	backupDir    string
	interval     time.Duration
	retentionDays int
}

// NewWorker создает новый воркер для бэкапов.
func NewWorker(store registry.StateStore, backupDir string, interval time.Duration, retentionDays int) *Worker {
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		log.Error().Err(err).Msg("Failed to create backup directory")
	}
	return &Worker{
		store:        store,
		backupDir:    backupDir,
		interval:     interval,
		retentionDays: retentionDays,
	}
}

// Run запускает периодическое создание бэкапов.
func (w *Worker) Run(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Interface("panic", r).Msg("Recovered from panic in Backup Worker")
		}
	}()
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// Делаем первый бэкап при старте (с небольшой задержкой)
	time.AfterFunc(5*time.Minute, func() {
		w.doBackup()
	})

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.doBackup()
		}
	}
}

func (w *Worker) doBackup() {
	filename := filepath.Join(w.backupDir, fmt.Sprintf("badger_backup_%s.bak", time.Now().Format("2006-01-02_15-04-05")))
	
	f, err := os.Create(filename)
	if err != nil {
		log.Error().Err(err).Str("file", filename).Msg("Failed to create backup file")
		return
	}
	defer f.Close()

	if err := w.store.BackupBadger(f); err != nil {
		log.Error().Err(err).Msg("Failed to backup BadgerDB")
		return
	}

	log.Info().Str("file", filename).Msg("Successfully created native database backup")
	w.cleanupOldBackups()
}

func (w *Worker) cleanupOldBackups() {
	if w.retentionDays <= 0 {
		return
	}

	files, err := os.ReadDir(w.backupDir)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read backup directory for cleanup")
		return
	}

	type backupFile struct {
		info os.FileInfo
		path string
	}

	backups := make([]backupFile, 0, len(files))
	for _, entry := range files {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".bak" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		backups = append(backups, backupFile{
			info: info,
			path: filepath.Join(w.backupDir, entry.Name()),
		})
	}

	// Сортируем от новых к старым
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].info.ModTime().After(backups[j].info.ModTime())
	})

	cutoffTime := time.Now().AddDate(0, 0, -w.retentionDays)
	
	for _, b := range backups {
		if b.info.ModTime().Before(cutoffTime) {
			if err := os.Remove(b.path); err != nil {
				log.Error().Err(err).Str("file", b.path).Msg("Failed to delete old backup")
			} else {
				log.Info().Str("file", b.path).Msg("Deleted old native backup")
			}
		}
	}
}

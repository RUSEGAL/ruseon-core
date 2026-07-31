package storage

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/rs/zerolog/log"

	"gritprofmediaserver/internal/config"
)

const (
	PrefixCamera = "camera:"
	PrefixTag    = "tag:"
)

// Storage обертка над BadgerDB
type Storage struct {
	db *badger.DB
}

// NewStorage открывает или создает БД в указанной директории.
func NewStorage(dir string) (*Storage, error) {
	opts := badger.DefaultOptions(dir)
	opts.Logger = nil // Отключаем дефолтные логи Badger (они слишком шумные)

	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	// Запускаем периодический Garbage Collection для BadgerDB (раз в час)
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			for err := db.RunValueLogGC(0.5); err == nil; err = db.RunValueLogGC(0.5) {
				log.Debug().Msg("BadgerDB Garbage Collection executed")
			}
		}
	}()

	return &Storage{db: db}, nil
}

// Close закрывает БД.
func (s *Storage) Close() error {
	return s.db.Close()
}

// SaveCamera сохраняет или обновляет камеру.
func (s *Storage) SaveCamera(cam *config.CameraConfig) error {
	data, err := json.Marshal(cam)
	if err != nil {
		return err
	}
	key := []byte(PrefixCamera + cam.ID)

	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, data)
	})
}

// UpdateCameraTx атомарно обновляет камеру. updateFn должна вернуть true, если нужно сохранить изменения.
func (s *Storage) UpdateCameraTx(id string, updateFn func(cam *config.CameraConfig) bool) error {
	key := []byte(PrefixCamera + id)
	return s.db.Update(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		
		var cam config.CameraConfig
		err = item.Value(func(val []byte) error {
			return json.Unmarshal(val, &cam)
		})
		if err != nil {
			return err
		}

		if changed := updateFn(&cam); changed {
			data, err := json.Marshal(cam)
			if err != nil {
				return err
			}
			return txn.Set(key, data)
		}
		
		return nil
	})
}

// GetCamera возвращает камеру по ID.
func (s *Storage) GetCamera(id string) (*config.CameraConfig, error) {
	key := []byte(PrefixCamera + id)
	var cam config.CameraConfig

	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &cam)
		})
	})

	if err != nil {
		return nil, err
	}
	return &cam, nil
}

// DeleteCamera удаляет камеру по ID.
func (s *Storage) DeleteCamera(id string) error {
	key := []byte(PrefixCamera + id)
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}

// ListCameras возвращает список всех камер.
func (s *Storage) ListCameras() ([]config.CameraConfig, error) {
	var cameras []config.CameraConfig

	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := []byte(PrefixCamera)

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(v []byte) error {
				var cam config.CameraConfig
				if err := json.Unmarshal(v, &cam); err != nil {
					return err
				}
				cameras = append(cameras, cam)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	return cameras, err
}

// SaveTag сохраняет тег.
func (s *Storage) SaveTag(tag *config.TagConfig) error {
	data, err := json.Marshal(tag)
	if err != nil {
		return err
	}
	key := []byte(PrefixTag + tag.ID)

	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, data)
	})
}

// DeleteTag удаляет тег.
func (s *Storage) DeleteTag(id string) error {
	key := []byte(PrefixTag + id)
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}

// GetTag возвращает тег по ID.
func (s *Storage) GetTag(id string) (*config.TagConfig, error) {
	key := []byte(PrefixTag + id)
	var tag config.TagConfig

	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &tag)
		})
	})

	if err != nil {
		return nil, err
	}
	return &tag, nil
}

// ListTags возвращает список всех тегов.
func (s *Storage) ListTags() ([]config.TagConfig, error) {
	var tags []config.TagConfig

	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := []byte(PrefixTag)

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(v []byte) error {
				var tag config.TagConfig
				if err := json.Unmarshal(v, &tag); err != nil {
					return err
				}
				tags = append(tags, tag)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	return tags, err
}

// MigrateFromConfig переносит данные из config.yaml в БД, если БД пуста.
// Внимание: после вызова этого метода, данные нужно удалить из конфига (это будет на этапе 18.2).
func (s *Storage) MigrateFromConfig(cfg *config.Config) error {
	// Проверяем, пуста ли база
	cams, err := s.ListCameras()
	if err != nil {
		return err
	}
	tags, err := s.ListTags()
	if err != nil {
		return err
	}

	if len(cams) == 0 && len(cfg.Cameras) > 0 {
		log.Info().Int("count", len(cfg.Cameras)).Msg("Migrating cameras from config.yaml to BadgerDB")
		for _, cam := range cfg.Cameras {
			// Копируем значение, чтобы не сохранять указатель на итератор
			c := cam 
			if err := s.SaveCamera(&c); err != nil {
				return fmt.Errorf("failed to migrate camera %s: %w", c.ID, err)
			}
		}
	}

	if len(tags) == 0 && len(cfg.GlobalTags) > 0 {
		log.Info().Int("count", len(cfg.GlobalTags)).Msg("Migrating global tags from config.yaml to BadgerDB")
		for _, tag := range cfg.GlobalTags {
			t := tag
			if err := s.SaveTag(&t); err != nil {
				return fmt.Errorf("failed to migrate tag %s: %w", t.ID, err)
			}
		}
	}

	return nil
}

// BackupData представляет структуру JSON-файла для ручного бэкапа.
type BackupData struct {
	Cameras []config.CameraConfig `json:"cameras"`
	Tags    []config.TagConfig    `json:"tags"`
}

// ExportJSON собирает все конфигурации камер и тегов в JSON-дамп.
func (s *Storage) ExportJSON() ([]byte, error) {
	cams, err := s.ListCameras()
	if err != nil {
		return nil, err
	}
	tags, err := s.ListTags()
	if err != nil {
		return nil, err
	}
	data := BackupData{
		Cameras: cams,
		Tags:    tags,
	}
	// Используем отступы для человекочитаемости
	return json.MarshalIndent(data, "", "  ")
}

// ImportJSON парсит JSON-дамп и атомарно записывает все камеры и теги.
func (s *Storage) ImportJSON(data []byte) error {
	var backup BackupData
	if err := json.Unmarshal(data, &backup); err != nil {
		return err
	}

	return s.db.Update(func(txn *badger.Txn) error {
		for _, cam := range backup.Cameras {
			c := cam
			val, err := json.Marshal(&c)
			if err != nil {
				return err
			}
			if err := txn.Set([]byte(PrefixCamera+c.ID), val); err != nil {
				return err
			}
		}
		for _, tag := range backup.Tags {
			t := tag
			val, err := json.Marshal(&t)
			if err != nil {
				return err
			}
			if err := txn.Set([]byte(PrefixTag+t.ID), val); err != nil {
				return err
			}
		}
		return nil
	})
}

// BackupBadger делает нативный дамп БД и пишет его в переданный io.Writer.
func (s *Storage) BackupBadger(w io.Writer) error {
	_, err := s.db.Backup(w, 0)
	return err
}

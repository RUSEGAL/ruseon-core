package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/rs/zerolog/log"

	"github.com/RUSEGAL/ruseon-core/internal/models"
	"github.com/RUSEGAL/ruseon-core/pkg/config"
	"github.com/go-co-op/gocron/v2"
	"github.com/samber/ro"
	rocron "github.com/samber/ro/plugins/cron"
)

var ErrNotFound = errors.New("not found")

const (
	PrefixCamera = "camera:"
	PrefixTag    = "tag:"
	PrefixFolder = "folder:"
	PrefixUser   = "user:"
)

// Storage обертка над BadgerDB
type Storage struct {
	db     *badger.DB
	cancel context.CancelFunc
}

// NewStorage открывает или создает БД в указанной директории.
func NewStorage(dir string) (*Storage, error) {
	opts := badger.DefaultOptions(dir)
	opts.Logger = nil // Отключаем дефолтные логи Badger (они слишком шумные)
	
	// Оптимизация потребления ресурсов (Этап 23.2)
	opts.MemTableSize = 16 << 20          // Уменьшаем MemTable с 64MB до 16MB
	opts.ValueLogFileSize = 64 << 20      // Уменьшаем файлы логов с 1GB до 64MB
	opts.NumMemtables = 1                 // Держим максимум 1 memtable в памяти (вместо 5)
	opts.NumLevelZeroTables = 1           // Меньше таблиц нулевого уровня в памяти
	opts.NumLevelZeroTablesStall = 2      // Сброс на диск происходит быстрее
	opts.SyncWrites = true                // Синхронная запись WAL для надежности контрольного слоя (ACID/fsync)
	
	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	// Запускаем периодический Garbage Collection для BadgerDB (каждую ночь в 03:00)
	sub := rocron.NewScheduler(gocron.CronJob("0 3 * * *", false)).Subscribe(ro.NewObserver(
		func(_ rocron.ScheduleJob) {
			// Лимит на работу GC - 30 минут, чтобы не создавать нагрузку днем
			gcCtx, gcCancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer gcCancel()

			log.Info().Msg("Starting nightly BadgerDB Garbage Collection")
			for {
				select {
				case <-gcCtx.Done():
					log.Info().Msg("Nightly BadgerDB GC time limit reached or stopped")
					return
				default:
					err := db.RunValueLogGC(0.5)
					if err != nil {
						if err != badger.ErrNoRewrite {
							log.Error().Err(err).Msg("BadgerDB GC error")
						} else {
							log.Debug().Msg("BadgerDB GC finished (no rewrite needed)")
						}
						return
					}
					log.Debug().Msg("BadgerDB Garbage Collection executed step")
				}
			}
		},
		func(err error) {
			log.Error().Err(err).Msg("BadgerDB GC cron error")
		},
		func() {},
	))

	cancel := func() {
		sub.Unsubscribe()
	}

	return &Storage{db: db, cancel: cancel}, nil
}

// Close закрывает БД.
func (s *Storage) Close() error {
	s.cancel() // Остановка горутины GC
	return s.db.Close()
}

// Ping проверяет доступность базы данных через открытие read-only транзакции.
func (s *Storage) Ping(ctx context.Context) error {
	if s.db == nil || s.db.IsClosed() {
		return errors.New("badger database is closed or uninitialized")
	}
	return s.db.View(func(_ *badger.Txn) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	})
}

// Sync сбрасывает все буферизованные записи BadgerDB на постоянный диск.
func (s *Storage) Sync() error {
	if s.db == nil || s.db.IsClosed() {
		return errors.New("badger database is closed or uninitialized")
	}
	return s.db.Sync()
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
	cameras := make([]config.CameraConfig, 0)

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
	tags := make([]config.TagConfig, 0)

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

// SaveFolder сохраняет папку.
func (s *Storage) SaveFolder(folder *config.FolderConfig) error {
	data, err := json.Marshal(folder)
	if err != nil {
		return err
	}
	key := []byte(PrefixFolder + folder.ID)

	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, data)
	})
}

// DeleteFolder удаляет папку.
func (s *Storage) DeleteFolder(id string) error {
	key := []byte(PrefixFolder + id)
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}

// GetFolder возвращает папку по ID.
func (s *Storage) GetFolder(id string) (*config.FolderConfig, error) {
	key := []byte(PrefixFolder + id)
	var folder config.FolderConfig

	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &folder)
		})
	})

	if err != nil {
		return nil, err
	}
	return &folder, nil
}

// ListFolders возвращает список всех папок.
func (s *Storage) ListFolders() ([]config.FolderConfig, error) {
	folders := make([]config.FolderConfig, 0)

	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := []byte(PrefixFolder)

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(v []byte) error {
				var folder config.FolderConfig
				if err := json.Unmarshal(v, &folder); err != nil {
					return err
				}
				folders = append(folders, folder)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	return folders, err
}

// SaveUser сохраняет объект пользователя.
func (s *Storage) SaveUser(user *models.User) error {
	key := []byte(PrefixUser + user.Username)
	data, err := json.Marshal(user)
	if err != nil {
		return err
	}
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, data)
	})
}

// GetUser возвращает объект пользователя по имени.
func (s *Storage) GetUser(username string) (*models.User, error) {
	key := []byte(PrefixUser + username)
	var user models.User
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &user)
		})
	})
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

// ListUsers возвращает список всех пользователей.
func (s *Storage) ListUsers() ([]models.User, error) {
	var users []models.User
	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefix := []byte(PrefixUser)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			var user models.User
			err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &user)
			})
			if err != nil {
				return err
			}
			users = append(users, user)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return users, nil
}

// DeleteUser удаляет пользователя по имени.
func (s *Storage) DeleteUser(username string) error {
	key := []byte(PrefixUser + username)
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}

// HasUsers возвращает true, если в БД есть хотя бы один пользователь.
func (s *Storage) HasUsers() (bool, error) {
	hasUsers := false
	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := []byte(PrefixUser)
		it.Seek(prefix)
		if it.ValidForPrefix(prefix) {
			hasUsers = true
		}
		return nil
	})
	return hasUsers, err
}


// MigrateFromConfig атомарно переносит данные из config.yaml в БД при первом старте.
func (s *Storage) MigrateFromConfig(cfg *config.Config) error {
	if cfg == nil {
		return nil
	}

	return s.db.Update(func(txn *badger.Txn) error {
		// Проверяем наличие существующих камер внутри транзакции
		optsCam := badger.DefaultIteratorOptions
		optsCam.PrefetchValues = false
		optsCam.Prefix = []byte(PrefixCamera)
		itCam := txn.NewIterator(optsCam)
		defer itCam.Close()
		itCam.Seek([]byte(PrefixCamera))
		hasCameras := itCam.ValidForPrefix([]byte(PrefixCamera))

		if !hasCameras && len(cfg.Cameras) > 0 {
			log.Info().Int("count", len(cfg.Cameras)).Msg("Migrating cameras from config.yaml to BadgerDB")
			for _, cam := range cfg.Cameras {
				c := cam
				data, err := json.Marshal(&c)
				if err != nil {
					return fmt.Errorf("failed to marshal camera %s: %w", c.ID, err)
				}
				if err := txn.Set([]byte(PrefixCamera+c.ID), data); err != nil {
					return fmt.Errorf("failed to migrate camera %s: %w", c.ID, err)
				}
			}
		}

		// Проверяем наличие существующих тегов внутри той же транзакции
		optsTag := badger.DefaultIteratorOptions
		optsTag.PrefetchValues = false
		optsTag.Prefix = []byte(PrefixTag)
		itTag := txn.NewIterator(optsTag)
		defer itTag.Close()
		itTag.Seek([]byte(PrefixTag))
		hasTags := itTag.ValidForPrefix([]byte(PrefixTag))

		if !hasTags && len(cfg.GlobalTags) > 0 {
			log.Info().Int("count", len(cfg.GlobalTags)).Msg("Migrating global tags from config.yaml to BadgerDB")
			for _, tag := range cfg.GlobalTags {
				t := tag
				data, err := json.Marshal(&t)
				if err != nil {
					return fmt.Errorf("failed to marshal tag %s: %w", t.ID, err)
				}
				if err := txn.Set([]byte(PrefixTag+t.ID), data); err != nil {
					return fmt.Errorf("failed to migrate tag %s: %w", t.ID, err)
				}
			}
		}

		return nil
	})
}

// BackupData представляет структуру JSON-файла для ручного бэкапа.
type BackupData struct {
	Cameras []config.CameraConfig `json:"cameras"`
	Tags    []config.TagConfig    `json:"tags"`
	Folders []config.FolderConfig `json:"folders,omitempty"`
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
	folders, err := s.ListFolders()
	if err != nil {
		return nil, err
	}
	data := BackupData{
		Cameras: cams,
		Tags:    tags,
		Folders: folders,
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
		for _, folder := range backup.Folders {
			f := folder
			val, err := json.Marshal(&f)
			if err != nil {
				return err
			}
			if err := txn.Set([]byte(PrefixFolder+f.ID), val); err != nil {
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

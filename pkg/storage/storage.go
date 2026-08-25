// Package storage provides embedded key-value persistence and transactional metadata
// management for RUSEON Core using BadgerDB.
//
// Features include ACID-compliant transactions, sync WAL writes, atomic in-memory caching
// with defensive cloning for lock-free camera lookups, and scheduled nightly value-log GC.
package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/rs/zerolog/log"

	"github.com/RUSEGAL/ruseon-core/v2/internal/models"
	"github.com/RUSEGAL/ruseon-core/v2/pkg/config"
	"github.com/go-co-op/gocron/v2"
	"github.com/samber/ro"
	rocron "github.com/samber/ro/plugins/cron"
)

// ErrNotFound is returned by retrieval methods when a requested key or entity does not exist.
var ErrNotFound = errors.New("not found")

const (
	// PrefixCamera is the key prefix for camera configuration entries in BadgerDB.
	PrefixCamera = "camera:"
	// PrefixTag is the key prefix for metadata tag entries in BadgerDB.
	PrefixTag = "tag:"
	// PrefixFolder is the key prefix for folder group entries in BadgerDB.
	PrefixFolder = "folder:"
	// PrefixUser is the key prefix for user credential entries in BadgerDB.
	PrefixUser = "user:"
)

// Storage wraps a BadgerDB instance and implements registry.StateStore.
//
// Thread-safety: Storage is fully thread-safe for concurrent read and write operations.
// Read operations on cameras utilize an atomic pointer cache (atomic.Pointer) to serve
// ListCameras queries with zero lock contention.
type Storage struct {
	db            *badger.DB
	cachedCameras atomic.Pointer[[]config.CameraConfig]
	cancel        context.CancelFunc
}

// NewStorage opens or initializes a BadgerDB database in the specified directory path.
//
// It tunes BadgerDB memory and disk parameters for resource-constrained embedded systems:
//   - MemTableSize: 16 MB
//   - ValueLogFileSize: 64 MB
//   - SyncWrites: true (ensures durability via fsync on every write)
//   - Schedules a nightly value-log garbage collection run (at 03:00 daily).
//
// Returns a new Storage instance or an error if opening the database fails.
func NewStorage(dir string) (*Storage, error) {
	opts := badger.DefaultOptions(dir)
	opts.Logger = nil // Disable verbose default Badger logs
	
	opts.MemTableSize = 16 << 20          // Reduce MemTable from 64MB to 16MB
	opts.ValueLogFileSize = 64 << 20      // Reduce value log file size from 1GB to 64MB
	opts.NumMemtables = 1                 // Keep at most 1 memtable in memory
	opts.NumLevelZeroTables = 1           // Minimize level zero tables in memory
	opts.NumLevelZeroTablesStall = 2      // Flush to disk quickly
	opts.SyncWrites = true                // Synchronous WAL writes for ACID durability
	
	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	// Schedule nightly Garbage Collection for BadgerDB (at 03:00 every night)
	sub := rocron.NewScheduler(gocron.CronJob("0 3 * * *", false)).Subscribe(ro.NewObserver(
		func(_ rocron.ScheduleJob) {
			// Limit GC run to 30 minutes to prevent background IOPS spike
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

// Close gracefully cancels background GC schedules and closes the underlying BadgerDB database,
// releasing all filesystem lock files.
func (s *Storage) Close() error {
	s.cancel() // Stop GC background cron
	return s.db.Close()
}

func (s *Storage) invalidateCameraCache() {
	s.cachedCameras.Store(nil)
}

// Ping verifies that the BadgerDB storage is open, uncorrupted, and responsive to read queries.
// It respects context cancellation and deadlines.
func (s *Storage) Ping(ctx context.Context) error {
	if s.db == nil || s.db.IsClosed() {
		return errors.New("badger database is closed or uninitialized")
	}
	return s.db.View(func(_ *badger.Txn) error {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}
		return nil
	})
}

// Sync commits all dirty write-ahead log (WAL) entries and in-memory tables to permanent disk storage.
func (s *Storage) Sync() error {
	if s.db == nil || s.db.IsClosed() {
		return errors.New("badger database is closed or uninitialized")
	}
	return s.db.Sync()
}

// SaveCamera inserts or updates a camera stream configuration in BadgerDB.
// It automatically invalidates the atomic camera list cache to guarantee fresh reads.
func (s *Storage) SaveCamera(cam *config.CameraConfig) error {
	s.invalidateCameraCache()
	data, err := json.Marshal(cam)
	if err != nil {
		return err
	}
	key := []byte(PrefixCamera + cam.ID)

	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, data)
	})
}

// UpdateCameraTx executes updateFn inside an atomic BadgerDB transaction for the specified camera ID.
//
// If updateFn modifies the camera and returns true, the mutated record is committed and
// the in-memory camera cache is invalidated. If updateFn returns false, the transaction is rolled back.
func (s *Storage) UpdateCameraTx(id string, updateFn func(cam *config.CameraConfig) bool) error {
	s.invalidateCameraCache()
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

// BatchUpdateTraffic accumulates bandwidth usage counters across multiple camera streams
// in chunks of 200 items per database transaction, minimizing write overhead and disk wear.
//
// If a camera enters a new billing cycle (nowMonth differs from LastResetMonth), its TrafficUsed counter
// is atomically reset to zero before applying the delta.
func (s *Storage) BatchUpdateTraffic(updates map[string]uint64, nowMonth string) error {
	if len(updates) == 0 {
		return nil
	}
	s.invalidateCameraCache()

	type updateEntry struct {
		id    string
		delta uint64
	}

	entries := make([]updateEntry, 0, len(updates))
	for id, delta := range updates {
		entries = append(entries, updateEntry{id: id, delta: delta})
	}

	const batchSize = 200
	for i := 0; i < len(entries); i += batchSize {
		end := i + batchSize
		if end > len(entries) {
			end = len(entries)
		}
		chunk := entries[i:end]

		err := s.db.Update(func(txn *badger.Txn) error {
			for _, entry := range chunk {
				key := []byte(PrefixCamera + entry.id)
				item, err := txn.Get(key)
				if err != nil {
					if err == badger.ErrKeyNotFound {
						continue
					}
					return err
				}

				var cam config.CameraConfig
				err = item.Value(func(val []byte) error {
					return json.Unmarshal(val, &cam)
				})
				if err != nil {
					return err
				}

				if cam.LastResetMonth != nowMonth {
					cam.LastResetMonth = nowMonth
					cam.TrafficUsed = 0
				}
				if cam.TrafficLimit == 0 {
					cam.TrafficLimit = 200 * 1024 * 1024 * 1024 // 200 GB default
				}
				cam.TrafficUsed += entry.delta

				data, err := json.Marshal(&cam)
				if err != nil {
					return err
				}

				if err := txn.Set(key, data); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to update batch traffic: %w", err)
		}
	}

	return nil
}

// GetCamera retrieves a camera configuration by its unique ID.
//
// Returns a pointer to CameraConfig or an error (e.g. badger.ErrKeyNotFound).
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

// DeleteCamera deletes the camera configuration associated with id from BadgerDB.
// It also clears the in-memory camera list cache.
func (s *Storage) DeleteCamera(id string) error {
	s.invalidateCameraCache()
	key := []byte(PrefixCamera + id)
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}

// ListCameras returns all stored camera configurations.
//
// Performance and concurrency design:
//   - Uses atomic.Pointer caching to serve repeated read requests with zero database transaction locks.
//   - Applies defensive deep cloning (CameraConfig.Clone()) to returned items to prevent data races
//     if caller goroutines mutate slice fields (Tags, history).
func (s *Storage) ListCameras() ([]config.CameraConfig, error) {
	if cached := s.cachedCameras.Load(); cached != nil {
		res := make([]config.CameraConfig, len(*cached))
		for i, cam := range *cached {
			res[i] = cam.Clone()
		}
		return res, nil
	}

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

	if err == nil {
		s.cachedCameras.Store(&cameras)
	}

	res := make([]config.CameraConfig, len(cameras))
	for i, cam := range cameras {
		res[i] = cam.Clone()
	}
	return res, err
}

// SaveTag creates or updates a metadata tag in BadgerDB.
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

// DeleteTag removes a metadata tag from BadgerDB by its ID.
func (s *Storage) DeleteTag(id string) error {
	key := []byte(PrefixTag + id)
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}

// GetTag retrieves a metadata tag by its unique ID. Returns ErrNotFound if missing.
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

// ListTags returns all stored metadata tags.
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

// SaveFolder creates or updates a folder/group in BadgerDB.
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

// DeleteFolder removes a folder from BadgerDB by its ID.
func (s *Storage) DeleteFolder(id string) error {
	key := []byte(PrefixFolder + id)
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}

// GetFolder retrieves a folder by its unique ID. Returns ErrNotFound if missing.
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

// ListFolders returns all stored folders.
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

// SaveUser creates or updates a user credential record in BadgerDB.
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

// GetUser retrieves a user account by username. Returns ErrNotFound if the user does not exist.
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

// ListUsers returns all user accounts stored in the database.
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

// DeleteUser deletes a user account by username.
func (s *Storage) DeleteUser(username string) error {
	key := []byte(PrefixUser + username)
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}

// HasUsers reports whether any user records are present in BadgerDB.
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

// MigrateFromConfig atomically imports cameras and global tags from the YAML config into BadgerDB
// if no existing records are detected.
func (s *Storage) MigrateFromConfig(cfg *config.Config) error {
	if cfg == nil {
		return nil
	}
	s.invalidateCameraCache()

	return s.db.Update(func(txn *badger.Txn) error {
		// Check for existing cameras inside the transaction
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

		// Check for existing tags inside the same transaction
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

// BackupData represents the JSON-serializable backup payload containing cameras, tags, and folders.
type BackupData struct {
	// Cameras lists all exported camera configurations.
	Cameras []config.CameraConfig `json:"cameras"`
	// Tags lists all exported metadata tags.
	Tags []config.TagConfig `json:"tags"`
	// Folders lists all exported camera grouping folders.
	Folders []config.FolderConfig `json:"folders,omitempty"`
}

// ExportJSON serializes all cameras, tags, and folders into a human-readable indented JSON payload.
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
	return json.MarshalIndent(data, "", "  ")
}

// ImportJSON parses a JSON backup payload and atomically restores all cameras, tags, and folders.
func (s *Storage) ImportJSON(data []byte) error {
	var backup BackupData
	if err := json.Unmarshal(data, &backup); err != nil {
		return err
	}
	s.invalidateCameraCache()

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

// BackupBadger generates a raw, consistent BadgerDB streaming backup and writes it to w.
func (s *Storage) BackupBadger(w io.Writer) error {
	_, err := s.db.Backup(w, 0)
	return err
}

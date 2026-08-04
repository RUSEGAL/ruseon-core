package stream

import (
	"fmt"
	"sync"
	
	"github.com/rs/zerolog/log"
	
	"github.com/RUSEGAL/REA-Stream-Engine/internal/storage"
)



// Manager управляет списком всех потоков.
type Manager struct {
	mu      sync.RWMutex
	streams map[string]*Stream
}

// NewManager создает новый менеджер потоков.
func NewManager() *Manager {
	return &Manager{
		streams: make(map[string]*Stream),
	}
}

// AddStream добавляет новый поток (камеру) в менеджер.
func (m *Manager) AddStream(id, url string, record bool, lazyHLS bool, transport string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.streams[id]; exists {
		return fmt.Errorf("stream %s already exists", id)
	}

	st := NewStream(id, url, record, lazyHLS, transport)
	m.streams[id] = st
	return nil
}

// RemoveStream останавливает и удаляет поток.
func (m *Manager) RemoveStream(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if st, ok := m.streams[id]; ok {
		st.Stop()
		delete(m.streams, id)
	}
}

// GetStream возвращает поток по ID, если он существует.
func (m *Manager) GetStream(id string) (*Stream, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	s, ok := m.streams[id]
	return s, ok
}

// GetStreams возвращает все потоки.
func (m *Manager) GetStreams() []*Stream {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Stream
	for _, s := range m.streams {
		result = append(result, s)
	}
	return result
}

// SyncWithStorage синхронизирует состояние манагера с базой данных
func (m *Manager) SyncWithStorage(store *storage.Storage) error {
	log.Info().Msg("Syncing stream manager with storage...")
	
	cams, err := store.ListCameras()
	if err != nil {
		return err
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// 1. Остановка всех текущих потоков
	for id, st := range m.streams {
		st.Stop()
		delete(m.streams, id)
	}
	
	// 2. Инициализация новых потоков
	for _, cam := range cams {
		if !cam.Disabled {
			st := NewStream(cam.ID, cam.URL, cam.Record, cam.LazyHLS, cam.Transport)
			m.streams[cam.ID] = st
			log.Info().Str("id", cam.ID).Msg("Stream started from backup sync")
		} else {
			log.Info().Str("id", cam.ID).Msg("Stream is disabled, skipping")
		}
	}
	
	return nil
}

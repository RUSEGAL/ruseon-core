package stream

import (
	"context"
	"errors"
	"fmt"
	"sync"
	
	"github.com/rs/zerolog/log"
	
	"github.com/RUSEGAL/ruseon-core/pkg/registry"
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

// Ready проверяет работоспособность менеджера потоков.
func (m *Manager) Ready(ctx context.Context) error {
	if m == nil {
		return errors.New("stream manager is not initialized")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
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

	result := make([]*Stream, 0, len(m.streams))
	for _, s := range m.streams {
		result = append(result, s)
	}
	return result
}

// HasStream проверяет, зарегистрирован ли поток в менеджере.
func (m *Manager) HasStream(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.streams[id]
	return exists
}

// UpsertStream атомарно создает, обновляет или останавливает поток в зависимости от конфигурации.
// Если поток отключен (disabled=true), он останавливается и удаляется из менеджера.
// Если поток включен (disabled=false), он создается или обновляется при изменении параметров.
func (m *Manager) UpsertStream(id, url string, record bool, lazyHLS bool, transport string, disabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.streams[id]
	if disabled {
		if exists {
			existing.Stop()
			delete(m.streams, id)
			log.Info().Str("id", id).Msg("Stream stopped and removed (disabled)")
		}
		return
	}

	if exists {
		if existing.MatchesConfig(url, record, lazyHLS, transport) {
			// Параметры стрима не изменились, оставляем работать без прерывания
			return
		}
		// Параметры изменились, пересоздаем стрим
		existing.Stop()
		delete(m.streams, id)
		log.Info().Str("id", id).Msg("Stream parameters changed, recreating stream")
	}

	st := NewStream(id, url, record, lazyHLS, transport)
	m.streams[id] = st
}

// SyncWithStorage синхронизирует состояние потоков с базой.
func (m *Manager) SyncWithStorage(store registry.StateStore) error {
	log.Info().Msg("Syncing stream manager with storage...")
	
	cams, err := store.ListCameras()
	if err != nil {
		return err
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	activeIDs := make(map[string]struct{}, len(cams))
	for _, cam := range cams {
		if !cam.Disabled {
			activeIDs[cam.ID] = struct{}{}
			if existing, exists := m.streams[cam.ID]; exists {
				if !existing.MatchesConfig(cam.URL, cam.Record, cam.LazyHLS, cam.Transport) {
					existing.Stop()
					m.streams[cam.ID] = NewStream(cam.ID, cam.URL, cam.Record, cam.LazyHLS, cam.Transport)
				}
			} else {
				m.streams[cam.ID] = NewStream(cam.ID, cam.URL, cam.Record, cam.LazyHLS, cam.Transport)
				log.Info().Str("id", cam.ID).Msg("Stream started from backup sync")
			}
		} else {
			log.Info().Str("id", cam.ID).Msg("Stream is disabled, skipping")
		}
	}
	
	// Остановка потоков, которых больше нет в активном списке
	for id, st := range m.streams {
		if _, ok := activeIDs[id]; !ok {
			st.Stop()
			delete(m.streams, id)
			log.Info().Str("id", id).Msg("Stream stopped (removed or disabled in storage)")
		}
	}
	
	return nil
}

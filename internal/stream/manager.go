package stream

import (
	"fmt"
	"sync"
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

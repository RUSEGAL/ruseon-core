package stream

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	
	"github.com/rs/zerolog/log"
	
	"github.com/RUSEGAL/ruseon-core/pkg/registry"
)

// Manager управляет списком всех потоков и запускает централизованный шедулер фоновых задач.
type Manager struct {
	mu      sync.RWMutex
	streams map[string]*Stream

	cumFrames     atomic.Uint64
	cumBytes      atomic.Uint64
	cumBytesSent  atomic.Uint64
	cumDrops      atomic.Uint64
	cumReconnects atomic.Uint64

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}


// NewManager создает новый менеджер потоков и запускает фоновый цикл обслуживания.
func NewManager() *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		streams: make(map[string]*Stream),
		ctx:     ctx,
		cancel:  cancel,
	}
	m.wg.Add(1)
	go m.housekeepingLoop()
	return m
}

func (m *Manager) housekeepingLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		// Каждая итерация изолирована: паника в одном стриме не убивает горутину шедулера
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error().Interface("panic", r).Msg("Recovered from panic in Manager.housekeepingLoop iteration")
				}
			}()

			select {
			case <-m.ctx.Done():
				return
			case t := <-ticker.C:
				streams := m.GetStreams()
				for _, st := range streams {
					st.TickHousekeeping(t)
				}
			}
		}()

		select {
		case <-m.ctx.Done():
			return
		default:
		}
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
func (m *Manager) AddStream(id, url string, record bool, lazyHLS bool, transport string, tokenAuth bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.streams[id]; exists {
		return fmt.Errorf("stream %s already exists", id)
	}

	st := NewStream(id, url, record, lazyHLS, transport, tokenAuth)
	m.streams[id] = st
	return nil
}

// RemoveStream останавливает и удаляет поток без удержания блокировки менеджера во время Stop().
func (m *Manager) RemoveStream(id string) {
	var toStop *Stream

	m.mu.Lock()
	if st, ok := m.streams[id]; ok {
		delete(m.streams, id)
		toStop = st
		// Сохраняем исторические метрики удаляемого потока в кумулятивный пул менеджера,
		// чтобы они не терялись при реконнектах или динамическом редактировании камер.
		m.cumFrames.Add(st.framesReceived.Load())
		m.cumBytes.Add(st.bytesReceived.Load())
		m.cumBytesSent.Add(st.bytesSent.Load())
		m.cumReconnects.Add(st.reconnects.Load())
		if st.ringBuffer != nil {
			m.cumDrops.Add(st.ringBuffer.GetTotalDrops())
		}
	}
	m.mu.Unlock()

	if toStop != nil {
		toStop.Stop()
	}
}

// GetCumulativeStats возвращает агрегированные счетчики за все время работы сервера.
// Суммирует исторически накопленные данные удаленных потоков (cum*) и живые метрики активных потоков.
func (m *Manager) GetCumulativeStats() (frames, bytesIn, bytesOut, drops, reconnects uint64) {
	frames = m.cumFrames.Load()
	bytesIn = m.cumBytes.Load()
	bytesOut = m.cumBytesSent.Load()
	drops = m.cumDrops.Load()
	reconnects = m.cumReconnects.Load()

	for _, st := range m.GetStreams() {
		stats := st.GetStats()
		frames += stats.Frames
		bytesIn += stats.BytesReceived
		bytesOut += stats.BytesSent
		reconnects += stats.Reconnects
		if st.GetRingBuffer() != nil {
			drops += st.GetRingBuffer().GetTotalDrops()
		}
	}
	return
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
// Остановка старого потока вынесена за пределы блокировки менеджера.
func (m *Manager) UpsertStream(id, url string, record bool, lazyHLS bool, transport string, tokenAuth bool, disabled bool) {
	var toStop *Stream
	var newStream *Stream

	m.mu.Lock()
	existing, exists := m.streams[id]
	switch {
	case disabled:
		if exists {
			delete(m.streams, id)
			toStop = existing
		}
	case exists:
		if !existing.MatchesConfig(url, record, lazyHLS, transport, tokenAuth) {
			toStop = existing
			newStream = NewStream(id, url, record, lazyHLS, transport, tokenAuth)
			m.streams[id] = newStream
		}
	default:
		newStream = NewStream(id, url, record, lazyHLS, transport, tokenAuth)
		m.streams[id] = newStream
	}
	m.mu.Unlock()

	if toStop != nil {
		toStop.Stop()
		if disabled {
			log.Info().Str("id", id).Msg("Stream stopped and removed (disabled)")
		} else {
			log.Info().Str("id", id).Msg("Stream parameters changed, recreating stream")
		}
	}
}

// SyncWithStorage синхронизирует состояние потоков с базой без удержания блокировки во время остановок.
func (m *Manager) SyncWithStorage(store registry.StateStore) error {
	log.Info().Msg("Syncing stream manager with storage...")
	
	cams, err := store.ListCameras()
	if err != nil {
		return err
	}
	
	var toStop []*Stream

	m.mu.Lock()
	activeIDs := make(map[string]struct{}, len(cams))
	for _, cam := range cams {
		if !cam.Disabled {
			activeIDs[cam.ID] = struct{}{}
			if existing, exists := m.streams[cam.ID]; exists {
				if !existing.MatchesConfig(cam.URL, cam.Record, cam.LazyHLS, cam.Transport, cam.TokenAuth) {
					toStop = append(toStop, existing)
					m.streams[cam.ID] = NewStream(cam.ID, cam.URL, cam.Record, cam.LazyHLS, cam.Transport, cam.TokenAuth)
				}
			} else {
				m.streams[cam.ID] = NewStream(cam.ID, cam.URL, cam.Record, cam.LazyHLS, cam.Transport, cam.TokenAuth)
				log.Info().Str("id", cam.ID).Msg("Stream started from backup sync")
			}
		} else {
			log.Info().Str("id", cam.ID).Msg("Stream is disabled, skipping")
		}
	}
	
	// Остановка потоков, которых больше нет в активном списке
	for id, st := range m.streams {
		if _, ok := activeIDs[id]; !ok {
			toStop = append(toStop, st)
			delete(m.streams, id)
			log.Info().Str("id", id).Msg("Stream marked for stop (removed or disabled in storage)")
		}
	}
	m.mu.Unlock()

	// Останавливаем потоки вне блокировки менеджера
	for _, st := range toStop {
		st.Stop()
	}
	
	return nil
}

// Close останавливает фоновый шедулер и все зарегистрированные потоки.
func (m *Manager) Close() {
	m.cancel()
	m.wg.Wait()

	m.mu.Lock()
	all := make([]*Stream, 0, len(m.streams))
	for id, st := range m.streams {
		all = append(all, st)
		delete(m.streams, id)
	}
	m.mu.Unlock()

	for _, st := range all {
		st.Stop()
	}
}

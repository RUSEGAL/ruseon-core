package stream

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	
	"github.com/rs/zerolog/log"
	
	"github.com/RUSEGAL/ruseon-core/v2/pkg/registry"
)

// Manager coordinates all active camera streams, provides thread-safe access and synchronization
// against StateStore, tracks cumulative lifetime telemetry metrics, and runs a centralized 1-second housekeeping loop.
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

// NewManager creates and initializes a new Stream Manager and starts its centralized 1-second housekeeping loop.
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
		select {
		case <-m.ctx.Done():
			return
		case t := <-ticker.C:
			streams := m.GetStreams()
			for _, st := range streams {
				func() {
					defer func() {
						if r := recover(); r != nil {
							log.Error().Interface("panic", r).Msg("Recovered from panic in stream housekeeping")
						}
					}()
					if st != nil {
						st.TickHousekeeping(t)
					}
				}()
			}
		}
	}
}

// Ready verifies that the stream manager is initialized and healthy.
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

// AddStream instantiates and registers a new camera stream under the given unique ID.
// Returns an error if a stream with the specified ID is already registered.
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

// RemoveStream removes a stream from the registry and stops its background ingest loop.
//
// Concurrency note: stream.Stop() is invoked outside the Manager mutex lock to prevent deadlocks.
// Cumulative counters from the terminated stream are preserved in the manager's lifetime pool.
func (m *Manager) RemoveStream(id string) {
	var toStop *Stream

	m.mu.Lock()
	if st, ok := m.streams[id]; ok {
		delete(m.streams, id)
		toStop = st
		// Retain historical metrics from the removed stream into the manager's cumulative pool
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

// GetCumulativeStats returns total lifetime metrics aggregated across both past (deleted/restarted)
// streams and currently active streams.
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

// GetStream retrieves an active Stream instance by camera ID.
// Returns the stream pointer and true if found, or nil and false otherwise.
func (m *Manager) GetStream(id string) (*Stream, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	s, ok := m.streams[id]
	return s, ok
}

// GetStreams returns a snapshot slice of all currently registered active streams.
func (m *Manager) GetStreams() []*Stream {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Stream, 0, len(m.streams))
	for _, s := range m.streams {
		result = append(result, s)
	}
	return result
}

// HasStream reports whether a stream with the specified ID is registered.
func (m *Manager) HasStream(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.streams[id]
	return exists
}

// UpsertStream creates, reconfigures, or removes a camera stream to match updated settings.
// If existing parameters have changed, the old stream is cleanly stopped and replaced.
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

// SyncWithStorage reconciles active in-memory streams with the camera configurations stored in StateStore.
// Cameras missing or marked as disabled in storage are stopped and removed.
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
	
	// Stop streams no longer present in active store list
	for id, st := range m.streams {
		if _, ok := activeIDs[id]; !ok {
			toStop = append(toStop, st)
			delete(m.streams, id)
			log.Info().Str("id", id).Msg("Stream marked for stop (removed or disabled in storage)")
		}
	}
	m.mu.Unlock()

	// Stop obsolete streams outside of manager mutex lock
	for _, st := range toStop {
		st.Stop()
	}
	
	return nil
}

// Close terminates the background housekeeping loop and stops all registered camera streams.
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

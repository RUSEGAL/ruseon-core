// Package eventbus provides an asynchronous, non-blocking event distribution
// pipeline for internal server events and outbound HTTP webhooks.
//
// Features include consistent hashing by Camera ID to maintain strict per-stream
// event ordering across concurrent workers, non-blocking queueing with drop-newest
// semantics on overload, and per-endpoint Circuit Breakers for failure isolation.
package eventbus

import (
	"encoding/json"
	"hash/fnv"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RUSEGAL/ruseon-core/v2/pkg/config"
	"github.com/RUSEGAL/ruseon-core/v2/pkg/metrics"
	"github.com/rs/zerolog/log"
)

// Event represents an individual system or telemetry notification.
type Event struct {
	// ID is the unique event identifier (optional).
	ID string `json:"id,omitempty"`
	// TimestampMs is the Unix epoch timestamp in milliseconds when the event was generated.
	TimestampMs int64 `json:"timestamp_ms"`
	// Topic describes the event type (e.g. "camera_online", "camera_offline", "storage_warning", "recording_failed").
	Topic string `json:"topic"`
	// CameraID is the optional identifier of the camera associated with the event.
	CameraID string `json:"camera_id,omitempty"`
	// Data holds arbitrary structured event payload.
	Data any `json:"data,omitempty"`
}

// Bus defines the common interface for publishing events and terminating the event bus.
type Bus interface {
	// Publish dispatches an event asynchronously across worker queues.
	Publish(topic string, cameraID string, data any)
	// Stop gracefully terminates all background workers and waits for in-flight deliveries.
	Stop()
}

// EventBus implements Bus and handles concurrent distribution of events to configured HTTP webhooks.
//
// Concurrency guarantees:
//   - Safe for concurrent calls by multiple goroutines.
//   - Uses consistent hashing on CameraID to ensure all events for a given camera are processed
//     sequentially in FIFO order by the same worker goroutine.
//   - Non-blocking publishing: if a worker's buffer (1000 events) is full, new events are dropped
//     and recorded via metrics.EventbusDropsTotal to protect producers from stalling.
type EventBus struct {
	mu              sync.RWMutex
	webhooks        []config.WebhookConfig
	workers         []chan Event
	circuitBreakers map[string]time.Time // Shared Circuit Breaker state across workers (URL -> unlock time)
	wg              sync.WaitGroup

	stopMu   sync.RWMutex
	stopped  atomic.Bool
	stopOnce sync.Once
}

// New creates and starts a new EventBus with the specified worker pool size and webhook configuration.
// If numWorkers is <= 0, a default pool of 4 workers is spawned.
func New(cfg config.EventsConfig, numWorkers int) Bus {
	if numWorkers <= 0 {
		numWorkers = 4
	}

	bus := &EventBus{
		webhooks:        cfg.Webhooks,
		workers:         make([]chan Event, numWorkers),
		circuitBreakers: make(map[string]time.Time),
	}

	for i := 0; i < numWorkers; i++ {
		bus.workers[i] = make(chan Event, 1000) // Buffer of 1000 events per worker
		bus.wg.Add(1)
		go bus.workerLoop(i, bus.workers[i])
	}

	return bus
}

// Publish enqueues a new event onto the worker corresponding to the hash of cameraID (or topic).
//
// The operation is non-blocking. If the targeted worker's channel buffer is full, the event
// is dropped and metrics.EventbusDropsTotal is incremented.
func (b *EventBus) Publish(topic string, cameraID string, data any) {
	if b.stopped.Load() || len(b.webhooks) == 0 {
		return
	}

	b.stopMu.RLock()
	defer b.stopMu.RUnlock()

	if b.stopped.Load() {
		return
	}

	event := Event{
		TimestampMs: time.Now().UnixMilli(),
		Topic:       topic,
		CameraID:    cameraID,
		Data:        data,
	}

	// Consistent hashing by CameraID (or Topic if CameraID is empty)
	// Guarantees strict FIFO processing order for any single camera
	key := event.CameraID
	if key == "" {
		key = event.Topic
	}

	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	workerID := int(h.Sum32() % uint32(len(b.workers))) // #nosec G115

	// Non-blocking send (Drop-Newest)
	select {
	case b.workers[workerID] <- event:
		// Successfully enqueued
	default:
		// Queue full, drop event to prevent producer blocking
		metrics.EventbusDropsTotal.Inc()
		log.Warn().Str("topic", event.Topic).Str("camera_id", event.CameraID).Int("worker_id", workerID).Msg("EventBus worker queue full, dropping event")
	}
}

// Stop gracefully shuts down all event bus workers.
// It closes all worker channels and blocks until all pending buffered events are processed.
// Subsequent calls to Stop or Publish are safe no-ops.
func (b *EventBus) Stop() {
	b.stopOnce.Do(func() {
		b.stopMu.Lock()
		b.stopped.Store(true)
		for _, ch := range b.workers {
			close(ch)
		}
		b.stopMu.Unlock()

		b.wg.Wait()
	})
}

func (b *EventBus) workerLoop(_ int, ch <-chan Event) {
	defer b.wg.Done()

	// Dedicated HTTP client with connection pooling and timeouts
	client := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 2,
			IdleConnTimeout:     30 * time.Second,
		},
	}

	for event := range ch {
		payload, err := json.Marshal(event)
		if err != nil {
			log.Error().Err(err).Msg("EventBus failed to marshal event")
			continue
		}

		for _, wh := range b.webhooks {
			if !matchesTopic(wh.Topics, event.Topic) {
				continue
			}

			// Circuit breaker check
			b.mu.RLock()
			unlockTime, isOpen := b.circuitBreakers[wh.URL]
			b.mu.RUnlock()

			if isOpen && time.Now().Before(unlockTime) {
				continue // Circuit is Open, skip attempt
			}

			err := sendWebhook(client, wh, payload)
			if err != nil {
				log.Warn().Err(err).Str("url", wh.URL).Msg("Webhook delivery failed, opening circuit for 30s")
				// Trip circuit breaker for 30 seconds
				b.mu.Lock()
				b.circuitBreakers[wh.URL] = time.Now().Add(30 * time.Second)
				b.mu.Unlock()
			} else if isOpen {
				// Reset circuit breaker on successful delivery
				b.mu.Lock()
				delete(b.circuitBreakers, wh.URL)
				b.mu.Unlock()
			}
		}
	}
}

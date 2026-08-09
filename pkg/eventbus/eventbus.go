package eventbus

import (
	"encoding/json"
	"hash/fnv"
	"net/http"
	"sync"
	"time"

	"github.com/RUSEGAL/ruseon-core/pkg/config"
	"github.com/rs/zerolog/log"
)

// Event представляет единицу события в системе
type Event struct {
	ID          string `json:"id"`
	TimestampMs int64  `json:"timestamp_ms"`
	Topic       string `json:"topic"`
	CameraID    string `json:"camera_id,omitempty"`
	Data        any    `json:"data,omitempty"`
}

type Bus interface {
	Publish(topic string, cameraID string, data any)
	Stop()
}

type EventBus struct {
	mu              sync.RWMutex
	webhooks        []config.WebhookConfig
	workers         []chan Event
	circuitBreakers map[string]time.Time // Общий Circuit Breaker для всех воркеров (URL -> время разблокировки)
	wg              sync.WaitGroup
	// quit канал убран в пользу закрытия каналов workers
}

// New создает новую шину событий и запускает воркеры.
func New(cfg config.EventsConfig, numWorkers int) Bus {
	if numWorkers <= 0 {
		numWorkers = 4 // Дефолтное количество воркеров
	}

	bus := &EventBus{
		webhooks:        cfg.Webhooks,
		workers:         make([]chan Event, numWorkers),
		circuitBreakers: make(map[string]time.Time),
	}

	for i := 0; i < numWorkers; i++ {
		bus.workers[i] = make(chan Event, 1000) // Буфер на 1000 событий на воркер
		bus.wg.Add(1)
		go bus.workerLoop(i, bus.workers[i])
	}

	return bus
}

func (b *EventBus) Publish(topic string, cameraID string, data any) {
	if len(b.webhooks) == 0 {
		return // Нет смысла рассылать, если нет подписчиков
	}

	// Защита от panic: send on closed channel во время Graceful Shutdown
	defer func() {
		_ = recover() // Канал уже закрыт (система останавливается), просто игнорируем событие
	}()

	event := Event{
		TimestampMs: time.Now().UnixMilli(),
		Topic:       topic,
		CameraID:    cameraID,
		Data:        data,
	}

	// Consistent hashing по CameraID (или Topic, если CameraID пустой)
	// Гарантирует сохранение порядка обработки для одной камеры
	key := event.CameraID
	if key == "" {
		key = event.Topic
	}

	h := fnv.New32a()
	h.Write([]byte(key))
	//nolint:gosec // len(b.workers) is always positive, safe to convert
	workerID := int(h.Sum32() % uint32(len(b.workers)))

	// Non-blocking send (Drop-Newest)
	select {
	case b.workers[workerID] <- event:
		// Успешно отправлено
	default:
		// Очередь воркера переполнена, дропаем событие
		log.Warn().Str("topic", event.Topic).Str("camera_id", event.CameraID).Int("worker_id", workerID).Msg("EventBus worker queue full, dropping event")
	}
}

func (b *EventBus) Stop() {
	// Закрываем каналы воркеров, чтобы они могли "доработать" оставшиеся события (Graceful Shutdown)
	for _, ch := range b.workers {
		close(ch)
	}
	b.wg.Wait()
}

func (b *EventBus) workerLoop(_ int, ch <-chan Event) {
	defer b.wg.Done()

	// Настраиваем жесткий HTTP клиент с таймаутом и лимитами
	client := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 2,
			IdleConnTimeout:     30 * time.Second,
		},
	}

	// Состояние Circuit Breaker'ов теперь общее для всех воркеров (b.circuitBreakers)
	// Доступ к нему будет контролироваться через b.mu

	// Читаем из канала пока он не будет закрыт (Graceful Shutdown)
	for event := range ch {
		payload, err := json.Marshal(event)
		if err != nil {
			log.Error().Err(err).Msg("EventBus failed to marshal event")
			continue
		}

		// Рассылка по всем вебхукам
		for _, wh := range b.webhooks {
			// Проверка топика
			if !matchesTopic(wh.Topics, event.Topic) {
				continue
			}

			// Проверка Circuit Breaker
			b.mu.RLock()
			unlockTime, isOpen := b.circuitBreakers[wh.URL]
			b.mu.RUnlock()

			if isOpen && time.Now().Before(unlockTime) {
				continue // Circuit is Open, пропускаем отправку
			}

			// Отправка (синхронно внутри воркера, чтобы сохранять порядок)
			err := sendWebhook(client, wh, payload)
			if err != nil {
				log.Warn().Err(err).Str("url", wh.URL).Msg("Webhook delivery failed, opening circuit for 30s")
				// Включаем Circuit Breaker на 30 секунд
				b.mu.Lock()
				b.circuitBreakers[wh.URL] = time.Now().Add(30 * time.Second)
				b.mu.Unlock()
			} else if isOpen {
				// Сбрасываем Circuit Breaker при успехе, если он был
				b.mu.Lock()
				delete(b.circuitBreakers, wh.URL)
				b.mu.Unlock()
			}
		}
	}
}

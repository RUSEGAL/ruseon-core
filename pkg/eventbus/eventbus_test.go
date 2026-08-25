package eventbus

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RUSEGAL/ruseon-core/v2/pkg/config"
)

func TestEventBus_DeliveryAndHMAC(t *testing.T) {
	var receivedPayloads [][]byte
	var receivedSignatures []string
	var mu sync.Mutex

	// Создаем тестовый HTTP-сервер
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		sig := r.Header.Get("X-Signature")
		
		mu.Lock()
		receivedPayloads = append(receivedPayloads, body)
		receivedSignatures = append(receivedSignatures, sig)
		mu.Unlock()
		
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	secret := "test-secret"
	cfg := config.EventsConfig{
		Webhooks: []config.WebhookConfig{
			{
				URL:    ts.URL,
				Topics: []string{"*"},
				Secret: secret,
			},
		},
	}

	bus := New(cfg, 2)
	
	bus.Publish("test_topic", "cam1", map[string]string{"foo": "bar"})
	bus.Stop() // Дожидаемся обработки (Graceful Shutdown)

	mu.Lock()
	defer mu.Unlock()

	if len(receivedPayloads) != 1 {
		t.Fatalf("Expected 1 webhook call, got %d", len(receivedPayloads))
	}

	if receivedSignatures[0] == "" {
		t.Errorf("Expected HMAC signature, got empty string")
	}

	var ev Event
	if err := json.Unmarshal(receivedPayloads[0], &ev); err != nil {
		t.Fatalf("Failed to parse received payload: %v", err)
	}

	if ev.Topic != "test_topic" || ev.CameraID != "cam1" {
		t.Errorf("Unexpected event data: %+v", ev)
	}
}

func TestEventBus_TopicFiltering(t *testing.T) {
	var calls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := config.EventsConfig{
		Webhooks: []config.WebhookConfig{
			{
				URL:    ts.URL,
				Topics: []string{"camera_offline"},
			},
		},
	}

	bus := New(cfg, 1)
	
	bus.Publish("camera_connected", "cam1", nil) // Должно быть проигнорировано
	bus.Publish("camera_offline", "cam1", nil)   // Должно быть отправлено
	bus.Stop()

	if calls.Load() != 1 {
		t.Errorf("Expected 1 call (filtered), got %d", calls.Load())
	}
}

func TestEventBus_CircuitBreaker(t *testing.T) {
	var calls atomic.Int32
	// Сервер всегда возвращает 500 ошибку
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	cfg := config.EventsConfig{
		Webhooks: []config.WebhookConfig{
			{
				URL:    ts.URL,
				Topics: []string{"*"},
			},
		},
	}

	bus := New(cfg, 1)
	
	// Первое событие вызовет ошибку HTTP 500 и включит Circuit Breaker
	bus.Publish("topic1", "cam1", nil)
	time.Sleep(100 * time.Millisecond) // Ждем отработки воркера

	// Второе событие должно быть дропнуто мгновенно, без HTTP запроса
	bus.Publish("topic2", "cam2", nil)
	time.Sleep(100 * time.Millisecond)

	bus.Stop()

	if calls.Load() != 1 {
		t.Errorf("Circuit breaker failed. Expected 1 call to failing server, got %d", calls.Load())
	}
}

func TestEventBus_DropNewest(t *testing.T) {
	// Создаем "зависший" сервер, который отвечает очень долго
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(1 * time.Second) // Воркер повиснет на 1 секунду
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := config.EventsConfig{
		Webhooks: []config.WebhookConfig{
			{URL: ts.URL, Topics: []string{"*"}},
		},
	}

	// 1 воркер. Буфер по дефолту 1000. 
	// Отправляем 1002 события. Одно возьмется в работу, 1000 заполнят буфер, последнее дропнется без блокировки
	bus := New(cfg, 1)

	publishStart := time.Now()
	for i := 0; i < 1002; i++ {
		bus.Publish("test_spam", "cam1", nil)
	}
	publishDuration := time.Since(publishStart)

	// Publish не должен был заблокироваться
	if publishDuration > 50*time.Millisecond {
		t.Errorf("Publish blocked for %v, should be non-blocking", publishDuration)
	}

	// Не ждем Stop(), иначе зависнем надолго (по 1с на каждый из 1000 элементов)
}

func TestEventBus_EdgeCases(_ *testing.T) {
	// Test New with <= 0 workers
	bus := New(config.EventsConfig{Webhooks: []config.WebhookConfig{{URL: "http://localhost", Topics: []string{"*"}}}}, 0)
	bus.Stop()

	// Test Publish with no webhooks
	busNoHooks := New(config.EventsConfig{}, 1)
	busNoHooks.Publish("test", "cam1", nil)
	busNoHooks.Stop()

	// Test json.Marshal error (passing a channel causes error)
	busInvalidData := New(config.EventsConfig{Webhooks: []config.WebhookConfig{{URL: "http://localhost", Topics: []string{"*"}}}}, 1)
	busInvalidData.Publish("test", "cam1", make(chan int))
	time.Sleep(10 * time.Millisecond)
	busInvalidData.Stop()
}


func TestEventBus_WebhookEdgeCases(_ *testing.T) {
	// Empty topics (allows all)
	busEmptyTopics := New(config.EventsConfig{Webhooks: []config.WebhookConfig{{URL: "http://127.0.0.1:1234", Topics: []string{}}}}, 1)
	busEmptyTopics.Publish("test", "cam1", nil)
	time.Sleep(10 * time.Millisecond)
	busEmptyTopics.Stop()

	// Invalid URL to trigger NewRequest error
	busInvalidURL := New(config.EventsConfig{Webhooks: []config.WebhookConfig{{URL: "://bad-url", Topics: []string{"*"}}}}, 1)
	busInvalidURL.Publish("test", "cam1", nil)
	time.Sleep(10 * time.Millisecond)
	busInvalidURL.Stop()
}

func TestEventBus_Stop_ConcurrentAndIdempotent(_ *testing.T) {
	cfg := config.EventsConfig{
		Webhooks: []config.WebhookConfig{
			{URL: "http://127.0.0.1:9999", Topics: []string{"*"}},
		},
	}
	bus := New(cfg, 4)

	var wg sync.WaitGroup

	// 10 concurrent publishers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				bus.Publish("topic", "cam", map[string]int{"id": id, "seq": j})
				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}

	// Concurrent Stop calls
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(10 * time.Millisecond)
			bus.Stop()
		}()
	}

	wg.Wait()

	// Post-stop publish should not panic
	bus.Publish("topic_post_stop", "cam", nil)
}



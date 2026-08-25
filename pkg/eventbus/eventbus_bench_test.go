package eventbus

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/RUSEGAL/ruseon-core/v2/pkg/config"
)

func BenchmarkEventBus_Publish(b *testing.B) {
	oldLogger := log.Logger
	log.Logger = zerolog.New(io.Discard)
	defer func() { log.Logger = oldLogger }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := config.EventsConfig{
		Webhooks: []config.WebhookConfig{
			{
				URL:    ts.URL,
				Topics: []string{"*"},
				Secret: "bench-secret",
			},
		},
	}

	bus := New(cfg, 8)
	defer bus.Stop()

	payload := map[string]string{"type": "motion", "zone": "entrance"}

	b.ReportAllocs()
	b.ResetTimer()
	var i int
	for b.Loop() {
		i++
		camID := fmt.Sprintf("cam_%d", i%100)
		bus.Publish("motion_detected", camID, payload)
	}
}

func BenchmarkEventBus_PublishParallel(b *testing.B) {
	oldLogger := log.Logger
	log.Logger = zerolog.New(io.Discard)
	defer func() { log.Logger = oldLogger }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := config.EventsConfig{
		Webhooks: []config.WebhookConfig{
			{
				URL:    ts.URL,
				Topics: []string{"*"},
				Secret: "bench-secret",
			},
		},
	}

	bus := New(cfg, 8)
	defer bus.Stop()

	payload := map[string]string{"type": "person", "score": "0.95"}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		var i int
		for pb.Next() {
			i++
			camID := fmt.Sprintf("cam_%d", i%600)
			bus.Publish("ai_analytics", camID, payload)
		}
	})
}

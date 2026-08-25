package stream

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/RUSEGAL/ruseon-core/v2/pkg/config"
	"github.com/RUSEGAL/ruseon-core/v2/pkg/registry"
)

// StartBillingTask launches a background ticker goroutine (1-minute interval) that accumulates
// camera bandwidth deltas in memory and commits aggregated traffic batches to the StateStore.
// On context shutdown, it ensures all pending traffic deltas are flushed to storage.
func StartBillingTask(ctx context.Context, _ *config.Config, manager *Manager, store registry.StateStore) {
	lastBytes := make(map[string]uint64)
	pendingDelta := make(map[string]uint64)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().Interface("panic", r).Msg("Recovered from panic in BillingWorker")
			}
		}()

		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				// При shutdown — сбрасываем накопленный трафик в базу
				collectBillingDelta(manager, lastBytes, pendingDelta)
				if len(pendingDelta) > 0 {
					_ = flushBilling(store, pendingDelta)
				}
				return
			case <-ticker.C:
				collectBillingDelta(manager, lastBytes, pendingDelta)
				if len(pendingDelta) > 0 {
					if err := flushBilling(store, pendingDelta); err != nil {
						log.Error().Err(err).Msg("BillingTask: failed to flush traffic batch to DB")
					} else {
						// Очищаем мапу после успешного сброса
						for k := range pendingDelta {
							delete(pendingDelta, k)
						}
					}
				}
			}
		}
	}()
}

// collectBillingDelta вычисляет дельту трафика для всех стримов в памяти (без обращения к диску)
func collectBillingDelta(manager *Manager, lastBytes, pendingDelta map[string]uint64) {
	activeIDs := make(map[string]struct{})
	for _, st := range manager.GetStreams() {
		activeIDs[st.ID] = struct{}{}
		current := st.GetStats().BytesReceived
		prev := lastBytes[st.ID]
		if current > prev {
			pendingDelta[st.ID] += current - prev
		} else if current < prev {
			// Поток был перезапущен, счетчик обнулился
			pendingDelta[st.ID] += current
		}
		lastBytes[st.ID] = current
	}

	// Очищаем удаленные камеры из lastBytes
	for id := range lastBytes {
		if _, ok := activeIDs[id]; !ok {
			delete(lastBytes, id)
		}
	}
}

// flushBilling сбрасывает накопленный трафик в хранилище за 1 пакетную транзакцию
func flushBilling(store registry.StateStore, pendingDelta map[string]uint64) error {
	nowMonth := time.Now().Format("2006-01")
	return store.BatchUpdateTraffic(pendingDelta, nowMonth)
}

package stream

import (
	"context"
	"time"
	"github.com/rs/zerolog/log"

	"github.com/RUSEGAL/REA-Stream-Engine/pkg/config"
	"github.com/RUSEGAL/REA-Stream-Engine/pkg/registry"
)

// StartBillingTask запускает фоновую задачу для трекинга трафика
func StartBillingTask(ctx context.Context, _ *config.Config, manager *Manager, store registry.StateStore) {
	
	// Храним последние значения байт для вычисления дельты
	lastBytes := make(map[string]uint64)

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
				return
			case <-ticker.C:
				processBilling(manager, store, lastBytes)
			}
		}
	}()
}

func processBilling(manager *Manager, store registry.StateStore, lastBytes map[string]uint64) {
	nowMonth := time.Now().Format("2006-01")

	cams, _ := store.ListCameras()
	for _, camMeta := range cams {
		_ = store.UpdateCameraTx(camMeta.ID, func(cam *config.CameraConfig) bool {
			changed := false
			
			// 1. Проверяем сброс трафика (1-е число месяца обрабатывается сменой месяца)
			if cam.LastResetMonth != nowMonth {
				cam.TrafficUsed = 0
				cam.LastResetMonth = nowMonth
				changed = true
			}

			// Дефолтный лимит 200 ГБ, если не задан
			if cam.TrafficLimit == 0 {
				cam.TrafficLimit = 200 * 1024 * 1024 * 1024 // 200 GB
				changed = true
			}

			// 2. Считаем дельту
			if st, ok := manager.GetStream(cam.ID); ok {
				currentBytes := st.GetStats().BytesReceived
				prev := lastBytes[cam.ID]
				
				if currentBytes > prev {
					delta := currentBytes - prev
					cam.TrafficUsed += delta
					changed = true
				} else if currentBytes < prev {
					// Поток был перезапущен, статистика обнулилась
					cam.TrafficUsed += currentBytes
					changed = true
				}
				lastBytes[cam.ID] = currentBytes
			}
			
			return changed
		})
	}
}

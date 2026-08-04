package stream

import (
	"time"

	"github.com/RUSEGAL/REA-Stream-Engine/internal/config"
	"github.com/RUSEGAL/REA-Stream-Engine/internal/storage"
)

// StartBillingTask запускает фоновую задачу для трекинга трафика
func StartBillingTask(cfg *config.Config, manager *Manager, store *storage.Storage) {
	ticker := time.NewTicker(1 * time.Minute)
	
	// Храним последние значения байт для вычисления дельты
	lastBytes := make(map[string]uint64)

	go func() {
		for range ticker.C {
			processBilling(manager, store, lastBytes)
		}
	}()
}

func processBilling(manager *Manager, store *storage.Storage, lastBytes map[string]uint64) {
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

package stream

import (
	"time"

	"gritprofmediaserver/internal/config"
)

// StartBillingTask запускает фоновую задачу для трекинга трафика
func StartBillingTask(cfg *config.Config, manager *Manager) {
	ticker := time.NewTicker(1 * time.Minute)
	
	// Храним последние значения байт для вычисления дельты
	lastBytes := make(map[string]uint64)

	go func() {
		for range ticker.C {
			changed := false
			nowMonth := time.Now().Format("2006-01")

			for i, camCfg := range cfg.Cameras {
				// 1. Проверяем сброс трафика (1-е число месяца обрабатывается сменой месяца)
				if camCfg.LastResetMonth != nowMonth {
					cfg.Cameras[i].TrafficUsed = 0
					cfg.Cameras[i].LastResetMonth = nowMonth
					changed = true
				}

				// Дефолтный лимит 200 ГБ, если не задан
				if cfg.Cameras[i].TrafficLimit == 0 {
					cfg.Cameras[i].TrafficLimit = 200 * 1024 * 1024 * 1024 // 200 GB
					changed = true
				}

				// 2. Считаем дельту
				if st, ok := manager.GetStream(camCfg.ID); ok {
					currentBytes := st.GetStats().BytesReceived
					prev := lastBytes[camCfg.ID]
					
					if currentBytes > prev {
						delta := currentBytes - prev
						cfg.Cameras[i].TrafficUsed += delta
						changed = true
					} else if currentBytes < prev {
						// Поток был перезапущен, статистика обнулилась
						cfg.Cameras[i].TrafficUsed += currentBytes
						changed = true
					}
					lastBytes[camCfg.ID] = currentBytes
				}
			}

			if changed {
				_ = cfg.Save("config.yaml")
			}
		}
	}()
}

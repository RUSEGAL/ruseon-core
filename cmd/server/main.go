package main

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"gritprofmediaserver/internal/api"
	"gritprofmediaserver/internal/config"
	"gritprofmediaserver/internal/logger"
	"gritprofmediaserver/internal/recorder"
	"gritprofmediaserver/internal/stream"
)

func main() {
	// 1. Загрузка конфигурации
	cfg, err := config.Load("config.yaml")
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		return
	}

	// 2. Инициализация логгера
	logger.Init(cfg.Server.Debug)
	log.Info().Msg("Starting GritprofMediaServer...")

	// 3. Восстановление архивов (Защита от сбоев питания)
	recorder.RecoverCrashedFiles("recordings")

	// 4. Инициализация StreamManager
	manager := stream.NewManager()
	
	// Запускаем фоновую очистку записей
	recorder.StartCleanupTask("recordings", cfg)
	
	// Запускаем трекинг трафика
	stream.StartBillingTask(cfg, manager)

	// Добавляем камеры из конфигурации
	for _, cam := range cfg.Cameras {
		if !cam.Disabled {
			manager.AddStream(cam.ID, cam.URL, cam.Record)
			log.Info().Str("id", cam.ID).Str("url", cam.URL).Bool("record", cam.Record).Msg("Added camera from config")
		} else {
			log.Info().Str("id", cam.ID).Msg("Camera is disabled, skipping stream creation")
		}
	}

	// 5. Инициализация HTTP сервера (Gin)
	handler := api.NewHandler(manager, cfg)
	router := api.SetupRouter(handler, cfg.Server.Debug)

	// 6. Запуск сервера
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Info().Str("addr", addr).Msg("Starting API server")
	
	if err := router.Run(addr); err != nil {
		log.Fatal().Err(err).Msg("Server failed")
	}
}

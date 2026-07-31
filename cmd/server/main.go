package main

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"gritprofmediaserver/internal/api"
	"gritprofmediaserver/internal/backup"
	"gritprofmediaserver/internal/config"
	"gritprofmediaserver/internal/logger"
	"gritprofmediaserver/internal/recorder"
	"gritprofmediaserver/internal/storage"
	"gritprofmediaserver/internal/stream"
	"context"
	"time"
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

	// Инициализация БД
	store, err := storage.NewStorage("data")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to open storage")
	}
	defer store.Close()

	// Миграция данных из config.yaml в БД (при первом запуске)
	if err := store.MigrateFromConfig(cfg); err != nil {
		log.Fatal().Err(err).Msg("Failed to migrate data from config")
	}

	// Очищаем камеры и теги из конфига, теперь они живут в BadgerDB
	if len(cfg.Cameras) > 0 || len(cfg.GlobalTags) > 0 {
		cfg.Cameras = nil
		cfg.GlobalTags = nil
		if err := cfg.Save("config.yaml"); err != nil {
			log.Error().Err(err).Msg("Failed to clean up config.yaml")
		} else {
			log.Info().Msg("Successfully cleaned dynamic data from config.yaml")
		}
	}

	// 4. Инициализация StreamManager
	manager := stream.NewManager()
	
	// Запускаем фоновую очистку записей
	recorder.StartCleanupTask("recordings", cfg, store)
	
	// Запускаем трекинг трафика
	stream.StartBillingTask(cfg, manager, store)

	// Запускаем фоновый бэкап базы данных (раз в 24 часа, храним 7 дней)
	backupWorker := backup.NewWorker(store, "data/backups", 24*time.Hour, 7)
	go backupWorker.Run(context.Background())

	// Добавляем камеры из БД
	cams, err := store.ListCameras()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load cameras from DB")
	}

	for _, cam := range cams {
		if !cam.Disabled {
			manager.AddStream(cam.ID, cam.URL, cam.Record)
			log.Info().Str("id", cam.ID).Str("url", cam.URL).Bool("record", cam.Record).Msg("Added camera from DB")
		} else {
			log.Info().Str("id", cam.ID).Msg("Camera is disabled, skipping stream creation")
		}
	}

	// 5. Инициализация HTTP сервера (Gin)
	handler := api.NewHandler(manager, cfg, store)
	router := api.SetupRouter(handler, cfg.Server.Debug)

	// 6. Запуск сервера
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Info().Str("addr", addr).Msg("Starting API server")
	
	if err := router.Run(addr); err != nil {
		log.Fatal().Err(err).Msg("Server failed")
	}
}

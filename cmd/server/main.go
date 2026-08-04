package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/RUSEGAL/REA-Stream-Engine/internal/api"
	"github.com/RUSEGAL/REA-Stream-Engine/internal/backup"
	"github.com/RUSEGAL/REA-Stream-Engine/internal/config"
	"github.com/RUSEGAL/REA-Stream-Engine/internal/logger"
	"github.com/RUSEGAL/REA-Stream-Engine/internal/recorder"
	"github.com/RUSEGAL/REA-Stream-Engine/internal/storage"
	"github.com/RUSEGAL/REA-Stream-Engine/internal/stream"
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
	log.Info().Msg("Starting REA Stream Engine...")

	// 2.1 Тюнинг Garbage Collector (Этап 23.1)
	if cfg.Server.GCPercent > 0 {
		debug.SetGCPercent(cfg.Server.GCPercent)
		log.Info().Int("gc_percent", cfg.Server.GCPercent).Msg("Applied GC tuning")
	}
	if cfg.Server.GCMemoryLimitMB > 0 {
		limitBytes := int64(cfg.Server.GCMemoryLimitMB) * 1024 * 1024
		debug.SetMemoryLimit(limitBytes)
		log.Info().Int("limit_mb", cfg.Server.GCMemoryLimitMB).Msg("Applied GC memory limit")
	}

	// 3. Восстановление архивов (Защита от сбоев питания)
	recorder.RecoverCrashedFiles("recordings")

	// Инициализация БД
	store, err := storage.NewStorage("data")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to open storage")
	}
	defer store.Close()

	ctx, cancelAll := context.WithCancel(context.Background())
	defer cancelAll()

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
	recorder.StartCleanupTask(ctx, "recordings", cfg, store)
	
	// Запускаем трекинг трафика
	stream.StartBillingTask(cfg, manager, store)

	// Запускаем фоновый бэкап базы данных (раз в 24 часа, храним 7 дней)
	backupWorker := backup.NewWorker(store, "data/backups", 24*time.Hour, 7)
	go backupWorker.Run(ctx)

	// Добавляем камеры из БД
	cams, err := store.ListCameras()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load cameras from DB")
	}

	for _, cam := range cams {
		if !cam.Disabled {
			_ = manager.AddStream(cam.ID, cam.URL, cam.Record, cam.LazyHLS, cam.Transport)
			log.Info().Str("id", cam.ID).Str("url", cam.URL).Bool("record", cam.Record).Msg("Added camera from DB")
		} else {
			log.Info().Str("id", cam.ID).Msg("Camera is disabled, skipping stream creation")
		}
	}

	// 5. Инициализация HTTP сервера (Gin)
	handler := api.NewHandler(manager, cfg, store)
	router := api.SetupRouter(handler, cfg.Server.Debug)

	// 6. Запуск сервера с Graceful Shutdown
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().Interface("panic", r).Msg("Recovered from panic in HTTP Server")
			}
		}()
		log.Info().Str("addr", addr).Msg("Starting API server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server failed")
		}
	}()

	// Ожидание сигнала для завершения (Graceful Shutdown)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("Shutting down server...")

	// Даем 5 секунд на корректное завершение текущих соединений
	ctxShutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctxShutdown); err != nil {
		log.Fatal().Err(err).Msg("Server forced to shutdown")
	}

	// Останавливаем потоки и менеджеры (опционально, если есть метод)
	for _, st := range manager.GetStreams() {
		manager.RemoveStream(st.ID)
	}

	// БД закроется через defer store.Close() в main, но мы дождемся его.
	log.Info().Msg("Server exiting")
}

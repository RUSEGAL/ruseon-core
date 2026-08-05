package main

import (
	"fmt"
	"runtime/debug"

	"github.com/rs/zerolog/log"

	"github.com/RUSEGAL/REA-Stream-Engine/pkg/auth"
	"github.com/RUSEGAL/REA-Stream-Engine/pkg/config"
	"github.com/RUSEGAL/REA-Stream-Engine/pkg/logger"
	"github.com/RUSEGAL/REA-Stream-Engine/internal/recorder"
	"github.com/RUSEGAL/REA-Stream-Engine/pkg/registry"
	"github.com/RUSEGAL/REA-Stream-Engine/pkg/storage"
	"github.com/RUSEGAL/REA-Stream-Engine/pkg/storage/localfs"
	"github.com/RUSEGAL/REA-Stream-Engine/pkg/engine"
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


	// Инициализация БД
	store, err := storage.NewStorage("data")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to open storage")
	}
	defer store.Close()

	registry.RegisterStateStore(store)

	localFS := localfs.NewLocalFS("")
	registry.RegisterBlobStore(localFS)

	authenticator := auth.NewLocalAuthenticator(cfg)
	registry.RegisterAuthenticator(authenticator)

	// Восстановление архивов (Защита от сбоев питания)
	recorder.RecoverCrashedFiles("recordings")

	engine.Run(cfg)
}

// Package main implements the entrypoint for the RUSEON Core media server daemon.
package main

import (
	"fmt"
	"runtime/debug"

	"github.com/rs/zerolog/log"

	"github.com/RUSEGAL/ruseon-core/v2/internal/recorder"
	"github.com/RUSEGAL/ruseon-core/v2/pkg/auth"
	"github.com/RUSEGAL/ruseon-core/v2/pkg/config"
	"github.com/RUSEGAL/ruseon-core/v2/pkg/engine"
	"github.com/RUSEGAL/ruseon-core/v2/pkg/eventbus"
	"github.com/RUSEGAL/ruseon-core/v2/pkg/logger"
	"github.com/RUSEGAL/ruseon-core/v2/pkg/registry"
	"github.com/RUSEGAL/ruseon-core/v2/pkg/storage"
	"github.com/RUSEGAL/ruseon-core/v2/pkg/storage/localfs"
)

// @title RUSEON Core API
// @version 1.0
// @description REST API for managing RUSEON video streaming server.
// @host localhost:8080
// @BasePath /

func main() {
	// 1. Загрузка конфигурации
	cfg, err := config.Load("config.yaml")
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		return
	}

	// 2. Инициализация логгера
	logger.Init(cfg.Server.Debug)
	log.Info().Msg("Starting RUSEON Core...")

	// 2.1 Fine-tuning Garbag
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

	bus := eventbus.New(cfg.Events, 4)
	registry.RegisterEventBus(bus)
	defer bus.Stop()

	// Восстановление архивов (Защита от сбоев питания)
	recorder.RecoverCrashedFiles("recordings")

	engine.Run(cfg)
}

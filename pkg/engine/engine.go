// Package engine orchestrates the lifecycle, subsystem initialization, background workers,
// network servers, and graceful shutdown of RUSEON Core.
package engine

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/RUSEGAL/ruseon-core/v2/internal/api"
	"github.com/RUSEGAL/ruseon-core/v2/internal/backup"
	"github.com/RUSEGAL/ruseon-core/v2/internal/grpc"
	"github.com/RUSEGAL/ruseon-core/v2/internal/mqtt"
	"github.com/RUSEGAL/ruseon-core/v2/internal/recorder"
	"github.com/RUSEGAL/ruseon-core/v2/internal/stream"
	"github.com/RUSEGAL/ruseon-core/v2/internal/webrtc"
	"github.com/RUSEGAL/ruseon-core/v2/pkg/config"
	"github.com/RUSEGAL/ruseon-core/v2/pkg/registry"
)

// Run initializes and executes all core streaming subsystems, background workers, and API servers.
//
// Prerequisites:
//   - Dependency injection providers (registry.CurrentStateStore, registry.CurrentBlobStore,
//     registry.CurrentAuthenticator, and registry.CurrentEventBus) must be registered prior to calling Run.
//
// Lifecycle flow:
//  1. Migrates initial YAML camera/tag definitions to the persistent StateStore (BadgerDB).
//  2. Initializes the stream.Manager and loads active camera streams.
//  3. Starts the optional MQTT metadata publisher worker.
//  4. Starts the gRPC frame extractor & AI metadata ingestion server.
//  5. Launches background periodic tasks (archive retention cleanup, billing metrics aggregation, database backup).
//  6. Initializes the WebRTC engine (pregenerating ECDSA certificates and UDP muxing).
//  7. Launches the primary HTTP/REST/HLS/WHEP API server (and optional pprof server).
//  8. Blocks until a termination signal (SIGINT or SIGTERM) is received.
//  9. Executes an ordered graceful shutdown (terminates background tasks, allows 5s for active HTTP requests,
//     stops gRPC server, and closes all camera ingest/playback streams).
func Run(cfg *config.Config) {
	ctx, cancelAll := context.WithCancel(context.Background())
	defer cancelAll()

	if err := registry.CurrentStateStore.MigrateFromConfig(cfg); err != nil {
		log.Fatal().Err(err).Msg("Failed to migrate data from config") //nolint:gocritic
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
	
	// Инициализация MQTT Publisher
	mqttPub, err := mqtt.NewPublisher(cfg.Events.MQTT)
	if err != nil {
		log.Error().Err(err).Msg("Failed to start MQTT Publisher")
	} else if mqttPub != nil {
		defer mqttPub.Close()
	}

	// Запуск gRPC Frame Extractor API
	grpcServer := grpc.NewServer(manager, mqttPub, cfg.Server.TLS.CertFile, cfg.Server.TLS.KeyFile)
	go func() {
		addr := fmt.Sprintf("%s:%d", cfg.Server.GRPC.Address, cfg.Server.GRPC.Port)
		if err := grpcServer.Start(addr); err != nil {
			log.Error().Err(err).Msg("gRPC server failed")
		}
	}()
	
	// Запускаем фоновую очистку записей
	recorder.StartCleanupTask(ctx, "recordings", cfg, registry.CurrentStateStore)
	
	// Запускаем трекинг трафика
	stream.StartBillingTask(ctx, cfg, manager, registry.CurrentStateStore)

	// Запускаем фоновый бэкап базы данных (раз в 24 часа, храним 7 дней)
	backupWorker := backup.NewWorker(registry.CurrentStateStore, "data/backups", 24*time.Hour, 7)
	go backupWorker.Run(ctx)

	// Добавляем камеры из БД
	cams, err := registry.CurrentStateStore.ListCameras()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load cameras from DB")
	}

	for _, cam := range cams {
		if !cam.Disabled {
			_ = manager.AddStream(cam.ID, cam.URL, cam.Record, cam.LazyHLS, cam.Transport, cam.TokenAuth)
			log.Info().Str("id", cam.ID).Str("url", cam.URL).Bool("record", cam.Record).Msg("Added camera from DB")
		} else {
			log.Info().Str("id", cam.ID).Msg("Camera is disabled, skipping stream creation")
		}
	}

	// 5. Инициализация WebRTC Engine и HTTP сервера (Gin)
	webrtcEngine, err := webrtc.NewEngine(cfg)
	if err != nil {
		log.Error().Err(err).Msg("Failed to initialize WebRTC engine")
	} else if webrtcEngine != nil {
		defer webrtcEngine.Close()
	}

	handler := api.NewHandler(manager, cfg, registry.CurrentStateStore, webrtcEngine)
	router := api.SetupRouter(handler, registry.CurrentAuthenticator, cfg.Server.Debug, cfg.Server.CORSAllowedOrigins)

	// 6. Запуск сервера с Graceful Shutdown
	// WriteTimeout установлен в 0 (отключен глобальный жесткий таймаут), чтобы поддерживать
	// долгоживущие SSE-потоки логов (/api/logs/stream) и выгрузку больших fMP4 архивов без обрыва сокета.
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      0,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().Interface("panic", r).Msg("Recovered from panic in HTTP Server")
			}
		}()
		log.Info().Str("addr", addr).Msg("Starting API server")
		var err error
		if cfg.Server.TLS.CertFile != "" && cfg.Server.TLS.KeyFile != "" {
			err = srv.ListenAndServeTLS(cfg.Server.TLS.CertFile, cfg.Server.TLS.KeyFile)
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server failed")
		}
	}()

	// 7. Запуск pprof сервера (если включен)
	if cfg.Server.PprofPort > 0 {
		pprofAddr := fmt.Sprintf("localhost:%d", cfg.Server.PprofPort)
		go func() {
			mux := http.NewServeMux()
			mux.HandleFunc("/debug/pprof/", pprof.Index)
			mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
			mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
			mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
			mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

			pprofSrv := &http.Server{
				Addr:              pprofAddr,
				Handler:           mux,
				ReadHeaderTimeout: 10 * time.Second,
				WriteTimeout:      30 * time.Second,
				IdleTimeout:       60 * time.Second,
			}

			log.Info().Str("addr", pprofAddr).Msg("Starting pprof profiling server")
			if err := pprofSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Error().Err(err).Msg("pprof server failed")
			}
		}()
	}

	// Ожидание сигнала для завершения (Graceful Shutdown)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("Shutting down server...")

	// Останавливаем фоновые контекстные задачи
	cancelAll()

	// Даем 5 секунд на корректное завершение текущих HTTP соединений
	ctxShutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctxShutdown); err != nil {
		log.Fatal().Err(err).Msg("Server forced to shutdown")
	}

	grpcServer.Stop()

	// Останавливаем все медиа-потоки
	manager.Close()

	log.Info().Msg("Server exiting")
}

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/astre-ash/omnigo/internal/config"
	"github.com/astre-ash/omnigo/internal/handler"
	"github.com/astre-ash/omnigo/internal/server"
	"github.com/astre-ash/omnigo/internal/service"
	"github.com/astre-ash/omnigo/internal/storage"
	"github.com/astre-ash/omnigo/internal/storage/postgres"
	"go.uber.org/zap"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		log.Fatalf("application terminated with error: %v", err)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.LoadServerConfig(os.Args[1:])
	if err != nil {
		return fmt.Errorf("failed to load server configuration: %w", err)
	}

	logger, err := zap.NewProduction()
	if err != nil {
		return fmt.Errorf("failed to initialize zap logger: %w", err)
	}
	defer func() { _ = logger.Sync() }()

	store, pinger, cleanup, err := initStorage(ctx, cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer cleanup()

	metricService := service.NewMetricService(store)
	metricHandler := handler.NewMetricHandler(metricService, logger)
	pingHandler := handler.NewPingHandler(pinger, logger)

	srv := server.NewServer(cfg.Address, metricHandler, pingHandler, logger)

	logger.Info("starting HTTP server", zap.String("addr", cfg.Address))
	return srv.Run(ctx)
}

func initStorage(
	ctx context.Context,
	cfg *config.ServerConfig,
	logger *zap.Logger,
) (store service.Storage, pinger handler.Pinger, closeFunc func(), err error) {

	if cfg.DatabaseDSN != "" {
		logger.Info("using PostgreSQL as metrics storage", zap.String("dsn", cfg.DatabaseDSN))
		pgStore, err := postgres.New(ctx, cfg.DatabaseDSN, logger)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to init postgres: %w", err)
		}

		retryingStore := postgres.NewRetryingStorage(pgStore, logger)

		return retryingStore, retryingStore, retryingStore.Close, nil
	}

	memStore := storage.NewMemStorage()

	if cfg.FileStoragePath != "" {
		logger.Info("using File Storage as persistent metrics store",
			zap.String("path", cfg.FileStoragePath),
			zap.Duration("interval", cfg.StoreInterval),
		)

		persister := storage.NewFileManager(cfg.FileStoragePath)
		pm, persistenceStore, err := storage.NewPersistenceManager(
			ctx,
			memStore,
			persister,
			cfg.StoreInterval,
			cfg.Restore,
			logger,
		)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to init persistence manager: %w", err)
		}

		pm.StartAsyncFlush(ctx)

		closeFunc = func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := pm.Stop(shutdownCtx); err != nil {
				logger.Error("failed to stop persistence manager and flush to file", zap.Error(err))
			}
		}

		return persistenceStore, nil, closeFunc, nil
	}

	logger.Info("using in-memory storage only (no persistence)")
	return memStore, nil, func() {}, nil
}

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
	"github.com/astre-ash/omnigo/internal/middleware"
	"github.com/astre-ash/omnigo/internal/server"
	"github.com/astre-ash/omnigo/internal/service"
	"github.com/astre-ash/omnigo/internal/storage"
	"github.com/astre-ash/omnigo/internal/storage/postgres"
	"github.com/astre-ash/omnigo/pkg/signature"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

const fileFlushShutdownTimeout = 5 * time.Second

func main() {
	os.Exit(execute())
}

func execute() int {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		stop()
	}()

	if err := run(ctx, os.Args[1:]); err != nil {
		log.Printf("application terminated with error: %v", err)
		return 1
	}

	return 0
}

func run(ctx context.Context, args []string) error {
	cfg, err := config.LoadServerConfig(args)
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
	if cleanup != nil {
		defer cleanup()
	}

	metricService := service.NewMetricService(store)
	metricHandler := handler.NewMetricHandler(metricService, logger)
	pingHandler := handler.NewPingHandler(pinger, logger)

	var signer middleware.SignVerifier
	if cfg.Key != "" {
		signer = signature.NewHMACSHA256(cfg.Key)
	}
	logger.Info("signature middleware configured", zap.Bool("enabled", signer != nil))

	srv := server.NewServer(cfg.Address, metricHandler, pingHandler, signer, logger)

	logger.Info("starting HTTP server", zap.String("addr", cfg.Address))
	return srv.Run(ctx)
}

func initStorage(
	ctx context.Context,
	cfg *config.ServerConfig,
	logger *zap.Logger,
) (service.Storage, handler.Pinger, func(), error) {
	if cfg.DatabaseDSN != "" {
		if connCfg, err := pgx.ParseConfig(cfg.DatabaseDSN); err == nil {
			logger.Info("using PostgreSQL as metrics storage",
				zap.String("host", connCfg.Host),
				zap.Uint16("port", connCfg.Port),
				zap.String("database", connCfg.Database),
			)
		}

		pgStore, err := postgres.New(ctx, cfg.DatabaseDSN, logger)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to init postgres: %w", err)
		}

		retryingStore := postgres.NewRetryingStorage(pgStore, logger)

		return retryingStore, pgStore, retryingStore.Close, nil
	}

	memStore := storage.NewMemStorage()
	noopPing := handler.NoopPinger{}

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

		closeFunc := func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), fileFlushShutdownTimeout)
			defer cancel()

			if err := pm.Stop(shutdownCtx); err != nil {
				logger.Error("failed to stop persistence manager and flush to file", zap.Error(err))
			}
		}

		return persistenceStore, noopPing, closeFunc, nil
	}

	logger.Info("using in-memory storage only (no persistence)")
	return memStore, noopPing, func() {}, nil
}

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/astre-ash/omnigo/internal/config"
	"github.com/astre-ash/omnigo/internal/handler"
	"github.com/astre-ash/omnigo/internal/server"
	"github.com/astre-ash/omnigo/internal/service"
	"github.com/astre-ash/omnigo/internal/storage"
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
	defer func() {
		_ = logger.Sync()
	}()

	memStore := storage.NewMemStorage()

	var persister storage.Persister
	if cfg.FileStoragePath != "" {
		persister = storage.NewFileManager(cfg.FileStoragePath)
	}

	pm, store, err := storage.NewPersistenceManager(
		memStore,
		persister,
		cfg.StoreInterval,
		cfg.Restore,
		logger,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize persistence: %w", err)
	}

	defer func() {
		if err := pm.Flush(); err != nil {
			logger.Error("failed to dump metrics on shutdown", zap.Error(err))
		} else if cfg.FileStoragePath != "" {
			logger.Info("metrics saved successfully on shutdown")
		}
	}()

	pm.StartAsyncFlush(ctx)

	metricService := service.NewMetricService(store)
	metricHandler := handler.NewMetricHandler(metricService, logger)
	srv := server.NewServer(cfg.Address, metricHandler, logger)

	logger.Info("server starting",
		zap.String("addr", cfg.Address),
		zap.Int("store_interval", cfg.StoreInterval),
		zap.String("file_storage_path", cfg.FileStoragePath),
		zap.Bool("restore", cfg.Restore),
	)

	if err := srv.Run(ctx); err != nil {
		return fmt.Errorf("http server runtime error: %w", err)
	}

	logger.Info("server exited cleanly")
	return nil
}

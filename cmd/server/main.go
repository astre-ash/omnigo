package main

import (
	"log"
	"os"

	"github.com/astre-ash/omnigo/internal/config"
	"github.com/astre-ash/omnigo/internal/handler"
	"github.com/astre-ash/omnigo/internal/server"
	"github.com/astre-ash/omnigo/internal/service"
	"github.com/astre-ash/omnigo/internal/storage"
	"go.uber.org/zap"
)

func main() {
	cfg, err := config.LoadServerConfig(os.Args[1:])
	if err != nil {
		log.Fatalf("failed to load server configuration: %v", err)
	}

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to initialize zap logger: %v", err)
	}

	defer func() {
		_ = logger.Sync()
	}()

	store := storage.NewMemStorage()
	metricService := service.NewMetricService(store)
	metricHandler := handler.NewMetricHandler(metricService, logger)

	srv := server.NewServer(cfg.Address, metricHandler, logger)

	if err := srv.Run(); err != nil {
		logger.Fatal("server terminated with error", zap.Error(err))
	}
}

package main

import (
	"log"

	"github.com/astre-ash/omnigo/internal/config"
	"github.com/astre-ash/omnigo/internal/handler"
	"github.com/astre-ash/omnigo/internal/server"
	"github.com/astre-ash/omnigo/internal/service"
	"github.com/astre-ash/omnigo/internal/storage"
)

func main() {
	cfg := config.LoadServerConfig()

	store := storage.NewMemStorage()
	metricService := service.NewMetricService(store)
	metricHandler := handler.NewMetricHandler(metricService)

	srv := server.NewServer(cfg.Address, metricHandler)

	if err := srv.Run(); err != nil {
		log.Fatalf("failed to run server: %v\n", err)
	}
}

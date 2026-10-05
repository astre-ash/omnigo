package main

import (
	"log"

	"github.com/astre-ash/omnigo/internal/config"
	"github.com/astre-ash/omnigo/internal/server"
	"github.com/astre-ash/omnigo/internal/storage"
)

func main() {
	cfg := config.LoadServerConfig()

	memStorage := storage.NewMemStorage()

	srv := server.NewServer(cfg.Address, memStorage)

	if err := srv.Run(); err != nil {
		log.Fatalf("failed to run server: %v\n", err)
	}
}

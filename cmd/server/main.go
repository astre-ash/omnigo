package main

import (
	"log"

	"github.com/astre-ash/omnigo/internal/server"
	"github.com/astre-ash/omnigo/internal/storage"
)

const defaultAddr = "127.0.0.1:8080"

func main() {

	memStorage := storage.NewMemStorage()

	srv := server.NewServer(defaultAddr, memStorage)

	if err := srv.Run(); err != nil {
		log.Fatalf("failed to run server: %v\n", err)
	}
}

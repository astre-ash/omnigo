package main

import (
	"log"
	"net/http"

	"github.com/astre-ash/omnigo/internal/handler"
	"github.com/astre-ash/omnigo/internal/storage"
)

func main() {

	memStorage := storage.NewMemStorage()
	updateHandler := handler.NewUpdateHandler(memStorage)

	mux := http.NewServeMux()
	mux.Handle("/update/", updateHandler)

	const addr = "localhost:8080"
	log.Printf("server is starting on %s\n", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("failed to start server: %v\n", err)
	}

}

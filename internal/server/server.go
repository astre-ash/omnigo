package server

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/astre-ash/omnigo/internal/domain"
	"github.com/astre-ash/omnigo/internal/handler"
)

type Server struct {
	addr       string
	httpServer *http.Server
	storage    domain.MetricStorage
}

func NewServer(addr string, storage domain.MetricStorage) *Server {
	s := &Server{
		addr:    addr,
		storage: storage,
	}

	router := s.setupRoutes()

	s.httpServer = &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	return s
}

func (s *Server) setupRoutes() http.Handler {
	mux := http.NewServeMux()

	updateHandler := handler.NewUpdateHandler(s.storage)
	mux.Handle("/update/", updateHandler)

	return mux
}

func (s *Server) Run() error {
	log.Printf("server is starting on %s\n", s.addr)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http server failed: %w", err)
	}

	return nil
}

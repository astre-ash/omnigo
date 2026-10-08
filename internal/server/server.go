package server

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/astre-ash/omnigo/internal/handler"
)

type Server struct {
	addr       string
	httpServer *http.Server
	handler    *handler.MetricHandler
}

func NewServer(addr string, h *handler.MetricHandler) *Server {
	s := &Server{
		addr:    addr,
		handler: h,
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
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", s.handler.GetAll)

	r.Route("/update", func(r chi.Router) {
		r.Post("/{type}/{name}/{value}", s.handler.Update)
	})

	r.Route("/value", func(r chi.Router) {
		r.Get("/{type}/{name}", s.handler.GetValue)
	})

	return r
}

func (s *Server) Run() error {
	log.Printf("server is starting on %s\n", s.addr)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http server failed: %w", err)
	}

	return nil
}

package server

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	appMiddleware "github.com/astre-ash/omnigo/internal/middleware"
)

type MetricHandler interface {
	Update(w http.ResponseWriter, r *http.Request)
	GetValue(w http.ResponseWriter, r *http.Request)
	GetAll(w http.ResponseWriter, r *http.Request)
	GetValueJSON(w http.ResponseWriter, r *http.Request)
	UpdateJSON(w http.ResponseWriter, r *http.Request)
}

type Server struct {
	addr       string
	httpServer *http.Server
	handler    MetricHandler
	logger     *zap.Logger
}

func NewServer(addr string, h MetricHandler, l *zap.Logger) *Server {
	s := &Server{
		addr:    addr,
		handler: h,
		logger:  l,
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

	r.Use(appMiddleware.RequestLogger(s.logger))
	r.Use(appMiddleware.Compress())
	r.Use(chiMiddleware.Recoverer)

	r.Get("/", s.handler.GetAll)

	r.Route("/update", func(r chi.Router) {
		r.Post("/{type}/{name}/{value}", s.handler.Update)
		r.Post("/", s.handler.UpdateJSON)
	})

	r.Route("/value", func(r chi.Router) {
		r.Post("/", s.handler.GetValueJSON)
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

package server

import (
	"context"
	"errors"
	"fmt"
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

func (s *Server) Run(ctx context.Context) error {
	s.logger.Info("starting HTTP server", zap.String("addr", s.addr))

	errCh := make(chan error, 1)

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("http server failed: %w", err)
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err

	case <-ctx.Done():
		s.logger.Info("shutting down HTTP server gracefully...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		defer cancel()

		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("failed to shutdown HTTP server: %w", err)
		}

		return nil
	}
}

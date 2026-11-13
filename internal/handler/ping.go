package handler

import (
	"context"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

type PingHandler struct {
	pinger Pinger
	logger *zap.Logger
}

func NewPingHandler(p Pinger, l *zap.Logger) *PingHandler {
	return &PingHandler{
		pinger: p,
		logger: l,
	}
}

func (h *PingHandler) Ping(w http.ResponseWriter, r *http.Request) {
	if h.pinger == nil {
		h.logger.Warn("ping check failed: database is not configured")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.pinger.Ping(ctx); err != nil {
		h.logger.Error("database ping error", zap.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

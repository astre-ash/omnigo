package handler

import (
	"context"
	"net/http"
	"time"

	"go.uber.org/zap"
)

const defaultPingTimeout = 2 * time.Second

type Pinger interface {
	Ping(ctx context.Context) error
}

type NoopPinger struct{}

func (NoopPinger) Ping(context.Context) error {
	return nil
}

type PingHandler struct {
	pinger  Pinger
	logger  *zap.Logger
	timeout time.Duration
}

type PingOption func(*PingHandler)

func WithPingTimeout(d time.Duration) PingOption {
	return func(h *PingHandler) {
		if d > 0 {
			h.timeout = d
		}
	}
}

func NewPingHandler(pinger Pinger, logger *zap.Logger, opts ...PingOption) *PingHandler {
	if logger == nil {
		logger = zap.NewNop()
	}

	h := &PingHandler{
		pinger:  pinger,
		logger:  logger,
		timeout: defaultPingTimeout,
	}

	for _, opt := range opts {
		if opt != nil {
			opt(h)
		}
	}

	return h
}

func (h *PingHandler) Ping(w http.ResponseWriter, r *http.Request) {
	if h.pinger == nil {
		h.log().Warn("ping check failed: database is not configured")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	if err := h.pinger.Ping(ctx); err != nil {
		h.log().Error("database ping error", zap.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *PingHandler) log() *zap.Logger {
	if h.logger == nil {
		return zap.NewNop()
	}
	return h.logger
}

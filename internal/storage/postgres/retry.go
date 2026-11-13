package postgres

import (
	"context"
	"errors"
	"io"
	"net"
	"syscall"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"

	"github.com/astre-ash/omnigo/internal/domain"
	"github.com/astre-ash/omnigo/pkg/retry"
)

type RetryingStorage struct {
	base   *Storage
	delays []time.Duration
	logger *zap.Logger
}

func NewRetryingStorage(base *Storage, logger *zap.Logger) *RetryingStorage {
	return &RetryingStorage{
		base:   base,
		delays: retry.DefaultDelays(),
		logger: logger,
	}
}

func (r *RetryingStorage) Close() {
	r.base.Close()
}

func (r *RetryingStorage) Ping(ctx context.Context) error {
	return retry.Do(ctx, r.delays, r.isRetryable, func() error {
		return r.base.Ping(ctx)
	})
}

func (r *RetryingStorage) UpdateGauge(ctx context.Context, name string, value float64) error {
	return retry.Do(ctx, r.delays, r.isRetryable, func() error {
		return r.base.UpdateGauge(ctx, name, value)
	})
}

func (r *RetryingStorage) UpdateCounter(ctx context.Context, name string, delta int64) error {
	return retry.Do(ctx, r.delays, r.isRetryable, func() error {
		return r.base.UpdateCounter(ctx, name, delta)
	})
}

func (r *RetryingStorage) GetGauge(ctx context.Context, name string) (float64, error) {
	var val float64
	err := retry.Do(ctx, r.delays, r.isRetryable, func() error {
		var err error
		val, err = r.base.GetGauge(ctx, name)
		return err
	})
	return val, err
}

func (r *RetryingStorage) GetCounter(ctx context.Context, name string) (int64, error) {
	var delta int64
	err := retry.Do(ctx, r.delays, r.isRetryable, func() error {
		var err error
		delta, err = r.base.GetCounter(ctx, name)
		return err
	})
	return delta, err
}

func (r *RetryingStorage) GetAll(ctx context.Context) (map[string]float64, map[string]int64, error) {
	var gauges map[string]float64
	var counters map[string]int64

	err := retry.Do(ctx, r.delays, r.isRetryable, func() error {
		var err error
		gauges, counters, err = r.base.GetAll(ctx)
		return err
	})
	return gauges, counters, err
}

func (r *RetryingStorage) UpdateBatch(ctx context.Context, metrics []domain.Metrics) error {
	return retry.Do(ctx, r.delays, r.isRetryable, func() error {
		return r.base.UpdateBatch(ctx, metrics)
	})
}

func (r *RetryingStorage) isRetryable(err error) bool {
	if err == nil {
		return false
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgerrcode.IsConnectionException(pgErr.Code) {
			r.logger.Warn("retrying postgres query after connection exception",
				zap.String("code", pgErr.Code),
				zap.String("message", pgErr.Message),
			)
			return true
		}
		return false
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		r.logger.Warn("retrying postgres query after network error", zap.Error(err))
		return true
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		r.logger.Warn("retrying postgres query after net.OpError", zap.Error(err))
		return true
	}

	if errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, io.EOF) {
		r.logger.Warn("retrying postgres query after socket error", zap.Error(err))
		return true
	}

	return false
}

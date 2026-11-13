package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/astre-ash/omnigo/internal/domain"
)

const (
	upsertGaugeQuery = `
		INSERT INTO metrics (id, type, value)
		VALUES (@id, @type, @value)
		ON CONFLICT (id, type)
		DO UPDATE SET value = EXCLUDED.value;
	`
	upsertCounterQuery = `
		INSERT INTO metrics (id, type, delta)
		VALUES (@id, @type, @delta)
		ON CONFLICT (id, type)
		DO UPDATE SET delta = COALESCE(metrics.delta, 0) + EXCLUDED.delta;
	`
	selectGaugeQuery   = `SELECT value FROM metrics WHERE id = @name AND type = @type;`
	selectCounterQuery = `SELECT delta FROM metrics WHERE id = @name AND type = @type;`
	selectAllQuery     = `SELECT id, type, delta, value FROM metrics;`

	rollbackTimeout = 5 * time.Second
)

type DBPool interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
	Ping(ctx context.Context) error
	Close()
}

var _ DBPool = (*pgxpool.Pool)(nil)

type Storage struct {
	pool DBPool
}

func New(ctx context.Context, dsn string, logger *zap.Logger) (*Storage, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	migrationDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open migration db connection: %w", err)
	}
	defer func() {
		_ = migrationDB.Close()
	}()

	if err := runMigrations(migrationDB); err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	logger.Info("database migrations applied successfully")

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	return NewWithPool(pool), nil
}

func NewWithPool(pool DBPool) *Storage {
	return &Storage{
		pool: pool,
	}
}

func (s *Storage) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

func (s *Storage) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *Storage) UpdateGauge(ctx context.Context, name string, value float64) error {
	_, err := s.pool.Exec(ctx, upsertGaugeQuery, pgx.NamedArgs{
		"id":    name,
		"type":  domain.TypeGauge,
		"value": value,
	})
	if err != nil {
		return fmt.Errorf("failed to update gauge %s: %w", name, err)
	}

	return nil
}

func (s *Storage) UpdateCounter(ctx context.Context, name string, delta int64) error {
	_, err := s.pool.Exec(ctx, upsertCounterQuery, pgx.NamedArgs{
		"id":    name,
		"type":  domain.TypeCounter,
		"delta": delta,
	})
	if err != nil {
		return fmt.Errorf("failed to update counter %s: %w", name, err)
	}

	return nil
}

func (s *Storage) GetGauge(ctx context.Context, name string) (float64, error) {
	var val *float64

	err := s.pool.QueryRow(ctx, selectGaugeQuery, pgx.NamedArgs{
		"name": name,
		"type": domain.TypeGauge,
	}).Scan(&val)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, domain.ErrMetricNotFound
		}
		return 0, fmt.Errorf("failed to get gauge %s: %w", name, err)
	}

	if val == nil {
		return 0, domain.ErrMetricNotFound
	}

	return *val, nil
}

func (s *Storage) GetCounter(ctx context.Context, name string) (int64, error) {
	var delta *int64

	err := s.pool.QueryRow(ctx, selectCounterQuery, pgx.NamedArgs{
		"name": name,
		"type": domain.TypeCounter,
	}).Scan(&delta)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, domain.ErrMetricNotFound
		}
		return 0, fmt.Errorf("failed to get counter %s: %w", name, err)
	}

	if delta == nil {
		return 0, domain.ErrMetricNotFound
	}

	return *delta, nil
}

func (s *Storage) GetAll(ctx context.Context) (map[string]float64, map[string]int64, error) {
	rows, err := s.pool.Query(ctx, selectAllQuery)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query metrics: %w", err)
	}
	defer rows.Close()

	gauges := make(map[string]float64, 32)
	counters := make(map[string]int64, 32)

	for rows.Next() {
		var id, mtype string
		var delta *int64
		var value *float64

		if err := rows.Scan(&id, &mtype, &delta, &value); err != nil {
			return nil, nil, fmt.Errorf("failed to scan metric row: %w", err)
		}

		switch mtype {
		case domain.TypeGauge:
			if value != nil {
				gauges[id] = *value
			}
		case domain.TypeCounter:
			if delta != nil {
				counters[id] = *delta
			}
		}
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("rows iteration failed: %w", err)
	}

	return gauges, counters, nil
}

func (s *Storage) UpdateBatch(ctx context.Context, metrics []domain.Metrics) error {
	if len(metrics) == 0 {
		return nil
	}

	batch, err := buildMetricsBatch(metrics)
	if err != nil {
		return err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	var committed bool
	defer func() {
		if !committed {
			rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), rollbackTimeout)
			defer cancel()
			_ = tx.Rollback(rollbackCtx)
		}
	}()

	br := tx.SendBatch(ctx, batch)

	for i := 0; i < batch.Len(); i++ {
		if _, err := br.Exec(); err != nil {
			_ = br.Close()
			return fmt.Errorf("failed to execute batch statement: %w", err)
		}
	}

	if err := br.Close(); err != nil {
		return fmt.Errorf("failed to close batch results: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	committed = true
	return nil
}

func buildMetricsBatch(metrics []domain.Metrics) (*pgx.Batch, error) {
	batch := &pgx.Batch{}

	for _, m := range metrics {
		switch m.MType {
		case domain.TypeGauge:
			if m.Value == nil {
				return nil, fmt.Errorf("%w: gauge metric %q has nil value", domain.ErrInvalidValue, m.ID)
			}
			batch.Queue(upsertGaugeQuery, pgx.NamedArgs{
				"id":    m.ID,
				"type":  m.MType,
				"value": *m.Value,
			})

		case domain.TypeCounter:
			if m.Delta == nil {
				return nil, fmt.Errorf("%w: counter metric %q has nil delta", domain.ErrInvalidValue, m.ID)
			}
			batch.Queue(upsertCounterQuery, pgx.NamedArgs{
				"id":    m.ID,
				"type":  m.MType,
				"delta": *m.Delta,
			})

		default:
			return nil, fmt.Errorf("%w: %q", domain.ErrUnknownMetricType, m.MType)
		}
	}

	return batch, nil
}

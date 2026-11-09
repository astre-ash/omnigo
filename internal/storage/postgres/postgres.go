package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/astre-ash/omnigo/internal/domain"
)

type Storage struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, dsn string, logger *zap.Logger) (*Storage, error) {
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

	return &Storage{
		pool: pool,
	}, nil
}

func (s *Storage) Close() {
	s.pool.Close()
}

func (s *Storage) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *Storage) UpdateGauge(ctx context.Context, name string, value float64) error {
	query := `
    INSERT INTO metrics (id, type, value)
    VALUES (@id, @type, @value)
    ON CONFLICT (id, type) 
    DO UPDATE SET value = EXCLUDED.value;
`

	_, err := s.pool.Exec(ctx, query, pgx.NamedArgs{
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
	query := `
		INSERT INTO metrics (id, type, delta)
		VALUES (@id, @type, @delta)
		ON CONFLICT (id, type) 
		DO UPDATE SET delta = metrics.delta + EXCLUDED.delta;
	`
	_, err := s.pool.Exec(ctx, query, pgx.NamedArgs{
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
	query := `SELECT value FROM metrics WHERE id = @name AND type = @type;`

	var val *float64

	err := s.pool.QueryRow(ctx, query, pgx.NamedArgs{
		"name": name,
		"type": domain.TypeGauge,
	}).Scan(&val)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, domain.ErrMetricNotFound
		}
		return 0, err
	}

	if val == nil {
		return 0, domain.ErrMetricNotFound
	}

	return *val, nil
}

func (s *Storage) GetCounter(ctx context.Context, name string) (int64, error) {
	query := `SELECT delta FROM metrics WHERE id = @name AND type = @type;`

	var delta *int64

	err := s.pool.QueryRow(ctx, query, pgx.NamedArgs{
		"name": name,
		"type": domain.TypeCounter,
	}).Scan(&delta)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, domain.ErrMetricNotFound
		}
		return 0, err
	}

	if delta == nil {
		return 0, domain.ErrMetricNotFound
	}

	return *delta, nil
}

func (s *Storage) GetAll(ctx context.Context) (map[string]float64, map[string]int64, error) {
	query := `SELECT id, type, delta, value FROM metrics;`
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	gauges := make(map[string]float64, 32)
	counters := make(map[string]int64, 32)

	for rows.Next() {
		var id, mtype string
		var delta *int64
		var value *float64

		if err := rows.Scan(&id, &mtype, &delta, &value); err != nil {
			return nil, nil, err
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
		return nil, nil, err
	}

	return gauges, counters, nil
}

package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/astre-ash/omnigo/internal/domain"
)

func TestStorage_Ping(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		mock.ExpectPing()

		s := NewWithPool(mock)
		require.NoError(t, s.Ping(ctx))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("failure", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		errPing := errors.New("connection refused")
		mock.ExpectPing().WillReturnError(errPing)

		s := NewWithPool(mock)
		err = s.Ping(ctx)
		assert.ErrorIs(t, err, errPing)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestStorage_UpdateGauge(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		mock.ExpectExec("INSERT INTO metrics").
			WithArgs(pgx.NamedArgs{
				"id":    "Alloc",
				"type":  domain.TypeGauge,
				"value": 42.5,
			}).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		s := NewWithPool(mock)
		require.NoError(t, s.UpdateGauge(ctx, "Alloc", 42.5))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("db error", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		errDB := errors.New("exec error")
		mock.ExpectExec("INSERT INTO metrics").
			WithArgs(pgx.NamedArgs{
				"id":    "Alloc",
				"type":  domain.TypeGauge,
				"value": 42.5,
			}).
			WillReturnError(errDB)

		s := NewWithPool(mock)
		err = s.UpdateGauge(ctx, "Alloc", 42.5)
		require.ErrorIs(t, err, errDB)
		assert.Contains(t, err.Error(), "failed to update gauge Alloc")
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestStorage_UpdateCounter(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		mock.ExpectExec("INSERT INTO metrics").
			WithArgs(pgx.NamedArgs{
				"id":    "PollCount",
				"type":  domain.TypeCounter,
				"delta": int64(5),
			}).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		s := NewWithPool(mock)
		require.NoError(t, s.UpdateCounter(ctx, "PollCount", 5))
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestStorage_GetGauge(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		rows := pgxmock.NewRows([]string{"value"}).AddRow(123.45)
		mock.ExpectQuery("SELECT value FROM metrics").
			WithArgs(pgx.NamedArgs{
				"name": "Alloc",
				"type": domain.TypeGauge,
			}).
			WillReturnRows(rows)

		s := NewWithPool(mock)
		val, err := s.GetGauge(ctx, "Alloc")
		require.NoError(t, err)
		assert.Equal(t, 123.45, val)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("not found on ErrNoRows", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		mock.ExpectQuery("SELECT value FROM metrics").
			WithArgs(pgx.NamedArgs{
				"name": "Missing",
				"type": domain.TypeGauge,
			}).
			WillReturnError(pgx.ErrNoRows)

		s := NewWithPool(mock)
		_, err = s.GetGauge(ctx, "Missing")
		assert.ErrorIs(t, err, domain.ErrMetricNotFound)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("not found when value is NULL", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		rows := pgxmock.NewRows([]string{"value"}).AddRow(nil)
		mock.ExpectQuery("SELECT value FROM metrics").
			WithArgs(pgx.NamedArgs{
				"name": "NullGauge",
				"type": domain.TypeGauge,
			}).
			WillReturnRows(rows)

		s := NewWithPool(mock)
		_, err = s.GetGauge(ctx, "NullGauge")
		assert.ErrorIs(t, err, domain.ErrMetricNotFound)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestStorage_GetCounter(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		rows := pgxmock.NewRows([]string{"delta"}).AddRow(int64(42))
		mock.ExpectQuery("SELECT delta FROM metrics").
			WithArgs(pgx.NamedArgs{
				"name": "PollCount",
				"type": domain.TypeCounter,
			}).
			WillReturnRows(rows)

		s := NewWithPool(mock)
		val, err := s.GetCounter(ctx, "PollCount")
		require.NoError(t, err)
		assert.Equal(t, int64(42), val)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("not found when delta is NULL", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		rows := pgxmock.NewRows([]string{"delta"}).AddRow(nil)
		mock.ExpectQuery("SELECT delta FROM metrics").
			WithArgs(pgx.NamedArgs{
				"name": "NullCounter",
				"type": domain.TypeCounter,
			}).
			WillReturnRows(rows)

		s := NewWithPool(mock)
		_, err = s.GetCounter(ctx, "NullCounter")
		assert.ErrorIs(t, err, domain.ErrMetricNotFound)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestStorage_GetAll(t *testing.T) {
	ctx := context.Background()

	t.Run("success skips NULL values", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		rows := pgxmock.NewRows([]string{"id", "type", "delta", "value"}).
			AddRow("Alloc", domain.TypeGauge, nil, 10.5).
			AddRow("PollCount", domain.TypeCounter, int64(7), nil).
			AddRow("EmptyGauge", domain.TypeGauge, nil, nil).
			AddRow("EmptyCounter", domain.TypeCounter, nil, nil)

		mock.ExpectQuery("SELECT id, type, delta, value FROM metrics").WillReturnRows(rows)

		s := NewWithPool(mock)
		gauges, counters, err := s.GetAll(ctx)
		require.NoError(t, err)
		assert.Equal(t, map[string]float64{"Alloc": 10.5}, gauges)
		assert.Equal(t, map[string]int64{"PollCount": 7}, counters)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("rows iteration error", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		errRow := errors.New("stream broken")
		rows := pgxmock.NewRows([]string{"id", "type", "delta", "value"}).
			AddRow("Alloc", domain.TypeGauge, nil, 10.5).
			RowError(0, errRow)

		mock.ExpectQuery("SELECT id, type, delta, value FROM metrics").WillReturnRows(rows)

		s := NewWithPool(mock)
		gauges, counters, err := s.GetAll(ctx)
		require.ErrorIs(t, err, errRow)
		assert.Nil(t, gauges)
		assert.Nil(t, counters)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestStorage_UpdateBatch(t *testing.T) {
	ctx := context.Background()

	t.Run("validation errors fail before Begin", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		s := NewWithPool(mock)

		err = s.UpdateBatch(ctx, []domain.Metrics{
			{ID: "Alloc", MType: domain.TypeGauge, Value: nil},
		})
		assert.ErrorIs(t, err, domain.ErrInvalidValue)

		err = s.UpdateBatch(ctx, []domain.Metrics{
			{ID: "Poll", MType: domain.TypeCounter, Delta: nil},
		})
		assert.ErrorIs(t, err, domain.ErrInvalidValue)

		err = s.UpdateBatch(ctx, []domain.Metrics{
			{ID: "Hist", MType: "histogram"},
		})
		assert.ErrorIs(t, err, domain.ErrUnknownMetricType)

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("success commits batch", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		mock.ExpectBegin()
		eb := mock.ExpectBatch()
		eb.ExpectExec("INSERT INTO metrics").
			WithArgs(pgx.NamedArgs{
				"id":    "Alloc",
				"type":  domain.TypeGauge,
				"value": 12.34,
			}).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))
		eb.ExpectExec("INSERT INTO metrics").
			WithArgs(pgx.NamedArgs{
				"id":    "Poll",
				"type":  domain.TypeCounter,
				"delta": int64(3),
			}).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mock.ExpectCommit()

		s := NewWithPool(mock)
		val := 12.34
		delta := int64(3)
		err = s.UpdateBatch(ctx, []domain.Metrics{
			{ID: "Alloc", MType: domain.TypeGauge, Value: &val},
			{ID: "Poll", MType: domain.TypeCounter, Delta: &delta},
		})
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("rollbacks transaction on batch statement error", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		errExec := errors.New("constraint violation")
		mock.ExpectBegin()
		eb := mock.ExpectBatch()
		eb.ExpectExec("INSERT INTO metrics").
			WithArgs(pgx.NamedArgs{
				"id":    "Alloc",
				"type":  domain.TypeGauge,
				"value": 12.34,
			}).
			WillReturnError(errExec)
		mock.ExpectRollback()

		s := NewWithPool(mock)
		val := 12.34
		err = s.UpdateBatch(ctx, []domain.Metrics{
			{ID: "Alloc", MType: domain.TypeGauge, Value: &val},
		})
		require.ErrorIs(t, err, errExec)
		assert.Contains(t, err.Error(), "failed to execute batch statement")
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

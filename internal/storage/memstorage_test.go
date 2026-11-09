package storage

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/astre-ash/omnigo/internal/domain"
)

func TestNewMemStorage(t *testing.T) {
	s := NewMemStorage()
	require.NotNil(t, s)
	assert.NotNil(t, s.gauges)
	assert.NotNil(t, s.counters)
}

func TestMemStorage_Gauge(t *testing.T) {
	ctx := context.Background()

	t.Run("gauge not found", func(t *testing.T) {
		s := NewMemStorage()

		val, err := s.GetGauge(ctx, "non_existent")
		assert.ErrorIs(t, err, domain.ErrMetricNotFound)
		assert.Equal(t, 0.0, val)
	})

	t.Run("gauge write and overwrite", func(t *testing.T) {
		s := NewMemStorage()

		// Initial write.
		err := s.UpdateGauge(ctx, "cpu_load", 12.34)
		require.NoError(t, err)

		val, err := s.GetGauge(ctx, "cpu_load")
		require.NoError(t, err)
		assert.Equal(t, 12.34, val)

		// Value replacement.
		err = s.UpdateGauge(ctx, "cpu_load", 99.99)
		require.NoError(t, err)

		val, err = s.GetGauge(ctx, "cpu_load")
		require.NoError(t, err)
		assert.Equal(t, 99.99, val)
	})

	t.Run("context canceled", func(t *testing.T) {
		s := NewMemStorage()
		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()

		err := s.UpdateGauge(canceledCtx, "cpu_load", 10.0)
		assert.ErrorIs(t, err, context.Canceled)

		_, err = s.GetGauge(canceledCtx, "cpu_load")
		assert.ErrorIs(t, err, context.Canceled)
	})
}

func TestMemStorage_Counter(t *testing.T) {
	ctx := context.Background()

	t.Run("counter not found", func(t *testing.T) {
		s := NewMemStorage()

		val, err := s.GetCounter(ctx, "non_existent")
		assert.ErrorIs(t, err, domain.ErrMetricNotFound)
		assert.Equal(t, int64(0), val)
	})

	t.Run("counter accumulate", func(t *testing.T) {
		s := NewMemStorage()

		// First increment.
		err := s.UpdateCounter(ctx, "page_views", 10)
		require.NoError(t, err)

		val, err := s.GetCounter(ctx, "page_views")
		require.NoError(t, err)
		assert.Equal(t, int64(10), val)

		// Second increment (10 + 5 = 15).
		err = s.UpdateCounter(ctx, "page_views", 5)
		require.NoError(t, err)

		val, err = s.GetCounter(ctx, "page_views")
		require.NoError(t, err)
		assert.Equal(t, int64(15), val)

		// Third increment (15 + 20 = 35).
		err = s.UpdateCounter(ctx, "page_views", 20)
		require.NoError(t, err)

		val, err = s.GetCounter(ctx, "page_views")
		require.NoError(t, err)
		assert.Equal(t, int64(35), val)
	})

	t.Run("context canceled", func(t *testing.T) {
		s := NewMemStorage()
		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()

		err := s.UpdateCounter(canceledCtx, "page_views", 5)
		assert.ErrorIs(t, err, context.Canceled)

		_, err = s.GetCounter(canceledCtx, "page_views")
		assert.ErrorIs(t, err, context.Canceled)
	})
}

func TestMemStorage_GetAll(t *testing.T) {
	ctx := context.Background()

	t.Run("empty storage", func(t *testing.T) {
		s := NewMemStorage()

		gauges, counters, err := s.GetAll(ctx)
		require.NoError(t, err)
		assert.NotNil(t, gauges)
		assert.NotNil(t, counters)
		assert.Empty(t, gauges)
		assert.Empty(t, counters)
	})

	t.Run("returns all metrics", func(t *testing.T) {
		s := NewMemStorage()

		expectedGauges := map[string]float64{
			"cpu_load": 12.34,
			"ram_free": 1024.5,
		}
		for k, v := range expectedGauges {
			require.NoError(t, s.UpdateGauge(ctx, k, v))
		}

		expectedCounters := map[string]int64{
			"poll_count": 10,
			"requests":   42,
		}
		for k, v := range expectedCounters {
			require.NoError(t, s.UpdateCounter(ctx, k, v))
		}

		gauges, counters, err := s.GetAll(ctx)
		require.NoError(t, err)
		assert.Equal(t, expectedGauges, gauges)
		assert.Equal(t, expectedCounters, counters)
	})

	t.Run("returned maps are isolated copies", func(t *testing.T) {
		s := NewMemStorage()

		require.NoError(t, s.UpdateGauge(ctx, "cpu_load", 12.34))
		require.NoError(t, s.UpdateCounter(ctx, "requests", 42))

		gauges, counters, err := s.GetAll(ctx)
		require.NoError(t, err)

		gauges["cpu_load"] = 0.0
		gauges["external_metric"] = 999.9

		counters["requests"] = 99999
		counters["external_counter"] = 1

		valG, err := s.GetGauge(ctx, "cpu_load")
		require.NoError(t, err)
		assert.Equal(t, 12.34, valG)

		_, err = s.GetGauge(ctx, "external_metric")
		assert.ErrorIs(t, err, domain.ErrMetricNotFound)

		valC, err := s.GetCounter(ctx, "requests")
		require.NoError(t, err)
		assert.Equal(t, int64(42), valC)

		_, err = s.GetCounter(ctx, "external_counter")
		assert.ErrorIs(t, err, domain.ErrMetricNotFound)
	})

	t.Run("context canceled", func(t *testing.T) {
		s := NewMemStorage()
		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()

		gauges, counters, err := s.GetAll(canceledCtx)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Nil(t, gauges)
		assert.Nil(t, counters)
	})
}

func TestMemStorage_Snapshot(t *testing.T) {
	ctx := context.Background()

	t.Run("empty storage returns empty slice", func(t *testing.T) {
		s := NewMemStorage()

		snapshot := s.Snapshot()
		require.NotNil(t, snapshot)
		assert.Empty(t, snapshot)
	})

	t.Run("snapshot contains all metrics with isolated pointers", func(t *testing.T) {
		s := NewMemStorage()

		require.NoError(t, s.UpdateGauge(ctx, "Alloc", 123.45))
		require.NoError(t, s.UpdateCounter(ctx, "PollCount", 10))

		snapshot := s.Snapshot()
		require.Len(t, snapshot, 2)

		// Assert gauge.
		allocMetric, found := findMetricInSlice(snapshot, "Alloc")
		require.True(t, found)
		assert.Equal(t, domain.TypeGauge, allocMetric.MType)
		require.NotNil(t, allocMetric.Value)
		assert.Equal(t, 123.45, *allocMetric.Value)
		assert.Nil(t, allocMetric.Delta)

		// Assert counter.
		pollMetric, found := findMetricInSlice(snapshot, "PollCount")
		require.True(t, found)
		assert.Equal(t, domain.TypeCounter, pollMetric.MType)
		require.NotNil(t, pollMetric.Delta)
		assert.Equal(t, int64(10), *pollMetric.Delta)
		assert.Nil(t, pollMetric.Value)
	})
}

func TestMemStorage_Restore(t *testing.T) {
	ctx := context.Background()

	t.Run("restore metrics into empty storage", func(t *testing.T) {
		s := NewMemStorage()

		metrics := []domain.Metrics{
			{ID: "Alloc", MType: domain.TypeGauge, Value: float64Ptr(543.21)},
			{ID: "PollCount", MType: domain.TypeCounter, Delta: int64Ptr(42)},
		}

		s.Restore(metrics)

		gaugeVal, err := s.GetGauge(ctx, "Alloc")
		require.NoError(t, err)
		assert.Equal(t, 543.21, gaugeVal)

		counterVal, err := s.GetCounter(ctx, "PollCount")
		require.NoError(t, err)
		assert.Equal(t, int64(42), counterVal)
	})

	t.Run("restore overwrites existing metrics", func(t *testing.T) {
		s := NewMemStorage()

		require.NoError(t, s.UpdateGauge(ctx, "Alloc", 100.0))
		require.NoError(t, s.UpdateCounter(ctx, "PollCount", 5))

		metrics := []domain.Metrics{
			{ID: "Alloc", MType: domain.TypeGauge, Value: float64Ptr(200.0)},
			{ID: "PollCount", MType: domain.TypeCounter, Delta: int64Ptr(25)},
		}

		s.Restore(metrics)

		gaugeVal, err := s.GetGauge(ctx, "Alloc")
		require.NoError(t, err)
		assert.Equal(t, 200.0, gaugeVal)

		counterVal, err := s.GetCounter(ctx, "PollCount")
		require.NoError(t, err)
		assert.Equal(t, int64(25), counterVal)
	})

	t.Run("ignores metrics with nil values and unknown types gracefully", func(t *testing.T) {
		s := NewMemStorage()

		metrics := []domain.Metrics{
			{ID: "NilGauge", MType: domain.TypeGauge, Value: nil},
			{ID: "NilCounter", MType: domain.TypeCounter, Delta: nil},
			{ID: "Unknown", MType: "unknown_type", Value: float64Ptr(1.0)},
		}

		s.Restore(metrics)

		_, err := s.GetGauge(ctx, "NilGauge")
		assert.ErrorIs(t, err, domain.ErrMetricNotFound)

		_, err = s.GetCounter(ctx, "NilCounter")
		assert.ErrorIs(t, err, domain.ErrMetricNotFound)

		gauges, counters, err := s.GetAll(ctx)
		require.NoError(t, err)
		assert.Empty(t, gauges)
		assert.Empty(t, counters)
	})
}

func TestMemStorage_SnapshotAndRestore_RoundTrip(t *testing.T) {
	ctx := context.Background()

	s1 := NewMemStorage()
	require.NoError(t, s1.UpdateGauge(ctx, "Alloc", 100.5))
	require.NoError(t, s1.UpdateGauge(ctx, "Sys", 200.0))
	require.NoError(t, s1.UpdateCounter(ctx, "PollCount", 15))

	snapshot := s1.Snapshot()

	s2 := NewMemStorage()
	s2.Restore(snapshot)

	gauges1, counters1, err := s1.GetAll(ctx)
	require.NoError(t, err)

	gauges2, counters2, err := s2.GetAll(ctx)
	require.NoError(t, err)

	assert.Equal(t, gauges1, gauges2)
	assert.Equal(t, counters1, counters2)
}

func TestMemStorage_Concurrent(t *testing.T) {
	ctx := context.Background()
	s := NewMemStorage()

	const (
		goroutines = 50
		iterations = 500
	)

	var wg sync.WaitGroup
	wg.Add(goroutines * 3)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = s.UpdateCounter(ctx, "requests", 1)
			}
		}()
	}

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = s.UpdateGauge(ctx, "temperature", float64(id+j))
				_, _ = s.GetGauge(ctx, "temperature")
				_, _ = s.GetCounter(ctx, "requests")

				if j%100 == 0 {
					_, _, _ = s.GetAll(ctx)
				}
			}
		}(i)
	}

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				if j%50 == 0 {
					snapshot := s.Snapshot()
					if len(snapshot) > 0 {
						s.Restore(snapshot[:1])
					}
				}
			}
		}()
	}

	wg.Wait()

	val, err := s.GetCounter(ctx, "requests")
	require.NoError(t, err, "counter 'requests' must exist")
	assert.Greater(t, val, int64(0))
}

func float64Ptr(v float64) *float64 {
	return &v
}

func int64Ptr(v int64) *int64 {
	return &v
}

func findMetricInSlice(metrics []domain.Metrics, id string) (domain.Metrics, bool) {
	for _, m := range metrics {
		if m.ID == id {
			return m, true
		}
	}
	return domain.Metrics{}, false
}

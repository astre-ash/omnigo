package storage

import (
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
	t.Run("gauge not found", func(t *testing.T) {
		s := NewMemStorage()

		val, err := s.GetGauge("non_existent")
		assert.ErrorIs(t, err, domain.ErrMetricNotFound)
		assert.Equal(t, 0.0, val)
	})

	t.Run("gauge write and overwrite", func(t *testing.T) {
		s := NewMemStorage()

		// Initial write.
		s.UpdateGauge("cpu_load", 12.34)
		val, err := s.GetGauge("cpu_load")
		require.NoError(t, err)
		assert.Equal(t, 12.34, val)

		// Value replacement.
		s.UpdateGauge("cpu_load", 99.99)
		val, err = s.GetGauge("cpu_load")
		require.NoError(t, err)
		assert.Equal(t, 99.99, val)
	})
}

func TestMemStorage_Counter(t *testing.T) {
	t.Run("counter not found", func(t *testing.T) {
		s := NewMemStorage()

		val, err := s.GetCounter("non_existent")
		assert.ErrorIs(t, err, domain.ErrMetricNotFound)
		assert.Equal(t, int64(0), val)
	})

	t.Run("counter accumulate", func(t *testing.T) {
		s := NewMemStorage()

		// First icrement.
		s.UpdateCounter("page_views", 10)
		val, err := s.GetCounter("page_views")
		require.NoError(t, err)
		assert.Equal(t, int64(10), val)

		// Second increment (10 + 5 = 15).
		s.UpdateCounter("page_views", 5)
		val, err = s.GetCounter("page_views")
		require.NoError(t, err)
		assert.Equal(t, int64(15), val)

		// Third increment (15 + 20 = 35).
		s.UpdateCounter("page_views", 20)
		val, err = s.GetCounter("page_views")
		require.NoError(t, err)
		assert.Equal(t, int64(35), val)
	})
}

func TestMemStorage_GetAllGauges(t *testing.T) {
	t.Run("empty storage", func(t *testing.T) {
		s := NewMemStorage()

		gauges := s.GetAllGauges()
		require.NotNil(t, gauges)
		assert.Empty(t, gauges)
	})

	t.Run("returns all gauges", func(t *testing.T) {
		s := NewMemStorage()

		expected := map[string]float64{
			"cpu_load": 12.34,
			"ram_free": 1024.5,
		}

		for k, v := range expected {
			s.UpdateGauge(k, v)
		}

		gauges := s.GetAllGauges()
		assert.Equal(t, expected, gauges)
	})

	t.Run("returned map is an isolated copy", func(t *testing.T) {
		s := NewMemStorage()

		s.UpdateGauge("cpu_load", 12.34)

		gauges := s.GetAllGauges()
		gauges["cpu_load"] = 0.0
		gauges["external_metric"] = 999.9

		val, err := s.GetGauge("cpu_load")
		require.NoError(t, err)
		assert.Equal(t, 12.34, val)

		_, err = s.GetGauge("external_metric")
		assert.ErrorIs(t, err, domain.ErrMetricNotFound, "external modifications should not affect storage")
	})
}

func TestMemStorage_GetAllCounters(t *testing.T) {
	t.Run("empty storage", func(t *testing.T) {
		s := NewMemStorage()

		counters := s.GetAllCounters()
		require.NotNil(t, counters)
		assert.Empty(t, counters)
	})

	t.Run("returns all counters", func(t *testing.T) {
		s := NewMemStorage()

		s.UpdateCounter("poll_count", 5)
		s.UpdateCounter("poll_count", 5) // в сумме 10
		s.UpdateCounter("requests", 42)

		expected := map[string]int64{
			"poll_count": 10,
			"requests":   42,
		}

		counters := s.GetAllCounters()
		assert.Equal(t, expected, counters)
	})

	t.Run("returned map is an isolated copy", func(t *testing.T) {
		s := NewMemStorage()

		s.UpdateCounter("requests", 42)

		counters := s.GetAllCounters()
		counters["requests"] = 99999
		counters["external_counter"] = 1

		val, err := s.GetCounter("requests")
		require.NoError(t, err)
		assert.Equal(t, int64(42), val)

		_, err = s.GetCounter("external_counter")
		assert.ErrorIs(t, err, domain.ErrMetricNotFound, "external modifications should not affect storage")
	})
}

func TestMemStorage_Snapshot(t *testing.T) {
	t.Run("empty storage returns empty slice", func(t *testing.T) {
		s := NewMemStorage()

		snapshot := s.Snapshot()
		require.NotNil(t, snapshot)
		assert.Empty(t, snapshot)
	})

	t.Run("snapshot contains all metrics with isolated pointers", func(t *testing.T) {
		s := NewMemStorage()

		s.UpdateGauge("Alloc", 123.45)
		s.UpdateCounter("PollCount", 10)

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
	t.Run("restore metrics into empty storage", func(t *testing.T) {
		s := NewMemStorage()

		metrics := []domain.Metrics{
			{ID: "Alloc", MType: domain.TypeGauge, Value: float64Ptr(543.21)},
			{ID: "PollCount", MType: domain.TypeCounter, Delta: int64Ptr(42)},
		}

		s.Restore(metrics)

		gaugeVal, err := s.GetGauge("Alloc")
		require.NoError(t, err)
		assert.Equal(t, 543.21, gaugeVal)

		counterVal, err := s.GetCounter("PollCount")
		require.NoError(t, err)
		assert.Equal(t, int64(42), counterVal)
	})

	t.Run("restore overwrites existing metrics", func(t *testing.T) {
		s := NewMemStorage()

		s.UpdateGauge("Alloc", 100.0)
		s.UpdateCounter("PollCount", 5)

		metrics := []domain.Metrics{
			{ID: "Alloc", MType: domain.TypeGauge, Value: float64Ptr(200.0)},
			{ID: "PollCount", MType: domain.TypeCounter, Delta: int64Ptr(25)},
		}

		s.Restore(metrics)

		gaugeVal, err := s.GetGauge("Alloc")
		require.NoError(t, err)
		assert.Equal(t, 200.0, gaugeVal)

		counterVal, err := s.GetCounter("PollCount")
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

		_, err := s.GetGauge("NilGauge")
		assert.ErrorIs(t, err, domain.ErrMetricNotFound)

		_, err = s.GetCounter("NilCounter")
		assert.ErrorIs(t, err, domain.ErrMetricNotFound)

		assert.Empty(t, s.GetAllGauges())
		assert.Empty(t, s.GetAllCounters())
	})
}

func TestMemStorage_SnapshotAndRestore_RoundTrip(t *testing.T) {
	s1 := NewMemStorage()
	s1.UpdateGauge("Alloc", 100.5)
	s1.UpdateGauge("Sys", 200.0)
	s1.UpdateCounter("PollCount", 15)

	snapshot := s1.Snapshot()

	s2 := NewMemStorage()
	s2.Restore(snapshot)

	assert.Equal(t, s1.GetAllGauges(), s2.GetAllGauges())
	assert.Equal(t, s1.GetAllCounters(), s2.GetAllCounters())
}

func TestMemStorage_Concurrent(t *testing.T) {
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
				s.UpdateCounter("requests", 1)
			}
		}()
	}

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				s.UpdateGauge("temperature", float64(id+j))
				_, _ = s.GetGauge("temperature")
				_, _ = s.GetCounter("requests")

				if j%100 == 0 {
					_ = s.GetAllGauges()
					_ = s.GetAllCounters()
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

	val, err := s.GetCounter("requests")
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

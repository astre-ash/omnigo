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

func TestMemStorage_Concurrent(t *testing.T) {
	s := NewMemStorage()

	const (
		goroutines = 50
		iterations = 1000
	)

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

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

	wg.Wait()

	val, err := s.GetCounter("requests")
	require.NoError(t, err, "counter 'requests' must exist")
	assert.Equal(t, int64(goroutines*iterations), val)
}

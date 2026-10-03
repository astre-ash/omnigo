package storage

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemStorage_Gauge(t *testing.T) {
	s := NewMemStorage()

	t.Run("gauge not found", func(t *testing.T) {
		val, ok := s.GetGauge("non_existent")
		assert.False(t, ok)
		assert.Equal(t, 0.0, val)
	})

	t.Run("gauge write and overwrite", func(t *testing.T) {
		// Initial write.
		s.UpdateGauge("cpu_load", 12.34)
		val, ok := s.GetGauge("cpu_load")
		require.True(t, ok, "metric should exist")
		assert.Equal(t, 12.34, val)

		// Value replacement.
		s.UpdateGauge("cpu_load", 99.99)
		val, ok = s.GetGauge("cpu_load")
		require.True(t, ok)
		assert.Equal(t, 99.99, val)
	})
}

func TestMemStorage_Counter(t *testing.T) {
	s := NewMemStorage()

	t.Run("counter not found", func(t *testing.T) {
		val, ok := s.GetCounter("non_existent")
		assert.False(t, ok)
		assert.Equal(t, int64(0), val)
	})

	t.Run("counter accumulate", func(t *testing.T) {
		// First icrement.
		s.UpdateCounter("page_views", 10)
		val, ok := s.GetCounter("page_views")
		require.True(t, ok, "metric should exist")
		assert.Equal(t, int64(10), val)

		// Second increment (10 + 5 = 15).
		s.UpdateCounter("page_views", 5)
		val, ok = s.GetCounter("page_views")
		require.True(t, ok)
		assert.Equal(t, int64(15), val)

		// Third increment (15 + 20 = 35).
		s.UpdateCounter("page_views", 20)
		val, ok = s.GetCounter("page_views")
		require.True(t, ok)
		assert.Equal(t, int64(35), val)
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
			}
		}(i)
	}

	wg.Wait()

	val, ok := s.GetCounter("requests")
	require.True(t, ok, "counter 'requests' must exist")
	assert.Equal(t, int64(goroutines*iterations), val)
}

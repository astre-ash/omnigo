package agent

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/astre-ash/omnigo/internal/domain"
)

func findMetric(metrics []domain.Metrics, id string) (domain.Metrics, bool) {
	for _, m := range metrics {
		if m.ID == id {
			return m, true
		}
	}
	return domain.Metrics{}, false
}

func TestCollector_PollAndMetrics(t *testing.T) {
	collector := NewCollector()

	initialMetrics := collector.GetMetrics()
	require.NotEmpty(t, initialMetrics, "metrics slice must not be empty even before poll")

	pollCountMetric, ok := findMetric(initialMetrics, "PollCount")
	require.True(t, ok, "metric PollCount must be present")
	assert.Equal(t, domain.TypeCounter, pollCountMetric.MType)
	require.NotNil(t, pollCountMetric.Delta, "PollCount Delta pointer must not be nil")
	assert.Equal(t, int64(0), *pollCountMetric.Delta, "initial PollCount must be 0")

	collector.Poll()
	metricsAfterFirstPoll := collector.GetMetrics()

	const expectedMetricsCount = 29
	assert.Len(t, metricsAfterFirstPoll, expectedMetricsCount, "unexpected count of collected metrics")

	pollCountMetric, ok = findMetric(metricsAfterFirstPoll, "PollCount")
	require.True(t, ok, "metric PollCount must be present")
	assert.Equal(t, domain.TypeCounter, pollCountMetric.MType)
	require.NotNil(t, pollCountMetric.Delta)
	assert.Equal(t, int64(1), *pollCountMetric.Delta, "PollCount must be 1 after first poll")

	randomMetric, found := findMetric(metricsAfterFirstPoll, "RandomValue")
	require.True(t, found, "metric RandomValue must be present")
	assert.Equal(t, domain.TypeGauge, randomMetric.MType)
	require.NotNil(t, randomMetric.Value, "RandomValue pointer must not be nil")
	assert.GreaterOrEqual(t, *randomMetric.Value, 0.0, "RandomValue must be >= 0.0")
	assert.Less(t, *randomMetric.Value, 1.0, "RandomValue must be < 1.0")

	allocMetric, found := findMetric(metricsAfterFirstPoll, "Alloc")
	require.True(t, found, "metric Alloc must be present")
	assert.Equal(t, domain.TypeGauge, allocMetric.MType)
	require.NotNil(t, allocMetric.Value, "Alloc pointer must not be nil")
	assert.Greater(t, *allocMetric.Value, 0.0, "Alloc should be greater than 0 after Poll")

	collector.Poll()
	metricsAfterSecondPoll := collector.GetMetrics()
	pollCountMetric, ok = findMetric(metricsAfterSecondPoll, "PollCount")
	require.True(t, ok)
	require.NotNil(t, pollCountMetric.Delta)
	assert.Equal(t, int64(2), *pollCountMetric.Delta, "PollCount must be 2 after second poll")
}

func TestCollector_ResetPollCount(t *testing.T) {
	collector := NewCollector()

	collector.Poll()
	collector.Poll()

	metrics := collector.GetMetrics()
	pollCount, ok := findMetric(metrics, "PollCount")
	require.True(t, ok)
	require.NotNil(t, pollCount.Delta)
	assert.Equal(t, int64(2), *pollCount.Delta)

	collector.ResetPollCount()

	metricsAfterReset := collector.GetMetrics()
	pollCountReset, ok := findMetric(metricsAfterReset, "PollCount")
	require.True(t, ok)
	require.NotNil(t, pollCountReset.Delta)
	assert.Equal(t, int64(0), *pollCountReset.Delta, "PollCount must be 0 after reset")

	collector.Poll()
	metricsAfterNewPoll := collector.GetMetrics()
	pollCountNew, ok := findMetric(metricsAfterNewPoll, "PollCount")
	require.True(t, ok)
	require.NotNil(t, pollCountNew.Delta)
	assert.Equal(t, int64(1), *pollCountNew.Delta, "PollCount must be 1 after poll following reset")
}

func TestCollector_Concurrency(t *testing.T) {
	collector := NewCollector()
	var wg sync.WaitGroup

	numGoroutines := 10
	iterations := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				collector.Poll()
			}
		}()
	}

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				metrics := collector.GetMetrics()
				_ = metrics
			}
		}()
	}

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				collector.ResetPollCount()
			}
		}()
	}

	wg.Wait()
}

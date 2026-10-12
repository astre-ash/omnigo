package agent

import (
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/astre-ash/omnigo/internal/domain"
)

func findMetric(metrics []MetricData, name string) (MetricData, bool) {
	for _, m := range metrics {
		if m.Name == name {
			return m, true
		}
	}
	return MetricData{}, false
}

func TestCollector_PollAndMetrics(t *testing.T) {
	collector := NewCollector()

	// 1. Check initial state before any Poll.
	initialMetrics := collector.GetMetrics()
	require.NotEmpty(t, initialMetrics, "metrics slice must not be empty even before poll")

	pollCountMetric, ok := findMetric(initialMetrics, "PollCount")
	require.True(t, ok, "metric PollCount must be present")
	assert.Equal(t, domain.TypeCounter, pollCountMetric.Type)
	assert.Equal(t, "0", pollCountMetric.Value, "initial PollCount must be 0")

	// 2. Perform first Poll.
	collector.Poll()
	metricsAfterFirstPoll := collector.GetMetrics()

	// Total metrics count: 27 runtime gauges + 1 RandomValue gauge + 1 PollCount counter = 29.
	const expectedMetricsCount = 29
	assert.Len(t, metricsAfterFirstPoll, expectedMetricsCount, "unexpected count of collected metrics")

	// Check PollCount increment.
	pollCountMetric, ok = findMetric(metricsAfterFirstPoll, "PollCount")
	require.True(t, ok, "metric PollCount must be present")
	assert.Equal(t, "1", pollCountMetric.Value, "PollCount must be 1 after first poll")

	// Check RandomValue validity.
	randomMetric, found := findMetric(metricsAfterFirstPoll, "RandomValue")
	require.True(t, found, "metric RandomValue must be present")
	assert.Equal(t, domain.TypeGauge, randomMetric.Type)

	randVal, err := strconv.ParseFloat(randomMetric.Value, 64)
	require.NoError(t, err, "RandomValue must be a valid float string")
	assert.GreaterOrEqual(t, randVal, 0.0, "RandomValue must be >= 0.0")
	assert.Less(t, randVal, 1.0, "RandomValue must be < 1.0")

	// Check that runtime metrics are actually gathered (Alloc should be > 0 in a running program).
	allocMetric, found := findMetric(metricsAfterFirstPoll, "Alloc")
	require.True(t, found, "metric Alloc must be present")
	assert.Equal(t, domain.TypeGauge, allocMetric.Type)

	allocVal, err := strconv.ParseFloat(allocMetric.Value, 64)
	require.NoError(t, err, "Alloc must be a valid float string")
	assert.Greater(t, allocVal, 0.0, "Alloc should be greater than 0 after Poll")

	// 3. Second Poll: verify counter increments monotonically.
	collector.Poll()
	metricsAfterSecondPoll := collector.GetMetrics()
	pollCountMetric, _ = findMetric(metricsAfterSecondPoll, "PollCount")
	assert.Equal(t, "2", pollCountMetric.Value, "PollCount must be 2 after second poll")
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

	wg.Wait()
}

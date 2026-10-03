package agent

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSender struct {
	sentCount int
}

func (m *mockSender) SendAll(metrics []MetricData) {
	m.sentCount += len(metrics)
}

func TestAgent_Run(t *testing.T) {
	cfg := AgentConfig{
		PollInterval:   10 * time.Millisecond,
		ReportInterval: 30 * time.Millisecond,
	}

	collector := NewCollector()
	mock := &mockSender{}
	app := NewAgent(cfg, collector, mock)

	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		app.Run(ctx)
		close(done)
	}()

	select {
	case <-done:

	case <-time.After(500 * time.Millisecond):
		t.Fatal("agent failed to stop within expected timeout")
	}

	assert.GreaterOrEqual(t, mock.sentCount, 29, "mockSender must receive at least one batch of metrics")

	metrics := collector.GetMetrics()
	pollMetric, found := findMetric(metrics, "PollCount")
	require.True(t, found, "metric PollCount must be present")

	pollCount, err := strconv.ParseInt(pollMetric.Value, 10, 64)
	require.NoError(t, err)
	assert.Greater(t, pollCount, int64(0), "pollCount must be incremented")
}

func TestAgent_Run_ImmediateCancel(t *testing.T) {
	cfg := AgentConfig{
		PollInterval:   1 * time.Second,
		ReportInterval: 5 * time.Second,
	}

	collector := NewCollector()
	mock := &mockSender{}
	app := NewAgent(cfg, collector, mock)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		app.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:

	case <-time.After(100 * time.Millisecond):
		t.Fatal("agent did not exit immediately on context cancellation")
	}

	assert.Equal(t, 0, mock.sentCount, "no metrics should be sent on immediate cancel")
}

package agent

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/astre-ash/omnigo/internal/domain"
)

type mockSender struct {
	mtx     sync.Mutex
	err     error
	batches [][]domain.Metrics
}

func (m *mockSender) SendAll(metrics []domain.Metrics) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	if m.err != nil {
		return m.err
	}

	batchCopy := make([]domain.Metrics, len(metrics))
	copy(batchCopy, metrics)
	m.batches = append(m.batches, batchCopy)

	return nil
}

func (m *mockSender) getBatches() [][]domain.Metrics {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	return m.batches
}

func TestAgent_Run_SuccessAndResetPollCount(t *testing.T) {
	pollInterval := 10 * time.Millisecond
	reportInterval := 30 * time.Millisecond

	collector := NewCollector()
	mock := &mockSender{}
	agent := NewAgent(pollInterval, reportInterval, collector, mock)

	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		agent.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("agent did not stop within expected time")
	}

	batches := mock.getBatches()
	require.NotEmpty(t, batches, "at least one batch of metrics must be sent")

	assert.Len(t, batches[0], 29, "expected 29 metrics in batch")

	firstBatchPoll, found1 := findMetric(batches[0], "PollCount")
	require.True(t, found1, "PollCount must be present in first batch")
	require.NotNil(t, firstBatchPoll.Delta, "PollCount Delta pointer must not be nil")
	assert.Greater(t, *firstBatchPoll.Delta, int64(0), "first batch pollCount should be > 0")

	if len(batches) > 1 {
		secondBatchPoll, found2 := findMetric(batches[1], "PollCount")
		require.True(t, found2, "PollCount must be present in second batch")
		require.NotNil(t, secondBatchPoll.Delta)
		assert.LessOrEqual(t, *secondBatchPoll.Delta, int64(4), "pollCount must be reset between successful reports")
	}
}

func TestAgent_Run_SendFailureRetainsPollCount(t *testing.T) {
	pollInterval := 10 * time.Millisecond
	reportInterval := 25 * time.Millisecond

	collector := NewCollector()

	mock := &mockSender{
		err: errors.New("network failure: connection refused"),
	}
	agent := NewAgent(pollInterval, reportInterval, collector, mock)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()

	done := make(chan struct{})

	go func() {
		agent.Run(ctx)
		close(done)
	}()

	<-done

	metrics := collector.GetMetrics()
	pollMetric, found := findMetric(metrics, "PollCount")
	require.True(t, found, "PollCount must be present")
	require.NotNil(t, pollMetric.Delta, "PollCount Delta must not be nil")
	assert.GreaterOrEqual(t, *pollMetric.Delta, int64(4), "pollCount must NOT be reset when sending fails")
}

func TestAgent_Run_GracefulShutdown(t *testing.T) {
	pollInterval := 1 * time.Second
	reportInterval := 5 * time.Second

	collector := NewCollector()
	mock := &mockSender{}
	agent := NewAgent(pollInterval, reportInterval, collector, mock)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		agent.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("agent failed to terminate immediately upon context cancellation")
	}

	assert.Empty(t, mock.getBatches(), "no batches should be sent when context is canceled immediately")
}

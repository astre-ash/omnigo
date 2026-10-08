package agent

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSender struct {
	mtx     sync.Mutex
	err     error
	batches [][]MetricData
}

func (m *mockSender) SendAll(metrics []MetricData) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	if m.err != nil {
		return m.err
	}

	batchCopy := make([]MetricData, len(metrics))
	copy(batchCopy, metrics)
	m.batches = append(m.batches, batchCopy)

	return nil
}

func (m *mockSender) getBatches() [][]MetricData {
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

	// 75ms = 7 polls, 2 reports.
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
	require.True(t, found1)

	val1, err := strconv.ParseInt(firstBatchPoll.Value, 10, 64)
	require.NoError(t, err)
	assert.Greater(t, val1, int64(0), "first batch pollCount should be > 0")

	if len(batches) > 1 {
		secondBatchPoll, found2 := findMetric(batches[1], "PollCount")
		require.True(t, found2)

		val2, err := strconv.ParseInt(secondBatchPoll.Value, 10, 64)
		require.NoError(t, err)
		assert.LessOrEqual(t, val2, int64(4), "pollCount must be reset between successful reports")
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
	require.True(t, found)

	finalPollCount, err := strconv.ParseInt(pollMetric.Value, 10, 64)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, finalPollCount, int64(4), "pollCount must NOT be reset when sending fails")
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

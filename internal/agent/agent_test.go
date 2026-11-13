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

var _ Sender = (*mockSender)(nil)

type mockSender struct {
	mtx     sync.Mutex
	err     error
	onSend  func(ctx context.Context, metrics []domain.Metrics) error
	sentCh  chan struct{}
	batches [][]domain.Metrics
	calls   int
}

func (m *mockSender) SendBatch(ctx context.Context, metrics []domain.Metrics) error {
	if m.onSend != nil {
		if err := m.onSend(ctx, metrics); err != nil {
			m.recordCall(nil, true)
			return err
		}
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	m.mtx.Lock()
	m.calls++
	err := m.err
	if err == nil {
		batchCopy := cloneMetrics(metrics)
		m.batches = append(m.batches, batchCopy)
	}
	ch := m.sentCh
	m.mtx.Unlock()

	if ch != nil {
		select {
		case ch <- struct{}{}:
		default:
		}
	}

	return err
}

func (m *mockSender) recordCall(metrics []domain.Metrics, failed bool) {
	m.mtx.Lock()
	m.calls++
	if !failed && metrics != nil {
		m.batches = append(m.batches, cloneMetrics(metrics))
	}
	ch := m.sentCh
	m.mtx.Unlock()

	if ch != nil {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (m *mockSender) getBatches() [][]domain.Metrics {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	res := make([][]domain.Metrics, len(m.batches))
	for i, b := range m.batches {
		res[i] = cloneMetrics(b)
	}
	return res
}

func (m *mockSender) getCalls() int {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	return m.calls
}

func cloneMetrics(metrics []domain.Metrics) []domain.Metrics {
	res := make([]domain.Metrics, len(metrics))
	for i, m := range metrics {
		res[i] = m
		if m.Delta != nil {
			d := *m.Delta
			res[i].Delta = &d
		}
		if m.Value != nil {
			v := *m.Value
			res[i].Value = &v
		}
	}
	return res
}

func TestAgent_report(t *testing.T) {
	ctx := context.Background()

	t.Run("decreases PollCount by reported delta after successful send", func(t *testing.T) {
		collector := NewCollector()
		collector.Poll()
		collector.Poll()
		collector.Poll()

		mock := &mockSender{}
		agent := NewAgent(time.Second, time.Second, collector, mock)

		agent.report(ctx)

		batches := mock.getBatches()
		require.Len(t, batches, 1)
		assert.Len(t, batches[0], 29)

		sentPoll, found := findMetric(batches[0], "PollCount")
		require.True(t, found)
		require.NotNil(t, sentPoll.Delta)
		assert.Equal(t, int64(3), *sentPoll.Delta)

		remainingPoll, found := findMetric(collector.GetMetrics(), "PollCount")
		require.True(t, found)
		require.NotNil(t, remainingPoll.Delta)
		assert.Equal(t, int64(0), *remainingPoll.Delta)
	})

	t.Run("retains new polls accumulated during in-flight SendBatch", func(t *testing.T) {
		collector := NewCollector()
		collector.Poll()
		collector.Poll()

		mock := &mockSender{
			onSend: func(ctx context.Context, metrics []domain.Metrics) error {
				collector.Poll()
				return nil
			},
		}
		agent := NewAgent(time.Second, time.Second, collector, mock)

		agent.report(ctx)

		batches := mock.getBatches()
		require.Len(t, batches, 1)

		sentPoll, found := findMetric(batches[0], "PollCount")
		require.True(t, found)
		require.NotNil(t, sentPoll.Delta)
		assert.Equal(t, int64(2), *sentPoll.Delta)

		remainingPoll, found := findMetric(collector.GetMetrics(), "PollCount")
		require.True(t, found)
		require.NotNil(t, remainingPoll.Delta)
		assert.Equal(t, int64(1), *remainingPoll.Delta, "polls made during SendBatch must be preserved")
	})

	t.Run("retains PollCount when SendBatch fails", func(t *testing.T) {
		collector := NewCollector()
		collector.Poll()
		collector.Poll()
		collector.Poll()

		mock := &mockSender{
			err: errors.New("network failure"),
		}
		agent := NewAgent(time.Second, time.Second, collector, mock)

		agent.report(ctx)

		assert.Equal(t, 1, mock.getCalls())
		assert.Empty(t, mock.getBatches())

		remainingPoll, found := findMetric(collector.GetMetrics(), "PollCount")
		require.True(t, found)
		require.NotNil(t, remainingPoll.Delta)
		assert.Equal(t, int64(3), *remainingPoll.Delta, "PollCount must not be decreased on send error")
	})
}

func TestAgent_Run_SuccessAndDecreasePollCount(t *testing.T) {
	pollInterval := 5 * time.Millisecond
	reportInterval := 15 * time.Millisecond

	collector := NewCollector()
	mock := &mockSender{
		sentCh: make(chan struct{}, 10),
	}
	agent := NewAgent(pollInterval, reportInterval, collector, mock)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		agent.Run(ctx)
		close(done)
	}()

	for i := 0; i < 2; i++ {
		select {
		case <-mock.sentCh:
		case <-time.After(1 * time.Second):
			t.Fatalf("timed out waiting for batch #%d", i+1)
		}
	}

	cancel()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("agent did not stop within expected time")
	}

	batches := mock.getBatches()
	require.GreaterOrEqual(t, len(batches), 2, "at least two batches of metrics must be sent")
	assert.Len(t, batches[0], 29, "expected 29 metrics in batch")

	firstBatchPoll, found1 := findMetric(batches[0], "PollCount")
	require.True(t, found1, "PollCount must be present in first batch")
	require.NotNil(t, firstBatchPoll.Delta, "PollCount Delta pointer must not be nil")
	assert.Greater(t, *firstBatchPoll.Delta, int64(0), "first batch PollCount should be > 0")

	secondBatchPoll, found2 := findMetric(batches[1], "PollCount")
	require.True(t, found2, "PollCount must be present in second batch")
	require.NotNil(t, secondBatchPoll.Delta)
	assert.GreaterOrEqual(t, *secondBatchPoll.Delta, int64(0))
}

func TestAgent_Run_SendFailureRetainsPollCount(t *testing.T) {
	pollInterval := 5 * time.Millisecond
	reportInterval := 15 * time.Millisecond

	collector := NewCollector()
	mock := &mockSender{
		err:    errors.New("network failure: connection refused"),
		sentCh: make(chan struct{}, 10),
	}
	agent := NewAgent(pollInterval, reportInterval, collector, mock)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		agent.Run(ctx)
		close(done)
	}()

	select {
	case <-mock.sentCh:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for failed send attempt")
	}

	cancel()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("agent did not stop within expected time")
	}

	require.GreaterOrEqual(t, mock.getCalls(), 1, "sender must be called at least once")

	metrics := collector.GetMetrics()
	pollMetric, found := findMetric(metrics, "PollCount")
	require.True(t, found, "PollCount must be present")
	require.NotNil(t, pollMetric.Delta, "PollCount Delta must not be nil")
	assert.GreaterOrEqual(t, *pollMetric.Delta, int64(1), "PollCount must NOT be decreased when sending fails")
}

func TestAgent_Run_GracefulShutdown(t *testing.T) {
	t.Run("stops immediately when context is canceled before first report", func(t *testing.T) {
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
		case <-time.After(200 * time.Millisecond):
			t.Fatal("agent failed to terminate immediately upon context cancellation")
		}

		assert.Empty(t, mock.getBatches(), "no batches should be sent when context is canceled immediately")
	})

	t.Run("waits for in-flight SendBatch to finish before exiting", func(t *testing.T) {
		inSend := make(chan struct{})
		releaseSend := make(chan struct{})
		var once sync.Once

		collector := NewCollector()
		mock := &mockSender{
			onSend: func(ctx context.Context, metrics []domain.Metrics) error {
				once.Do(func() { close(inSend) })
				<-releaseSend
				return nil
			},
		}
		agent := NewAgent(5*time.Millisecond, 10*time.Millisecond, collector, mock)

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			agent.Run(ctx)
			close(done)
		}()

		select {
		case <-inSend:
		case <-time.After(1 * time.Second):
			t.Fatal("timed out waiting for SendBatch to start")
		}

		cancel()

		select {
		case <-done:
			t.Fatal("agent exited before in-flight SendBatch completed")
		case <-time.After(30 * time.Millisecond):
		}

		close(releaseSend)

		select {
		case <-done:
		case <-time.After(1 * time.Second):
			t.Fatal("agent did not exit after in-flight SendBatch finished")
		}
	})
}

package storage

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/astre-ash/omnigo/internal/domain"
)

var _ MemoryStorage = (*mockMemoryStorage)(nil)
var _ SnapshotWriter = (*mockSnapshotWriter)(nil)

type mockMemoryStorage struct {
	// UpdateGauge.
	updateGaugeName  string
	updateGaugeVal   float64
	updateGaugeCalls int
	updateGaugeErr   error

	// UpdateCounter.
	updateCounterName  string
	updateCounterVal   int64
	updateCounterCalls int
	updateCounterErr   error

	// UpdateBatch.
	updateBatchMetrics []domain.Metrics
	updateBatchCalls   int
	updateBatchErr     error

	// Get & GetAll.
	getGaugeVal   float64
	getGaugeErr   error
	getCounterVal int64
	getCounterErr error
	allGauges     map[string]float64
	allCounters   map[string]int64
	getAllErr     error

	// Snapshot.
	snapshotRes   []domain.Metrics
	snapshotCalls int
	snapshotErr   error
}

func (m *mockMemoryStorage) UpdateGauge(ctx context.Context, name string, value float64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.updateGaugeCalls++
	m.updateGaugeName = name
	m.updateGaugeVal = value
	return m.updateGaugeErr
}

func (m *mockMemoryStorage) UpdateCounter(ctx context.Context, name string, value int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.updateCounterCalls++
	m.updateCounterName = name
	m.updateCounterVal = value
	return m.updateCounterErr
}

func (m *mockMemoryStorage) UpdateBatch(ctx context.Context, metrics []domain.Metrics) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.updateBatchCalls++
	m.updateBatchMetrics = metrics
	return m.updateBatchErr
}

func (m *mockMemoryStorage) GetGauge(ctx context.Context, name string) (float64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return m.getGaugeVal, m.getGaugeErr
}

func (m *mockMemoryStorage) GetCounter(ctx context.Context, name string) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return m.getCounterVal, m.getCounterErr
}

func (m *mockMemoryStorage) GetAll(ctx context.Context) (map[string]float64, map[string]int64, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	return m.allGauges, m.allCounters, m.getAllErr
}

func (m *mockMemoryStorage) Snapshot(ctx context.Context) ([]domain.Metrics, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.snapshotCalls++
	return m.snapshotRes, m.snapshotErr
}

type mockSnapshotWriter struct {
	saveCalls     int
	savedSnapshot []domain.Metrics
	saveErr       error
}

func (m *mockSnapshotWriter) Save(snapshot []domain.Metrics) error {
	m.saveCalls++
	m.savedSnapshot = snapshot
	return m.saveErr
}

func TestNewSyncStorage(t *testing.T) {
	mem := &mockMemoryStorage{}
	writer := &mockSnapshotWriter{}

	syncStorage := NewSyncStorage(mem, writer)
	require.NotNil(t, syncStorage)
	assert.Equal(t, mem, syncStorage.MemoryStorage)
	assert.Equal(t, writer, syncStorage.fileManager)
}

func TestSyncStorage_UpdateGauge(t *testing.T) {
	ctx := context.Background()
	errMem := errors.New("memory error")
	errSnap := errors.New("snapshot error")
	errDisk := errors.New("disk write failure")

	val := 123.45
	expectedSnapshot := []domain.Metrics{
		{ID: "Alloc", MType: domain.TypeGauge, Value: &val},
	}

	t.Run("success: updates memory, takes snapshot and saves to disk", func(t *testing.T) {
		mem := &mockMemoryStorage{snapshotRes: expectedSnapshot}
		writer := &mockSnapshotWriter{}
		s := NewSyncStorage(mem, writer)

		err := s.UpdateGauge(ctx, "Alloc", 123.45)
		require.NoError(t, err)

		assert.Equal(t, 1, mem.updateGaugeCalls)
		assert.Equal(t, "Alloc", mem.updateGaugeName)
		assert.Equal(t, 123.45, mem.updateGaugeVal)
		assert.Equal(t, 1, mem.snapshotCalls)

		assert.Equal(t, 1, writer.saveCalls)
		assert.Equal(t, expectedSnapshot, writer.savedSnapshot)
	})

	t.Run("failure: memory update fails, snapshot and disk save not called", func(t *testing.T) {
		mem := &mockMemoryStorage{updateGaugeErr: errMem}
		writer := &mockSnapshotWriter{}
		s := NewSyncStorage(mem, writer)

		err := s.UpdateGauge(ctx, "Alloc", 100.0)
		require.ErrorIs(t, err, errMem)

		assert.Equal(t, 1, mem.updateGaugeCalls)
		assert.Equal(t, 0, mem.snapshotCalls)
		assert.Equal(t, 0, writer.saveCalls)
	})

	t.Run("failure: snapshot creation fails", func(t *testing.T) {
		mem := &mockMemoryStorage{snapshotErr: errSnap}
		writer := &mockSnapshotWriter{}
		s := NewSyncStorage(mem, writer)

		err := s.UpdateGauge(ctx, "Alloc", 123.45)
		require.Error(t, err)
		assert.ErrorIs(t, err, errSnap)
		assert.Contains(t, err.Error(), "failed to make snapshot for sync")

		assert.Equal(t, 1, mem.updateGaugeCalls)
		assert.Equal(t, 1, mem.snapshotCalls)
		assert.Equal(t, 0, writer.saveCalls)
	})

	t.Run("failure: disk save fails", func(t *testing.T) {
		mem := &mockMemoryStorage{snapshotRes: expectedSnapshot}
		writer := &mockSnapshotWriter{saveErr: errDisk}
		s := NewSyncStorage(mem, writer)

		err := s.UpdateGauge(ctx, "Alloc", 123.45)
		require.Error(t, err)
		assert.ErrorIs(t, err, errDisk)
		assert.Contains(t, err.Error(), "failed to sync metrics to disk")

		assert.Equal(t, 1, mem.updateGaugeCalls)
		assert.Equal(t, 1, mem.snapshotCalls)
		assert.Equal(t, 1, writer.saveCalls)
	})

	t.Run("failure: context canceled", func(t *testing.T) {
		mem := &mockMemoryStorage{}
		writer := &mockSnapshotWriter{}
		s := NewSyncStorage(mem, writer)

		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()

		err := s.UpdateGauge(canceledCtx, "Alloc", 100.0)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, 0, mem.updateGaugeCalls)
		assert.Equal(t, 0, writer.saveCalls)
	})
}

func TestSyncStorage_UpdateCounter(t *testing.T) {
	ctx := context.Background()
	errMem := errors.New("memory error")
	errSnap := errors.New("snapshot error")
	errDisk := errors.New("disk write failure")

	delta := int64(10)
	expectedSnapshot := []domain.Metrics{
		{ID: "PollCount", MType: domain.TypeCounter, Delta: &delta},
	}

	t.Run("success: updates memory, takes snapshot and saves to disk", func(t *testing.T) {
		mem := &mockMemoryStorage{snapshotRes: expectedSnapshot}
		writer := &mockSnapshotWriter{}
		s := NewSyncStorage(mem, writer)

		err := s.UpdateCounter(ctx, "PollCount", 10)
		require.NoError(t, err)

		assert.Equal(t, 1, mem.updateCounterCalls)
		assert.Equal(t, "PollCount", mem.updateCounterName)
		assert.Equal(t, int64(10), mem.updateCounterVal)
		assert.Equal(t, 1, mem.snapshotCalls)

		assert.Equal(t, 1, writer.saveCalls)
		assert.Equal(t, expectedSnapshot, writer.savedSnapshot)
	})

	t.Run("failure: memory update fails, disk save not called", func(t *testing.T) {
		mem := &mockMemoryStorage{updateCounterErr: errMem}
		writer := &mockSnapshotWriter{}
		s := NewSyncStorage(mem, writer)

		err := s.UpdateCounter(ctx, "PollCount", 5)
		require.ErrorIs(t, err, errMem)

		assert.Equal(t, 1, mem.updateCounterCalls)
		assert.Equal(t, 0, mem.snapshotCalls)
		assert.Equal(t, 0, writer.saveCalls)
	})

	t.Run("failure: snapshot creation fails", func(t *testing.T) {
		mem := &mockMemoryStorage{snapshotErr: errSnap}
		writer := &mockSnapshotWriter{}
		s := NewSyncStorage(mem, writer)

		err := s.UpdateCounter(ctx, "PollCount", 10)
		require.Error(t, err)
		assert.ErrorIs(t, err, errSnap)
		assert.Contains(t, err.Error(), "failed to make snapshot for sync")

		assert.Equal(t, 1, mem.updateCounterCalls)
		assert.Equal(t, 1, mem.snapshotCalls)
		assert.Equal(t, 0, writer.saveCalls)
	})

	t.Run("failure: disk save fails", func(t *testing.T) {
		mem := &mockMemoryStorage{snapshotRes: expectedSnapshot}
		writer := &mockSnapshotWriter{saveErr: errDisk}
		s := NewSyncStorage(mem, writer)

		err := s.UpdateCounter(ctx, "PollCount", 10)
		require.Error(t, err)
		assert.ErrorIs(t, err, errDisk)
		assert.Contains(t, err.Error(), "failed to sync metrics to disk")

		assert.Equal(t, 1, mem.updateCounterCalls)
		assert.Equal(t, 1, mem.snapshotCalls)
		assert.Equal(t, 1, writer.saveCalls)
	})

	t.Run("failure: context canceled", func(t *testing.T) {
		mem := &mockMemoryStorage{}
		writer := &mockSnapshotWriter{}
		s := NewSyncStorage(mem, writer)

		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()

		err := s.UpdateCounter(canceledCtx, "PollCount", 5)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, 0, mem.updateCounterCalls)
		assert.Equal(t, 0, writer.saveCalls)
	})
}

func TestSyncStorage_UpdateBatch(t *testing.T) {
	ctx := context.Background()
	errMem := errors.New("memory batch error")
	errSnap := errors.New("snapshot error")
	errDisk := errors.New("disk write failure")

	val := 42.0
	delta := int64(1)
	batch := []domain.Metrics{
		{ID: "Alloc", MType: domain.TypeGauge, Value: &val},
		{ID: "PollCount", MType: domain.TypeCounter, Delta: &delta},
	}

	t.Run("success: updates batch in memory, takes snapshot and saves to disk", func(t *testing.T) {
		mem := &mockMemoryStorage{snapshotRes: batch}
		writer := &mockSnapshotWriter{}
		s := NewSyncStorage(mem, writer)

		err := s.UpdateBatch(ctx, batch)
		require.NoError(t, err)

		assert.Equal(t, 1, mem.updateBatchCalls)
		assert.Equal(t, batch, mem.updateBatchMetrics)
		assert.Equal(t, 1, mem.snapshotCalls)

		assert.Equal(t, 1, writer.saveCalls)
		assert.Equal(t, batch, writer.savedSnapshot)
	})

	t.Run("failure: memory batch update fails", func(t *testing.T) {
		mem := &mockMemoryStorage{updateBatchErr: errMem}
		writer := &mockSnapshotWriter{}
		s := NewSyncStorage(mem, writer)

		err := s.UpdateBatch(ctx, batch)
		require.ErrorIs(t, err, errMem)

		assert.Equal(t, 1, mem.updateBatchCalls)
		assert.Equal(t, 0, mem.snapshotCalls)
		assert.Equal(t, 0, writer.saveCalls)
	})

	t.Run("failure: snapshot creation fails", func(t *testing.T) {
		mem := &mockMemoryStorage{snapshotErr: errSnap}
		writer := &mockSnapshotWriter{}
		s := NewSyncStorage(mem, writer)

		err := s.UpdateBatch(ctx, batch)
		require.Error(t, err)
		assert.ErrorIs(t, err, errSnap)
		assert.Contains(t, err.Error(), "failed to make snapshot for sync")

		assert.Equal(t, 1, mem.updateBatchCalls)
		assert.Equal(t, 1, mem.snapshotCalls)
		assert.Equal(t, 0, writer.saveCalls)
	})

	t.Run("failure: disk save fails", func(t *testing.T) {
		mem := &mockMemoryStorage{snapshotRes: batch}
		writer := &mockSnapshotWriter{saveErr: errDisk}
		s := NewSyncStorage(mem, writer)

		err := s.UpdateBatch(ctx, batch)
		require.Error(t, err)
		assert.ErrorIs(t, err, errDisk)
		assert.Contains(t, err.Error(), "failed to sync metrics to disk")

		assert.Equal(t, 1, mem.updateBatchCalls)
		assert.Equal(t, 1, mem.snapshotCalls)
		assert.Equal(t, 1, writer.saveCalls)
	})

	t.Run("failure: context canceled", func(t *testing.T) {
		mem := &mockMemoryStorage{}
		writer := &mockSnapshotWriter{}
		s := NewSyncStorage(mem, writer)

		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()

		err := s.UpdateBatch(canceledCtx, batch)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, 0, mem.updateBatchCalls)
		assert.Equal(t, 0, writer.saveCalls)
	})
}

func TestSyncStorage_EmbeddedMethodsDelegation(t *testing.T) {
	ctx := context.Background()

	expectedGauges := map[string]float64{"Alloc": 42.5}
	expectedCounters := map[string]int64{"PollCount": 7}

	mem := &mockMemoryStorage{
		getGaugeVal:   42.5,
		getCounterVal: 7,
		allGauges:     expectedGauges,
		allCounters:   expectedCounters,
	}
	writer := &mockSnapshotWriter{}
	s := NewSyncStorage(mem, writer)

	gVal, err := s.GetGauge(ctx, "Alloc")
	require.NoError(t, err)
	assert.Equal(t, 42.5, gVal)

	cVal, err := s.GetCounter(ctx, "PollCount")
	require.NoError(t, err)
	assert.Equal(t, int64(7), cVal)

	gauges, counters, err := s.GetAll(ctx)
	require.NoError(t, err)
	assert.Equal(t, expectedGauges, gauges)
	assert.Equal(t, expectedCounters, counters)
}

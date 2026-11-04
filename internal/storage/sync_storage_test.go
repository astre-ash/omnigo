package storage

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/astre-ash/omnigo/internal/domain"
)

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

	// Get & GetAll.
	getGaugeVal   float64
	getGaugeErr   error
	getCounterVal int64
	getCounterErr error
	allGauges     map[string]float64
	allCounters   map[string]int64

	// Snapshot.
	snapshotRes   []domain.Metrics
	snapshotCalls int
}

func (m *mockMemoryStorage) UpdateGauge(name string, value float64) error {
	m.updateGaugeCalls++
	m.updateGaugeName = name
	m.updateGaugeVal = value
	return m.updateGaugeErr
}

func (m *mockMemoryStorage) UpdateCounter(name string, value int64) error {
	m.updateCounterCalls++
	m.updateCounterName = name
	m.updateCounterVal = value
	return m.updateCounterErr
}

func (m *mockMemoryStorage) GetGauge(name string) (float64, error) {
	return m.getGaugeVal, m.getGaugeErr
}

func (m *mockMemoryStorage) GetCounter(name string) (int64, error) {
	return m.getCounterVal, m.getCounterErr
}

func (m *mockMemoryStorage) GetAllGauges() map[string]float64 {
	return m.allGauges
}

func (m *mockMemoryStorage) GetAllCounters() map[string]int64 {
	return m.allCounters
}

func (m *mockMemoryStorage) Snapshot() []domain.Metrics {
	m.snapshotCalls++
	return m.snapshotRes
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
	errMem := errors.New("memory error")
	errDisk := errors.New("disk write failure")

	val := 123.45
	expectedSnapshot := []domain.Metrics{
		{ID: "Alloc", MType: domain.TypeGauge, Value: &val},
	}

	t.Run("success: updates memory, takes snapshot and saves to disk", func(t *testing.T) {
		mem := &mockMemoryStorage{snapshotRes: expectedSnapshot}
		writer := &mockSnapshotWriter{}
		s := NewSyncStorage(mem, writer)

		err := s.UpdateGauge("Alloc", 123.45)
		require.NoError(t, err)

		assert.Equal(t, 1, mem.updateGaugeCalls)
		assert.Equal(t, "Alloc", mem.updateGaugeName)
		assert.Equal(t, 123.45, mem.updateGaugeVal)
		assert.Equal(t, 1, mem.snapshotCalls)

		assert.Equal(t, 1, writer.saveCalls)
		assert.Equal(t, expectedSnapshot, writer.savedSnapshot)
	})

	t.Run("failure: memory update fails, disk save not called", func(t *testing.T) {
		mem := &mockMemoryStorage{updateGaugeErr: errMem}
		writer := &mockSnapshotWriter{}
		s := NewSyncStorage(mem, writer)

		err := s.UpdateGauge("Alloc", 100.0)
		require.ErrorIs(t, err, errMem)

		assert.Equal(t, 1, mem.updateGaugeCalls)
		assert.Equal(t, 0, mem.snapshotCalls, "snapshot should not be called if memory update failed")
		assert.Equal(t, 0, writer.saveCalls, "save should not be called if memory update failed")
	})

	t.Run("failure: disk save fails", func(t *testing.T) {
		mem := &mockMemoryStorage{snapshotRes: expectedSnapshot}
		writer := &mockSnapshotWriter{saveErr: errDisk}
		s := NewSyncStorage(mem, writer)

		err := s.UpdateGauge("Alloc", 123.45)
		require.Error(t, err)
		assert.ErrorIs(t, err, errDisk)
		assert.Contains(t, err.Error(), "failed to sync gauge to disk")

		assert.Equal(t, 1, mem.updateGaugeCalls)
		assert.Equal(t, 1, mem.snapshotCalls)
		assert.Equal(t, 1, writer.saveCalls)
	})
}

func TestSyncStorage_UpdateCounter(t *testing.T) {
	errMem := errors.New("memory error")
	errDisk := errors.New("disk write failure")

	delta := int64(10)
	expectedSnapshot := []domain.Metrics{
		{ID: "PollCount", MType: domain.TypeCounter, Delta: &delta},
	}

	t.Run("success: updates memory, takes snapshot and saves to disk", func(t *testing.T) {
		mem := &mockMemoryStorage{snapshotRes: expectedSnapshot}
		writer := &mockSnapshotWriter{}
		s := NewSyncStorage(mem, writer)

		err := s.UpdateCounter("PollCount", 10)
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

		err := s.UpdateCounter("PollCount", 5)
		require.ErrorIs(t, err, errMem)

		assert.Equal(t, 1, mem.updateCounterCalls)
		assert.Equal(t, 0, mem.snapshotCalls, "snapshot should not be called if memory update failed")
		assert.Equal(t, 0, writer.saveCalls, "save should not be called if memory update failed")
	})

	t.Run("failure: disk save fails", func(t *testing.T) {
		mem := &mockMemoryStorage{snapshotRes: expectedSnapshot}
		writer := &mockSnapshotWriter{saveErr: errDisk}
		s := NewSyncStorage(mem, writer)

		err := s.UpdateCounter("PollCount", 10)
		require.Error(t, err)
		assert.ErrorIs(t, err, errDisk)
		assert.Contains(t, err.Error(), "failed to sync counter to disk")

		assert.Equal(t, 1, mem.updateCounterCalls)
		assert.Equal(t, 1, mem.snapshotCalls)
		assert.Equal(t, 1, writer.saveCalls)
	})
}

func TestSyncStorage_EmbeddedMethodsDelegation(t *testing.T) {
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

	gVal, err := s.GetGauge("Alloc")
	require.NoError(t, err)
	assert.Equal(t, 42.5, gVal)

	cVal, err := s.GetCounter("PollCount")
	require.NoError(t, err)
	assert.Equal(t, int64(7), cVal)

	assert.Equal(t, expectedGauges, s.GetAllGauges())
	assert.Equal(t, expectedCounters, s.GetAllCounters())
}

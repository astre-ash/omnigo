package storage

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/astre-ash/omnigo/internal/domain"
)

type mockPersister struct {
	mtx           sync.Mutex
	loadRes       []domain.Metrics
	loadErr       error
	loadCalls     int
	savedSnapshot []domain.Metrics
	saveErr       error
	saveCalls     int
}

func (m *mockPersister) Load() ([]domain.Metrics, error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.loadCalls++
	return m.loadRes, m.loadErr
}

func (m *mockPersister) Save(snapshot []domain.Metrics) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.saveCalls++
	m.savedSnapshot = snapshot
	return m.saveErr
}

func (m *mockPersister) getSaveCalls() int {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	return m.saveCalls
}

type mockInMemStorage struct {
	mu              sync.Mutex
	snapshotRes     []domain.Metrics
	snapshotCalls   int
	restoredMetrics []domain.Metrics
	restoreCalls    int

	updateGaugeErr   error
	updateCounterErr error
	getGaugeVal      float64
	getCounterVal    int64
	allGauges        map[string]float64
	allCounters      map[string]int64
}

func (m *mockInMemStorage) Snapshot() []domain.Metrics {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snapshotCalls++
	return m.snapshotRes
}

func (m *mockInMemStorage) Restore(metrics []domain.Metrics) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.restoreCalls++
	m.restoredMetrics = metrics
}

func (m *mockInMemStorage) UpdateGauge(name string, value float64) error {
	return m.updateGaugeErr
}

func (m *mockInMemStorage) UpdateCounter(name string, value int64) error {
	return m.updateCounterErr
}

func (m *mockInMemStorage) GetGauge(name string) (float64, error) {
	return m.getGaugeVal, nil
}

func (m *mockInMemStorage) GetCounter(name string) (int64, error) {
	return m.getCounterVal, nil
}

func (m *mockInMemStorage) GetAllGauges() map[string]float64 {
	return m.allGauges
}

func (m *mockInMemStorage) GetAllCounters() map[string]int64 {
	return m.allCounters
}

func TestNewPersistenceManager_Restore(t *testing.T) {
	testMetrics := []domain.Metrics{
		{
			ID:    "Alloc",
			MType: domain.TypeGauge,
		},
		{
			ID:    "PollCount",
			MType: domain.TypeCounter,
		},
	}

	t.Run("successfully restores metrics when restore is true and metrics exist", func(t *testing.T) {
		persister := &mockPersister{loadRes: testMetrics}
		mem := &mockInMemStorage{}

		pm, _, err := NewPersistenceManager(mem, persister, 300, true, zap.NewNop())
		require.NoError(t, err)
		require.NotNil(t, pm)

		assert.Equal(t, 1, persister.loadCalls)
		assert.Equal(t, 1, mem.restoreCalls)
		assert.Equal(t, testMetrics, mem.restoredMetrics)
	})

	t.Run("does not fail when persister.Load returns error", func(t *testing.T) {
		persister := &mockPersister{loadErr: errors.New("file corrupted")}
		mem := &mockInMemStorage{}

		pm, _, err := NewPersistenceManager(mem, persister, 300, true, zap.NewNop())
		require.NoError(t, err, "initialization should not fail on load error")
		require.NotNil(t, pm)

		assert.Equal(t, 1, persister.loadCalls)
		assert.Equal(t, 0, mem.restoreCalls, "Restore must not be called on load error")
	})

	t.Run("does not call Restore when loaded metrics slice is empty", func(t *testing.T) {
		persister := &mockPersister{loadRes: []domain.Metrics{}}
		mem := &mockInMemStorage{}

		_, _, err := NewPersistenceManager(mem, persister, 300, true, zap.NewNop())
		require.NoError(t, err)

		assert.Equal(t, 1, persister.loadCalls)
		assert.Equal(t, 0, mem.restoreCalls)
	})

	t.Run("does not load when restore is false", func(t *testing.T) {
		persister := &mockPersister{loadRes: testMetrics}
		mem := &mockInMemStorage{}

		_, _, err := NewPersistenceManager(mem, persister, 300, false, zap.NewNop())
		require.NoError(t, err)

		assert.Equal(t, 0, persister.loadCalls)
		assert.Equal(t, 0, mem.restoreCalls)
	})

	t.Run("does not panic when persister is nil and restore is true", func(t *testing.T) {
		mem := &mockInMemStorage{}

		pm, activeStorage, err := NewPersistenceManager(mem, nil, 300, true, zap.NewNop())
		require.NoError(t, err)
		assert.Equal(t, mem, activeStorage)
		assert.Nil(t, pm.persister)
	})
}

func TestNewPersistenceManager_ActiveStorageSelection(t *testing.T) {
	t.Run("returns SyncStorage when storeInterval is 0 and persister is present", func(t *testing.T) {
		persister := &mockPersister{}
		mem := &mockInMemStorage{}

		_, activeStorage, err := NewPersistenceManager(mem, persister, 0, false, zap.NewNop())
		require.NoError(t, err)

		_, ok := activeStorage.(*SyncStorage)
		assert.True(t, ok, "expected activeStorage to be *SyncStorage for synchronous mode (storeInterval=0)")
	})

	t.Run("returns original memStorage when storeInterval > 0 (async mode)", func(t *testing.T) {
		persister := &mockPersister{}
		mem := &mockInMemStorage{}

		_, activeStorage, err := NewPersistenceManager(mem, persister, 60, false, zap.NewNop())
		require.NoError(t, err)

		assert.Equal(t, mem, activeStorage, "expected activeStorage to be plain memStorage")
	})

	t.Run("returns original memStorage when storeInterval is 0 but persister is nil", func(t *testing.T) {
		mem := &mockInMemStorage{}

		_, activeStorage, err := NewPersistenceManager(mem, nil, 0, false, zap.NewNop())
		require.NoError(t, err)

		assert.Equal(t, mem, activeStorage)
	})
}

func TestPersistenceManager_Flush(t *testing.T) {
	val := 99.9
	snapshot := []domain.Metrics{
		{
			ID:    "Alloc",
			MType: domain.TypeGauge,
			Value: &val,
		},
	}

	t.Run("successfully flushes snapshot to persister", func(t *testing.T) {
		mem := &mockInMemStorage{snapshotRes: snapshot}
		persister := &mockPersister{}

		pm := &PersistenceManager{
			memStore:  mem,
			persister: persister,
			logger:    zap.NewNop(),
		}

		err := pm.Flush()
		require.NoError(t, err)

		assert.Equal(t, 1, mem.snapshotCalls)
		assert.Equal(t, 1, persister.saveCalls)
		assert.Equal(t, snapshot, persister.savedSnapshot)
	})

	t.Run("returns error when persister.Save fails", func(t *testing.T) {
		errSave := errors.New("disk is full")
		mem := &mockInMemStorage{snapshotRes: snapshot}
		persister := &mockPersister{saveErr: errSave}

		pm := &PersistenceManager{
			memStore:  mem,
			persister: persister,
			logger:    zap.NewNop(),
		}

		err := pm.Flush()
		require.Error(t, err)
		assert.ErrorIs(t, err, errSave)
		assert.Contains(t, err.Error(), "failed to flush snapshot")
	})

	t.Run("returns nil without errors when persister is nil", func(t *testing.T) {
		mem := &mockInMemStorage{snapshotRes: snapshot}

		pm := &PersistenceManager{
			memStore:  mem,
			persister: nil,
			logger:    zap.NewNop(),
		}

		err := pm.Flush()
		assert.NoError(t, err)
		assert.Equal(t, 0, mem.snapshotCalls)
	})
}

func TestPersistenceManager_StartAsyncFlush(t *testing.T) {
	t.Run("does not start goroutine when storeInterval <= 0", func(t *testing.T) {
		persister := &mockPersister{}
		pm := &PersistenceManager{
			storeInterval: 0,
			persister:     persister,
			logger:        zap.NewNop(),
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		pm.StartAsyncFlush(ctx)
		assert.Equal(t, 0, persister.getSaveCalls())
	})

	t.Run("does not start goroutine when persister is nil", func(t *testing.T) {
		pm := &PersistenceManager{
			storeInterval: 10,
			persister:     nil,
			logger:        zap.NewNop(),
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		pm.StartAsyncFlush(ctx)
	})

	t.Run("periodically dumps metrics and terminates on context cancellation", func(t *testing.T) {
		mem := &mockInMemStorage{}
		persister := &mockPersister{}

		pm := &PersistenceManager{
			memStore:      mem,
			persister:     persister,
			storeInterval: 1,
			logger:        zap.NewNop(),
		}

		ctx, cancel := context.WithCancel(context.Background())

		pm.StartAsyncFlush(ctx)

		require.Eventually(t, func() bool {
			return persister.getSaveCalls() >= 1
		}, 2*time.Second, 100*time.Millisecond, "expected async flush to trigger at least once")

		cancel()

		currentCalls := persister.getSaveCalls()
		time.Sleep(300 * time.Millisecond)
		assert.Equal(t, currentCalls, persister.getSaveCalls(), "no more flushes should happen after context cancellation")
	})
}

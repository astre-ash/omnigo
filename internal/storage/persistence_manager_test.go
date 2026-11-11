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

var _ Persister = (*mockPersister)(nil)
var _ InMemStorage = (*mockInMemStorage)(nil)

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
	mtx             sync.Mutex
	snapshotRes     []domain.Metrics
	snapshotErr     error
	snapshotCalls   int
	restoredMetrics []domain.Metrics
	restoreCalls    int
	restoreErr      error

	updateGaugeErr   error
	updateCounterErr error
	updateBatchErr   error
	getGaugeVal      float64
	getGaugeErr      error
	getCounterVal    int64
	getCounterErr    error
	allGauges        map[string]float64
	allCounters      map[string]int64
	getAllErr        error
}

func (m *mockInMemStorage) Snapshot(ctx context.Context) ([]domain.Metrics, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.snapshotCalls++
	return m.snapshotRes, m.snapshotErr
}

func (m *mockInMemStorage) Restore(ctx context.Context, metrics []domain.Metrics) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.restoreCalls++
	m.restoredMetrics = metrics
	return m.restoreErr
}

func (m *mockInMemStorage) getSnapshotCalls() int {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	return m.snapshotCalls
}

func (m *mockInMemStorage) UpdateGauge(ctx context.Context, name string, value float64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return m.updateGaugeErr
}

func (m *mockInMemStorage) UpdateCounter(ctx context.Context, name string, value int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return m.updateCounterErr
}

func (m *mockInMemStorage) UpdateBatch(ctx context.Context, metrics []domain.Metrics) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return m.updateBatchErr
}

func (m *mockInMemStorage) GetGauge(ctx context.Context, name string) (float64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return m.getGaugeVal, m.getGaugeErr
}

func (m *mockInMemStorage) GetCounter(ctx context.Context, name string) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return m.getCounterVal, m.getCounterErr
}

func (m *mockInMemStorage) GetAll(ctx context.Context) (map[string]float64, map[string]int64, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	return m.allGauges, m.allCounters, m.getAllErr
}

func TestNewPersistenceManager_Restore(t *testing.T) {
	ctx := context.Background()
	testMetrics := []domain.Metrics{
		{ID: "Alloc", MType: domain.TypeGauge},
		{ID: "PollCount", MType: domain.TypeCounter},
	}

	t.Run("successfully restores metrics when restore is true and metrics exist", func(t *testing.T) {
		persister := &mockPersister{loadRes: testMetrics}
		mem := &mockInMemStorage{}

		pm, activeStorage, err := NewPersistenceManager(ctx, mem, persister, 300*time.Second, true, zap.NewNop())
		require.NoError(t, err)
		require.NotNil(t, pm)
		assert.Equal(t, mem, activeStorage)

		assert.Equal(t, 1, persister.loadCalls)
		assert.Equal(t, 1, mem.restoreCalls)
		assert.Equal(t, testMetrics, mem.restoredMetrics)
	})

	t.Run("returns error when mem.Restore fails", func(t *testing.T) {
		restoreErr := errors.New("failed to restore to map")
		persister := &mockPersister{loadRes: testMetrics}
		mem := &mockInMemStorage{restoreErr: restoreErr}

		pm, activeStorage, err := NewPersistenceManager(ctx, mem, persister, 300*time.Second, true, zap.NewNop())
		require.Error(t, err)
		assert.ErrorIs(t, err, restoreErr)
		assert.Contains(t, err.Error(), "failed to restore metrics to memory")
		assert.Nil(t, pm)
		assert.Nil(t, activeStorage)
	})

	t.Run("does not panic or fail when persister.Load returns error (logs warning)", func(t *testing.T) {
		persister := &mockPersister{loadErr: errors.New("file corrupted")}
		mem := &mockInMemStorage{}

		pm, activeStorage, err := NewPersistenceManager(ctx, mem, persister, 300*time.Second, true, zap.NewNop())
		require.NoError(t, err)
		require.NotNil(t, pm)
		assert.Equal(t, mem, activeStorage)

		assert.Equal(t, 1, persister.loadCalls)
		assert.Equal(t, 0, mem.restoreCalls, "Restore must not be called on load error")
	})

	t.Run("does not call Restore when loaded metrics slice is empty", func(t *testing.T) {
		persister := &mockPersister{loadRes: []domain.Metrics{}}
		mem := &mockInMemStorage{}

		pm, activeStorage, err := NewPersistenceManager(ctx, mem, persister, 300*time.Second, true, zap.NewNop())
		require.NoError(t, err)
		require.NotNil(t, pm)
		assert.Equal(t, mem, activeStorage)

		assert.Equal(t, 1, persister.loadCalls)
		assert.Equal(t, 0, mem.restoreCalls)
	})

	t.Run("does not load when restore is false", func(t *testing.T) {
		persister := &mockPersister{loadRes: testMetrics}
		mem := &mockInMemStorage{}

		pm, activeStorage, err := NewPersistenceManager(ctx, mem, persister, 300*time.Second, false, zap.NewNop())
		require.NoError(t, err)
		require.NotNil(t, pm)
		assert.Equal(t, mem, activeStorage)

		assert.Equal(t, 0, persister.loadCalls)
		assert.Equal(t, 0, mem.restoreCalls)
	})

	t.Run("does not panic when persister is nil and restore is true", func(t *testing.T) {
		mem := &mockInMemStorage{}

		pm, activeStorage, err := NewPersistenceManager(ctx, mem, nil, 300*time.Second, true, zap.NewNop())
		require.NoError(t, err)
		require.NotNil(t, pm)
		assert.Equal(t, mem, activeStorage)
		assert.Nil(t, pm.persister)
	})

	t.Run("returns error when context is already canceled", func(t *testing.T) {
		mem := &mockInMemStorage{}
		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()

		pm, activeStorage, err := NewPersistenceManager(canceledCtx, mem, nil, 300*time.Second, true, zap.NewNop())
		require.ErrorIs(t, err, context.Canceled)
		assert.Nil(t, pm)
		assert.Nil(t, activeStorage)
	})
}

func TestNewPersistenceManager_ActiveStorageSelection(t *testing.T) {
	ctx := context.Background()

	t.Run("returns SyncStorage when storeInterval is 0 and persister is present", func(t *testing.T) {
		persister := &mockPersister{}
		mem := &mockInMemStorage{}

		_, activeStorage, err := NewPersistenceManager(ctx, mem, persister, 0, false, zap.NewNop())
		require.NoError(t, err)

		_, ok := activeStorage.(*SyncStorage)
		assert.True(t, ok, "expected activeStorage to be *SyncStorage for synchronous mode (storeInterval=0)")
	})

	t.Run("returns original memStorage when storeInterval > 0 (async mode)", func(t *testing.T) {
		persister := &mockPersister{}
		mem := &mockInMemStorage{}

		_, activeStorage, err := NewPersistenceManager(ctx, mem, persister, 60*time.Second, false, zap.NewNop())
		require.NoError(t, err)

		assert.Equal(t, mem, activeStorage, "expected activeStorage to be plain memStorage")
	})

	t.Run("returns original memStorage when storeInterval is 0 but persister is nil", func(t *testing.T) {
		mem := &mockInMemStorage{}

		_, activeStorage, err := NewPersistenceManager(ctx, mem, nil, 0, false, zap.NewNop())
		require.NoError(t, err)

		assert.Equal(t, mem, activeStorage)
	})
}

func TestPersistenceManager_Flush(t *testing.T) {
	ctx := context.Background()
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

		err := pm.Flush(ctx)
		require.NoError(t, err)

		assert.Equal(t, 1, mem.getSnapshotCalls())
		assert.Equal(t, 1, persister.getSaveCalls())
		assert.Equal(t, snapshot, persister.savedSnapshot)
	})

	t.Run("returns error when memStore.Snapshot fails", func(t *testing.T) {
		errSnap := errors.New("failed to acquire lock for snapshot")
		mem := &mockInMemStorage{snapshotErr: errSnap}
		persister := &mockPersister{}

		pm := &PersistenceManager{
			memStore:  mem,
			persister: persister,
			logger:    zap.NewNop(),
		}

		err := pm.Flush(ctx)
		require.Error(t, err)
		assert.ErrorIs(t, err, errSnap)
		assert.Contains(t, err.Error(), "failed to get snapshot")
		assert.Equal(t, 0, persister.getSaveCalls())
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

		err := pm.Flush(ctx)
		require.Error(t, err)
		assert.ErrorIs(t, err, errSave)
		assert.Contains(t, err.Error(), "failed to save snapshot")
	})

	t.Run("returns nil without errors when persister is nil", func(t *testing.T) {
		mem := &mockInMemStorage{snapshotRes: snapshot}

		pm := &PersistenceManager{
			memStore:  mem,
			persister: nil,
			logger:    zap.NewNop(),
		}

		err := pm.Flush(ctx)
		assert.NoError(t, err)
		assert.Equal(t, 0, mem.getSnapshotCalls())
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
		assert.Nil(t, pm.cancel)
	})

	t.Run("does not start goroutine when persister is nil", func(t *testing.T) {
		pm := &PersistenceManager{
			storeInterval: 10 * time.Millisecond,
			persister:     nil,
			logger:        zap.NewNop(),
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		pm.StartAsyncFlush(ctx)
		assert.Nil(t, pm.cancel)
	})

	t.Run("periodically dumps metrics and terminates on context cancellation", func(t *testing.T) {
		mem := &mockInMemStorage{}
		persister := &mockPersister{}

		pm := &PersistenceManager{
			memStore:      mem,
			persister:     persister,
			storeInterval: 25 * time.Millisecond,
			logger:        zap.NewNop(),
		}

		ctx, cancel := context.WithCancel(context.Background())

		pm.StartAsyncFlush(ctx)

		require.Eventually(t, func() bool {
			return persister.getSaveCalls() >= 1
		}, 1*time.Second, 10*time.Millisecond, "expected async flush to trigger at least once")

		cancel()

		currentCalls := persister.getSaveCalls()
		time.Sleep(75 * time.Millisecond)
		assert.Equal(t, currentCalls, persister.getSaveCalls(), "no more flushes should happen after context cancellation")
	})
}

func TestPersistenceManager_Stop(t *testing.T) {
	ctx := context.Background()

	t.Run("stops async flush and performs final flush", func(t *testing.T) {
		mem := &mockInMemStorage{}
		persister := &mockPersister{}

		pm := &PersistenceManager{
			memStore:      mem,
			persister:     persister,
			storeInterval: 25 * time.Millisecond,
			logger:        zap.NewNop(),
		}

		pm.StartAsyncFlush(ctx)

		require.Eventually(t, func() bool {
			return persister.getSaveCalls() >= 1
		}, 1*time.Second, 10*time.Millisecond)

		callsBeforeStop := persister.getSaveCalls()
		err := pm.Stop(ctx)
		require.NoError(t, err)

		assert.GreaterOrEqual(t, persister.getSaveCalls(), callsBeforeStop+1)
	})

	t.Run("safe to call Stop without StartAsyncFlush", func(t *testing.T) {
		mem := &mockInMemStorage{}
		persister := &mockPersister{}

		pm := &PersistenceManager{
			memStore:  mem,
			persister: persister,
			logger:    zap.NewNop(),
		}

		err := pm.Stop(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, persister.getSaveCalls())
	})
}

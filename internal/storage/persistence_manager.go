package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/astre-ash/omnigo/internal/domain"
	"github.com/astre-ash/omnigo/internal/service"
	"go.uber.org/zap"
)

type Persister interface {
	Load() ([]domain.Metrics, error)
	Save(snapshot []domain.Metrics) error
}

type InMemStorage interface {
	service.Storage
	Snapshot() []domain.Metrics
	Restore(metrics []domain.Metrics)
}

type PersistenceManager struct {
	memStore      InMemStorage
	persister     Persister
	storeInterval int
	logger        *zap.Logger
}

func NewPersistenceManager(
	mem InMemStorage,
	persister Persister,
	storeInterval int,
	restore bool,
	logger *zap.Logger,
) (*PersistenceManager, service.Storage, error) {

	pm := &PersistenceManager{
		memStore:      mem,
		persister:     persister,
		storeInterval: storeInterval,
		logger:        logger,
	}

	if restore && persister != nil {
		metrics, err := persister.Load()
		if err != nil {
			logger.Warn("failed to restore metrics from storage", zap.Error(err))
		} else if len(metrics) > 0 {
			mem.Restore(metrics)
			logger.Info("metrics restored successfully", zap.Int("count", len(metrics)))
		}
	}

	var activeStorage service.Storage = mem
	if storeInterval == 0 && persister != nil {
		activeStorage = NewSyncStorage(mem, persister)
	}

	return pm, activeStorage, nil
}

func (pm *PersistenceManager) StartAsyncFlush(ctx context.Context) {
	if pm.storeInterval <= 0 || pm.persister == nil {
		return
	}

	go func() {
		ticker := time.NewTicker(time.Duration(pm.storeInterval) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := pm.Flush(); err != nil {
					pm.logger.Error("failed to periodically dump metrics", zap.Error(err))
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (pm *PersistenceManager) Flush() error {
	if pm.persister == nil {
		return nil
	}
	if err := pm.persister.Save(pm.memStore.Snapshot()); err != nil {
		return fmt.Errorf("failed to flush snapshot: %w", err)
	}
	return nil
}

package storage

import (
	"fmt"

	"github.com/astre-ash/omnigo/internal/domain"
	"github.com/astre-ash/omnigo/internal/service"
)

type SnapshotWriter interface {
	Save(snapshot []domain.Metrics) error
}

type MemoryStorage interface {
	service.Storage
	Snapshot() []domain.Metrics
}

type SyncStorage struct {
	MemoryStorage
	fileManager SnapshotWriter
}

func NewSyncStorage(mem MemoryStorage, fm SnapshotWriter) *SyncStorage {
	return &SyncStorage{
		MemoryStorage: mem,
		fileManager:   fm,
	}
}

func (s *SyncStorage) UpdateGauge(name string, value float64) error {
	if err := s.MemoryStorage.UpdateGauge(name, value); err != nil {
		return err
	}

	if err := s.fileManager.Save(s.MemoryStorage.Snapshot()); err != nil {
		return fmt.Errorf("failed to sync gauge to disk: %w", err)
	}
	return nil
}

func (s *SyncStorage) UpdateCounter(name string, value int64) error {
	if err := s.MemoryStorage.UpdateCounter(name, value); err != nil {
		return err
	}

	if err := s.fileManager.Save(s.MemoryStorage.Snapshot()); err != nil {
		return fmt.Errorf("failed to sync counter to disk: %w", err)
	}
	return nil
}

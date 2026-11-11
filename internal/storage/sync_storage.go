package storage

import (
	"context"
	"fmt"

	"github.com/astre-ash/omnigo/internal/domain"
	"github.com/astre-ash/omnigo/internal/service"
)

type SnapshotWriter interface {
	Save(snapshot []domain.Metrics) error
}

type MemoryStorage interface {
	service.Storage
	Snapshot(ctx context.Context) ([]domain.Metrics, error)
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

func (s *SyncStorage) UpdateGauge(ctx context.Context, name string, value float64) error {
	if err := s.MemoryStorage.UpdateGauge(ctx, name, value); err != nil {
		return err
	}
	return s.sync(ctx)
}

func (s *SyncStorage) UpdateCounter(ctx context.Context, name string, value int64) error {
	if err := s.MemoryStorage.UpdateCounter(ctx, name, value); err != nil {
		return err
	}
	return s.sync(ctx)
}

func (s *SyncStorage) UpdateBatch(ctx context.Context, metrics []domain.Metrics) error {
	if err := s.MemoryStorage.UpdateBatch(ctx, metrics); err != nil {
		return err
	}
	return s.sync(ctx)
}

func (s *SyncStorage) sync(ctx context.Context) error {
	snapshot, err := s.MemoryStorage.Snapshot(ctx)
	if err != nil {
		return fmt.Errorf("failed to make snapshot for sync: %w", err)
	}

	if err := s.fileManager.Save(snapshot); err != nil {
		return fmt.Errorf("failed to sync metrics to disk: %w", err)
	}
	return nil
}

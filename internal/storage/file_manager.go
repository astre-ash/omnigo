package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/astre-ash/omnigo/internal/domain"
	"github.com/astre-ash/omnigo/pkg/retry"
)

type FileManager struct {
	filePath string
	delays   []time.Duration
}

func NewFileManager(filePath string) *FileManager {
	return &FileManager{
		filePath: filePath,
		delays:   retry.DefaultDelays(),
	}
}

func (fm *FileManager) Save(snapshot []domain.Metrics) error {
	return fm.SaveContext(context.Background(), snapshot)
}

func (fm *FileManager) SaveContext(ctx context.Context, snapshot []domain.Metrics) error {
	if fm.filePath == "" {
		return nil
	}

	return retry.Do(ctx, fm.delays, isFSRetryable, func() error {
		return fm.saveAtomic(snapshot)
	})
}

func (fm *FileManager) saveAtomic(snapshot []domain.Metrics) error {
	dir := filepath.Dir(fm.filePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	tmpFile, err := os.CreateTemp(dir, "metrics-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	var success bool
	defer func() {
		if !success {
			_ = tmpFile.Close()
			_ = os.Remove(tmpFile.Name())
		}
	}()

	enc := json.NewEncoder(tmpFile)
	enc.SetIndent("", "  ")
	if err := enc.Encode(snapshot); err != nil {
		return fmt.Errorf("failed to encode metrics: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tmpFile.Name(), fm.filePath); err != nil {
		return fmt.Errorf("failed to rename temp file to target: %w", err)
	}

	success = true
	return nil
}

func (fm *FileManager) Load() ([]domain.Metrics, error) {
	return fm.LoadContext(context.Background())
}

func (fm *FileManager) LoadContext(ctx context.Context) ([]domain.Metrics, error) {
	if fm.filePath == "" {
		return nil, nil
	}

	var metrics []domain.Metrics
	err := retry.Do(ctx, fm.delays, isFSRetryable, func() error {
		var err error
		metrics, err = fm.loadStream()
		return err
	})

	return metrics, err
}

func (fm *FileManager) loadStream() ([]domain.Metrics, error) {
	file, err := os.Open(fm.filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open storage file: %w", err)
	}
	defer file.Close()

	var metrics []domain.Metrics
	if err := json.NewDecoder(file).Decode(&metrics); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to decode metrics JSON: %w", err)
	}

	return metrics, nil
}

func isFSRetryable(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, syscall.EBUSY) ||
		errors.Is(err, syscall.EAGAIN) ||
		errors.Is(err, syscall.EINTR) ||
		errors.Is(err, syscall.ETIMEDOUT) {
		return true
	}

	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return isFSRetryable(pathErr.Err)
	}

	return false
}

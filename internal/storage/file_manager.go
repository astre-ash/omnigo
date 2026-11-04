package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/astre-ash/omnigo/internal/domain"
)

type FileManager struct {
	filePath string
}

func NewFileManager(filePath string) *FileManager {
	return &FileManager{filePath: filePath}
}

func (fm *FileManager) Save(snapshot []domain.Metrics) error {
	if fm.filePath == "" {
		return nil
	}

	dir := filepath.Dir(fm.filePath)
	if dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, "metrics-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write data to temp file: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tmpFile.Name(), fm.filePath); err != nil {
		return fmt.Errorf("failed to rename temp file to target: %w", err)
	}

	return nil
}

func (fm *FileManager) Load() ([]domain.Metrics, error) {
	if fm.filePath == "" {
		return nil, nil
	}

	data, err := os.ReadFile(fm.filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read storage file: %w", err)
	}

	if len(data) == 0 {
		return nil, nil
	}

	var metrics []domain.Metrics
	if err := json.Unmarshal(data, &metrics); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metrics JSON: %w", err)
	}

	return metrics, nil
}

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

func (fm *FileManager) Save(snapshort []domain.Metrics) error {
	if fm.filePath == "" {
		return nil
	}

	dir := filepath.Dir(fm.filePath)
	if dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	data, err := json.MarshalIndent(snapshort, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	if err := os.WriteFile(fm.filePath, data, 0666); err != nil {
		return fmt.Errorf("failed to write metrics to file: %w", err)
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

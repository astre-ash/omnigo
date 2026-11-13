package storage

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/astre-ash/omnigo/internal/domain"
	"github.com/astre-ash/omnigo/pkg/retry"
)

func toPtr[T any](v T) *T {
	return &v
}

func TestNewFileManager(t *testing.T) {
	path := "/tmp/test.json"
	fm := NewFileManager(path)
	require.NotNil(t, fm)
	assert.Equal(t, path, fm.filePath)
	assert.Equal(t, retry.DefaultDelays(), fm.delays)
}

func TestFileManager_EmptyPath(t *testing.T) {
	fm := NewFileManager("")

	t.Run("save with empty path is no-op", func(t *testing.T) {
		metrics := []domain.Metrics{
			{ID: "Alloc", MType: domain.TypeGauge, Value: toPtr(12.34)},
		}
		assert.NoError(t, fm.Save(metrics), "save must succeed as no-op when path is empty")

		canceledCtx, cancel := context.WithCancel(context.Background())
		cancel()
		assert.NoError(t, fm.SaveContext(canceledCtx, metrics))
	})

	t.Run("load with empty path returns nil", func(t *testing.T) {
		metrics, err := fm.Load()
		assert.NoError(t, err, "load must succeed as no-op when path is empty")
		assert.Nil(t, metrics)

		canceledCtx, cancel := context.WithCancel(context.Background())
		cancel()
		metrics, err = fm.LoadContext(canceledCtx)
		assert.NoError(t, err)
		assert.Nil(t, metrics)
	})
}

func TestFileManager_SaveAndLoad_Success(t *testing.T) {
	tmpDir := t.TempDir()

	filePath := filepath.Join(tmpDir, "nested", "dir", "metrics.json")
	fm := NewFileManager(filePath)

	expectedMetrics := []domain.Metrics{
		{
			ID:    "Alloc",
			MType: domain.TypeGauge,
			Value: toPtr(12345.67),
		},
		{
			ID:    "PollCount",
			MType: domain.TypeCounter,
			Delta: toPtr(int64(42)),
		},
	}

	err := fm.Save(expectedMetrics)
	require.NoError(t, err, "failed to save metrics")
	require.FileExists(t, filePath, "target file must exist after save")

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "\n  ", "file content should be indented JSON")

	actualMetrics, err := fm.Load()
	require.NoError(t, err, "failed to load metrics")
	require.Len(t, actualMetrics, 2)
	assert.Equal(t, expectedMetrics, actualMetrics, "loaded metrics must match saved metrics")

	entries, err := os.ReadDir(filepath.Dir(filePath))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "metrics.json", entries[0].Name())
}

func TestFileManager_Save_OverwritesExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "metrics.json")
	fm := NewFileManager(filePath)

	initial := []domain.Metrics{
		{ID: "Alloc", MType: domain.TypeGauge, Value: toPtr(1.0)},
	}
	require.NoError(t, fm.Save(initial))

	updated := []domain.Metrics{
		{ID: "Alloc", MType: domain.TypeGauge, Value: toPtr(99.5)},
	}
	require.NoError(t, fm.Save(updated))

	actual, err := fm.Load()
	require.NoError(t, err)
	assert.Equal(t, updated, actual)
}

func TestFileManager_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "metrics.json")
	fm := NewFileManager(filePath)

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	t.Run("SaveContext returns context.Canceled", func(t *testing.T) {
		err := fm.SaveContext(canceledCtx, []domain.Metrics{
			{ID: "Alloc", MType: domain.TypeGauge, Value: toPtr(1.0)},
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
		assert.NoFileExists(t, filePath)
	})

	t.Run("LoadContext returns context.Canceled", func(t *testing.T) {
		metrics, err := fm.LoadContext(canceledCtx)
		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Nil(t, metrics)
	})
}

func TestFileManager_Load_NonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "not_found.json")
	fm := NewFileManager(filePath)

	metrics, err := fm.Load()
	require.NoError(t, err, "non-existent file should not cause an error")
	assert.Nil(t, metrics, "metrics should be nil when file does not exist")
}

func TestFileManager_Load_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "empty.json")

	err := os.WriteFile(filePath, []byte{}, 0666)
	require.NoError(t, err)

	fm := NewFileManager(filePath)
	metrics, err := fm.Load()
	require.NoError(t, err, "empty file should not cause an error")
	assert.Nil(t, metrics, "metrics should be nil when file is empty")
}

func TestFileManager_Load_CorruptedJSON(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "corrupted.json")

	err := os.WriteFile(filePath, []byte(`{"id": "Alloc", "value": }`), 0666)
	require.NoError(t, err)

	fm := NewFileManager(filePath)
	metrics, err := fm.Load()
	require.Error(t, err, "expected error on invalid JSON")
	assert.Contains(t, err.Error(), "failed to decode metrics JSON")
	assert.Nil(t, metrics)
}

func TestFileManager_Load_Error(t *testing.T) {
	t.Run("fails to open when parent path is a regular file", func(t *testing.T) {
		tmpDir := t.TempDir()
		blockingFile := filepath.Join(tmpDir, "not_a_dir")
		require.NoError(t, os.WriteFile(blockingFile, []byte("data"), 0644))

		fm := NewFileManager(filepath.Join(blockingFile, "metrics.json"))

		metrics, err := fm.Load()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to open storage file")
		assert.Nil(t, metrics)
	})

	t.Run("fails when path is a directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		fm := NewFileManager(tmpDir)

		metrics, err := fm.Load()
		require.Error(t, err, "reading a directory path should fail")
		assert.Nil(t, metrics)
	})
}

func TestFileManager_Save_Error(t *testing.T) {
	t.Run("fails when target is an existing directory (rename fails) and cleans up temp file", func(t *testing.T) {
		tmpDir := t.TempDir()
		targetDir := filepath.Join(tmpDir, "target_dir")
		require.NoError(t, os.Mkdir(targetDir, 0755))

		fm := NewFileManager(targetDir)

		metrics := []domain.Metrics{
			{
				ID:    "Alloc",
				MType: domain.TypeGauge,
				Value: toPtr(100.0),
			},
		}

		err := fm.Save(metrics)
		require.Error(t, err, "saving over a directory path should fail")
		assert.Contains(t, err.Error(), "failed to rename temp file to target")

		entries, readErr := os.ReadDir(tmpDir)
		require.NoError(t, readErr)
		require.Len(t, entries, 1, "temp file must be removed on failure")
		assert.Equal(t, "target_dir", entries[0].Name())
	})

	t.Run("fails when directory cannot be created", func(t *testing.T) {
		tmpDir := t.TempDir()
		blockingFile := filepath.Join(tmpDir, "file_blocking_dir")
		require.NoError(t, os.WriteFile(blockingFile, []byte("blocker"), 0644))

		fm := NewFileManager(filepath.Join(blockingFile, "sub", "metrics.json"))

		err := fm.Save([]domain.Metrics{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create directory")
	})

	t.Run("fails when metrics cannot be encoded and cleans up temp file", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "metrics.json")
		fm := NewFileManager(filePath)

		nanVal := math.NaN()
		metrics := []domain.Metrics{
			{ID: "Alloc", MType: domain.TypeGauge, Value: &nanVal},
		}

		err := fm.Save(metrics)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to encode metrics")

		entries, readErr := os.ReadDir(tmpDir)
		require.NoError(t, readErr)
		assert.Empty(t, entries, "target and temp files must not remain after encode failure")
	})
}

func TestIsFSRetryable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "EBUSY",
			err:      syscall.EBUSY,
			expected: true,
		},
		{
			name:     "EAGAIN",
			err:      syscall.EAGAIN,
			expected: true,
		},
		{
			name:     "EINTR",
			err:      syscall.EINTR,
			expected: true,
		},
		{
			name:     "ETIMEDOUT",
			err:      syscall.ETIMEDOUT,
			expected: true,
		},
		{
			name:     "wrapped os.PathError with retryable errno",
			err:      fmt.Errorf("failed to open: %w", &os.PathError{Op: "open", Path: "/tmp/m.json", Err: syscall.EBUSY}),
			expected: true,
		},
		{
			name:     "os.PathError with non-retryable errno",
			err:      &os.PathError{Op: "open", Path: "/tmp/m.json", Err: syscall.EACCES},
			expected: false,
		},
		{
			name:     "os.ErrNotExist",
			err:      os.ErrNotExist,
			expected: false,
		},
		{
			name:     "generic error",
			err:      errors.New("json syntax error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isFSRetryable(tt.err))
		})
	}
}

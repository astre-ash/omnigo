package storage

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/astre-ash/omnigo/internal/domain"
)

func toPtr[T any](v T) *T {
	return &v
}

func TestNewFileManager(t *testing.T) {
	path := "/tmp/test.json"
	fm := NewFileManager(path)
	require.NotNil(t, fm)
	assert.Equal(t, path, fm.filePath)
}

func TestFileManager_EmptyPath(t *testing.T) {
	fm := NewFileManager("")

	t.Run("save with empty path is no-op", func(t *testing.T) {
		metrics := []domain.Metrics{
			{ID: "Alloc", MType: domain.TypeGauge, Value: toPtr(12.34)},
		}
		err := fm.Save(metrics)
		assert.NoError(t, err, "save must succeed as no-op when path is empty")
	})

	t.Run("load with empty path returns nil", func(t *testing.T) {
		metrics, err := fm.Load()
		assert.NoError(t, err, "load must succeed as no-op when path is empty")
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
	assert.Contains(t, err.Error(), "failed to unmarshal metrics JSON")
	assert.Nil(t, metrics)
}

func TestFileManager_Load_Error(t *testing.T) {
	tmpDir := t.TempDir()

	fm := NewFileManager(tmpDir)

	metrics, err := fm.Load()
	require.Error(t, err, "reading a directory path should fail")
	assert.Contains(t, err.Error(), "failed to read storage file")
	assert.Nil(t, metrics)
}

func TestFileManager_Save_Error(t *testing.T) {
	t.Run("fails when target is an existing directory (rename fails)", func(t *testing.T) {
		tmpDir := t.TempDir()

		fm := NewFileManager(tmpDir)

		metrics := []domain.Metrics{
			{ID: "Alloc",
				MType: domain.TypeGauge,
				Value: toPtr(100.0),
			},
		}

		err := fm.Save(metrics)
		require.Error(t, err, "saving over a directory path should fail")
		assert.Contains(t, err.Error(), "failed to rename temp file to target")
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

	t.Run("fails when metrics cannot be marshaled", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "metrics.json")
		fm := NewFileManager(filePath)

		nanVal := math.NaN()
		metrics := []domain.Metrics{
			{ID: "Alloc", MType: domain.TypeGauge, Value: &nanVal},
		}

		err := fm.Save(metrics)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to marshal metrics")
	})
}

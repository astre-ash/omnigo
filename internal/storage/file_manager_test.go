package storage

import (
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

	t.Run("write with empty path is no-op", func(t *testing.T) {
		metrics := []domain.Metrics{
			{ID: "Alloc", MType: domain.TypeGauge, Value: toPtr(12.34)},
		}
		err := fm.Save(metrics)
		assert.NoError(t, err, "write must succeed as no-op when path is empty")
	})

	t.Run("read with empty path returns nil", func(t *testing.T) {
		metrics, err := fm.Load()
		assert.NoError(t, err, "read must succeed as no-op when path is empty")
		assert.Nil(t, metrics)
	})
}

func TestFileManager_WriteAndRead_Success(t *testing.T) {
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
	require.NoError(t, err, "failed to write metrics")
	require.FileExists(t, filePath, "target file must exist after write")

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "\n  ", "file content should be indented JSON")

	actualMetrics, err := fm.Load()
	require.NoError(t, err, "failed to read metrics")
	require.Len(t, actualMetrics, 2)
	assert.Equal(t, expectedMetrics, actualMetrics, "read metrics must match written metrics")
}

func TestFileManager_Read_NonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "not_found.json")
	fm := NewFileManager(filePath)

	metrics, err := fm.Load()
	require.NoError(t, err, "non-existent file should not cause an error")
	assert.Nil(t, metrics, "metrics should be nil when file does not exist")
}

func TestFileManager_Read_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "empty.json")

	err := os.WriteFile(filePath, []byte{}, 0666)
	require.NoError(t, err)

	fm := NewFileManager(filePath)
	metrics, err := fm.Load()
	require.NoError(t, err, "empty file should not cause an error")
	assert.Nil(t, metrics, "metrics should be nil when file is empty")
}

func TestFileManager_Read_CorruptedJSON(t *testing.T) {
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

func TestFileManager_Write_Error(t *testing.T) {
	tmpDir := t.TempDir()

	fm := NewFileManager(tmpDir)

	metrics := []domain.Metrics{
		{ID: "Alloc", MType: domain.TypeGauge, Value: toPtr(100.0)},
	}

	err := fm.Save(metrics)
	require.Error(t, err, "writing to a directory path should fail")
	assert.Contains(t, err.Error(), "failed to write metrics to file")
}

func TestFileManager_Read_Error(t *testing.T) {
	tmpDir := t.TempDir()

	fm := NewFileManager(tmpDir)

	metrics, err := fm.Load()
	require.Error(t, err, "reading a directory path should fail")
	assert.Contains(t, err.Error(), "failed to read storage file")
	assert.Nil(t, metrics)
}

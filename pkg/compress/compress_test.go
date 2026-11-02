package compress

import (
	"bytes"
	"compress/gzip"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGzipCompressor(t *testing.T) {
	c := NewGzipCompressor()
	require.NotNil(t, c)
}

func TestGzipCompressor_Compress(t *testing.T) {
	compressor := NewGzipCompressor()

	t.Run("successfully compresses payload and decompresses back", func(t *testing.T) {
		originalData := []byte(`{"id":"Alloc","type":"gauge","value":123.456}`)

		compressed, err := compressor.Compress(originalData)
		require.NoError(t, err)
		require.NotEmpty(t, compressed)

		zr, err := gzip.NewReader(bytes.NewReader(compressed))
		require.NoError(t, err)
		defer zr.Close()

		decompressed, err := io.ReadAll(zr)
		require.NoError(t, err)
		assert.Equal(t, originalData, decompressed)
	})

	t.Run("successfully compresses empty slice", func(t *testing.T) {
		compressed, err := compressor.Compress([]byte{})
		require.NoError(t, err)

		require.NotEmpty(t, compressed)

		zr, err := gzip.NewReader(bytes.NewReader(compressed))
		require.NoError(t, err)
		defer zr.Close()

		decompressed, err := io.ReadAll(zr)
		require.NoError(t, err)
		assert.Empty(t, decompressed)
	})

	t.Run("effectively reduces size for compressible data", func(t *testing.T) {

		largeData := []byte(strings.Repeat("metric_value_to_compress_repeatedly_", 100))

		compressed, err := compressor.Compress(largeData)
		require.NoError(t, err)
		assert.Less(t, len(compressed), len(largeData), "compressed data must be smaller than original")
	})
}

func TestNewWriter(t *testing.T) {
	t.Run("creates valid gzip writer and writes data", func(t *testing.T) {
		var buf bytes.Buffer

		zw, err := NewWriter(&buf)
		require.NoError(t, err)
		require.NotNil(t, zw)

		testData := []byte("streaming data for gzip writer")
		_, err = zw.Write(testData)
		require.NoError(t, err)
		require.NoError(t, zw.Close())

		zr, err := gzip.NewReader(&buf)
		require.NoError(t, err)
		defer zr.Close()

		result, err := io.ReadAll(zr)
		require.NoError(t, err)
		assert.Equal(t, testData, result)
	})
}

func TestNewReader(t *testing.T) {
	t.Run("successfully creates reader for valid gzip data", func(t *testing.T) {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		_, err := zw.Write([]byte("test content"))
		require.NoError(t, err)
		require.NoError(t, zw.Close())

		zr, err := NewReader(&buf)
		require.NoError(t, err)
		require.NotNil(t, zr)
		defer zr.Close()

		content, err := io.ReadAll(zr)
		require.NoError(t, err)
		assert.Equal(t, "test content", string(content))
	})

	t.Run("returns error when stream is not valid gzip", func(t *testing.T) {
		invalidData := bytes.NewReader([]byte("not a gzip stream at all"))

		zr, err := NewReader(invalidData)
		require.Error(t, err, "NewReader must validate header and return error on invalid gzip data")
		assert.Nil(t, zr)
	})

	t.Run("returns error on empty reader", func(t *testing.T) {
		emptyData := bytes.NewReader([]byte{})

		zr, err := NewReader(emptyData)
		require.Error(t, err)
		assert.Nil(t, zr)
	})
}

func TestCompressAndDecompress_RoundTrip(t *testing.T) {
	compressor := NewGzipCompressor()
	data := []byte(`{"metrics":[{"id":"test","type":"counter","delta":1}]}`)

	compressed, err := compressor.Compress(data)
	require.NoError(t, err)

	zr, err := NewReader(bytes.NewReader(compressed))
	require.NoError(t, err)
	defer zr.Close()

	decompressed, err := io.ReadAll(zr)
	require.NoError(t, err)
	assert.Equal(t, data, decompressed)
}

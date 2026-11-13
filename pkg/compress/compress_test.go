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

func TestGetWriter_And_PutWriter(t *testing.T) {
	t.Run("gets writer from pool, writes data, and releases back", func(t *testing.T) {
		var buf bytes.Buffer

		zw := GetWriter(&buf)
		require.NotNil(t, zw)

		testData := []byte("streaming data for pooled gzip writer")
		_, err := zw.Write(testData)
		require.NoError(t, err)
		require.NoError(t, zw.Close())

		PutWriter(zw)

		zr, err := gzip.NewReader(&buf)
		require.NoError(t, err)
		defer zr.Close()

		result, err := io.ReadAll(zr)
		require.NoError(t, err)
		assert.Equal(t, testData, result)
	})

	t.Run("reuses writer from pool cleanly", func(t *testing.T) {
		var buf1 bytes.Buffer
		zw1 := GetWriter(&buf1)
		_, err := zw1.Write([]byte("first message"))
		require.NoError(t, err)
		require.NoError(t, zw1.Close())
		PutWriter(zw1)

		var buf2 bytes.Buffer
		zw2 := GetWriter(&buf2)
		_, err = zw2.Write([]byte("second message"))
		require.NoError(t, err)
		require.NoError(t, zw2.Close())
		PutWriter(zw2)

		zr, err := gzip.NewReader(&buf2)
		require.NoError(t, err)
		defer zr.Close()

		content, err := io.ReadAll(zr)
		require.NoError(t, err)
		assert.Equal(t, "second message", string(content))
	})
}

func TestGetReader_And_PutReader(t *testing.T) {
	t.Run("successfully reads valid gzip data", func(t *testing.T) {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		_, err := zw.Write([]byte("test content"))
		require.NoError(t, err)
		require.NoError(t, zw.Close())

		zr, err := GetReader(&buf)
		require.NoError(t, err)
		require.NotNil(t, zr)
		defer PutReader(zr)

		content, err := io.ReadAll(zr)
		require.NoError(t, err)
		assert.Equal(t, "test content", string(content))
	})

	t.Run("returns error when stream is not valid gzip", func(t *testing.T) {
		invalidData := bytes.NewReader([]byte("not a gzip stream at all"))

		zr, err := GetReader(invalidData)
		require.Error(t, err, "GetReader must return an error on invalid gzip header")
		assert.Nil(t, zr)
	})

	t.Run("returns error on empty reader", func(t *testing.T) {
		emptyData := bytes.NewReader([]byte{})

		zr, err := GetReader(emptyData)
		require.Error(t, err)
		assert.Nil(t, zr)
	})

	t.Run("reuses reader from pool cleanly", func(t *testing.T) {

		var buf1, buf2 bytes.Buffer

		zw1 := gzip.NewWriter(&buf1)
		_, _ = zw1.Write([]byte("message 1"))
		_ = zw1.Close()

		zw2 := gzip.NewWriter(&buf2)
		_, _ = zw2.Write([]byte("message 2"))
		_ = zw2.Close()

		zr1, err := GetReader(&buf1)
		require.NoError(t, err)
		content1, err := io.ReadAll(zr1)
		require.NoError(t, err)
		assert.Equal(t, "message 1", string(content1))
		PutReader(zr1)

		zr2, err := GetReader(&buf2)
		require.NoError(t, err)
		defer PutReader(zr2)

		content2, err := io.ReadAll(zr2)
		require.NoError(t, err)
		assert.Equal(t, "message 2", string(content2))
	})
}

func TestCompressAndDecompress_RoundTrip(t *testing.T) {
	compressor := NewGzipCompressor()
	data := []byte(`{"metrics":[{"id":"test","type":"counter","delta":1}]}`)

	compressed, err := compressor.Compress(data)
	require.NoError(t, err)

	zr, err := GetReader(bytes.NewReader(compressed))
	require.NoError(t, err)
	defer PutReader(zr)

	decompressed, err := io.ReadAll(zr)
	require.NoError(t, err)
	assert.Equal(t, data, decompressed)
}

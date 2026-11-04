package agent

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/astre-ash/omnigo/internal/domain"
)

func float64Ptr(v float64) *float64 {
	return &v
}

func int64Ptr(v int64) *int64 {
	return &v
}

func decompressGzip(r io.Reader) ([]byte, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	return io.ReadAll(zr)
}

type mockCompressor struct {
	compressFn func(data []byte) ([]byte, error)
}

func (m *mockCompressor) Compress(data []byte) ([]byte, error) {
	if m.compressFn != nil {
		return m.compressFn(data)
	}

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func TestMetricSender_Send(t *testing.T) {
	t.Run("successful request with gzip compression, JSON body, and headers", func(t *testing.T) {
		var (
			receivedMethod          string
			receivedPath            string
			receivedContentType     string
			receivedContentEncoding string
			receivedAcceptEncoding  string
			decompressedBody        []byte
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedMethod = r.Method
			receivedPath = r.URL.Path
			receivedContentType = r.Header.Get("Content-Type")
			receivedContentEncoding = r.Header.Get("Content-Encoding")
			receivedAcceptEncoding = r.Header.Get("Accept-Encoding")

			var err error
			decompressedBody, err = decompressGzip(r.Body)
			if err != nil {
				http.Error(w, "failed to decompress gzip body", http.StatusBadRequest)
				return
			}

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		sender := NewMetricSender(server.URL, &mockCompressor{})
		metric := domain.Metrics{
			ID:    "Alloc",
			MType: "gauge",
			Value: float64Ptr(123.45),
		}

		err := sender.Send(metric)
		require.NoError(t, err)

		assert.Equal(t, http.MethodPost, receivedMethod)
		assert.Equal(t, "/update/", receivedPath)
		assert.Equal(t, "application/json", receivedContentType)
		assert.Equal(t, "gzip", receivedContentEncoding)
		assert.Equal(t, "gzip", receivedAcceptEncoding)

		var actualMetric domain.Metrics
		err = json.Unmarshal(decompressedBody, &actualMetric)
		require.NoError(t, err)
		assert.Equal(t, metric, actualMetric)
	})

	t.Run("successful request without compressor sends uncompressed body", func(t *testing.T) {
		var (
			receivedMethod          string
			receivedContentType     string
			receivedContentEncoding string
			receivedAcceptEncoding  string
			receivedBody            []byte
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedMethod = r.Method
			receivedContentType = r.Header.Get("Content-Type")
			receivedContentEncoding = r.Header.Get("Content-Encoding")
			receivedAcceptEncoding = r.Header.Get("Accept-Encoding")

			var err error
			receivedBody, err = io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "failed to read body", http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		sender := NewMetricSender(server.URL, nil)
		metric := domain.Metrics{
			ID:    "Alloc",
			MType: "gauge",
			Value: float64Ptr(42.0),
		}

		err := sender.Send(metric)
		require.NoError(t, err)

		assert.Equal(t, http.MethodPost, receivedMethod)
		assert.Equal(t, "application/json", receivedContentType)
		assert.Empty(t, receivedContentEncoding, "Content-Encoding should not be set when compressor is nil")
		assert.Equal(t, "gzip", receivedAcceptEncoding)

		var actualMetric domain.Metrics
		err = json.Unmarshal(receivedBody, &actualMetric)
		require.NoError(t, err)
		assert.Equal(t, metric, actualMetric)
	})

	t.Run("returns error when compressor fails", func(t *testing.T) {
		errCompression := errors.New("compression failed")
		compressor := &mockCompressor{
			compressFn: func(data []byte) ([]byte, error) {
				return nil, errCompression
			},
		}

		sender := NewMetricSender("http://localhost:8080", compressor)
		metric := domain.Metrics{
			ID:    "Alloc",
			MType: "gauge",
			Value: float64Ptr(100),
		}

		err := sender.Send(metric)
		require.Error(t, err)
		assert.ErrorIs(t, err, errCompression)
		assert.Contains(t, err.Error(), "failed to compress metric payload")
	})

	t.Run("returns error when metric serialization fails", func(t *testing.T) {
		sender := NewMetricSender("http://localhost:8080", &mockCompressor{})
		nanVal := math.NaN()
		metric := domain.Metrics{
			ID:    "Alloc",
			MType: "gauge",
			Value: &nanVal,
		}

		err := sender.Send(metric)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to marshal metric")
	})

	t.Run("returns error when request creation fails", func(t *testing.T) {
		sender := NewMetricSender("http://localhost\x7f:8080", nil)
		metric := domain.Metrics{
			ID:    "Alloc",
			MType: "gauge",
			Value: float64Ptr(100),
		}

		err := sender.Send(metric)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create request")
	})

	t.Run("returns error when server responds with non-200 status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		sender := NewMetricSender(server.URL, &mockCompressor{})
		metric := domain.Metrics{
			ID:    "Alloc",
			MType: "gauge",
			Value: float64Ptr(100),
		}

		err := sender.Send(metric)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected status code: 400")
	})

	t.Run("returns error when server is unreachable", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		server.Close()

		sender := NewMetricSender(server.URL, &mockCompressor{})
		metric := domain.Metrics{
			ID:    "Alloc",
			MType: "gauge",
			Value: float64Ptr(100),
		}

		err := sender.Send(metric)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "request failed")
	})
}

func TestMetricSender_SendAll(t *testing.T) {
	t.Run("successful sending all metrics with gzip decompression", func(t *testing.T) {
		var (
			mtx             sync.Mutex
			receivedMetrics []domain.Metrics
			receivedPaths   []string
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mtx.Lock()
			defer mtx.Unlock()

			assert.Equal(t, "gzip", r.Header.Get("Content-Encoding"))
			receivedPaths = append(receivedPaths, r.URL.Path)

			decompressed, err := decompressGzip(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			var m domain.Metrics
			if err := json.Unmarshal(decompressed, &m); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			receivedMetrics = append(receivedMetrics, m)

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		sender := NewMetricSender(server.URL, &mockCompressor{})
		metrics := []domain.Metrics{
			{
				ID:    "Alloc",
				MType: "gauge",
				Value: float64Ptr(100.5),
			},
			{
				ID:    "PollCount",
				MType: "counter",
				Delta: int64Ptr(1),
			},
		}

		err := sender.SendAll(metrics)
		require.NoError(t, err)

		mtx.Lock()
		defer mtx.Unlock()

		require.Len(t, receivedMetrics, 2)
		assert.Equal(t, []string{"/update/", "/update/"}, receivedPaths)
		assert.Equal(t, metrics, receivedMetrics)
	})

	t.Run("aborts and returns error when a metric fails to send", func(t *testing.T) {
		var (
			mtx       sync.Mutex
			callCount int
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mtx.Lock()
			callCount++
			current := callCount
			mtx.Unlock()

			if current == 2 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		sender := NewMetricSender(server.URL, &mockCompressor{})
		metrics := []domain.Metrics{
			{ID: "Alloc", MType: "gauge", Value: float64Ptr(100.5)},
			{ID: "PollCount", MType: "counter", Delta: int64Ptr(1)},
			{ID: "Frees", MType: "gauge", Value: float64Ptr(200.0)},
		}

		err := sender.SendAll(metrics)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send metric PollCount (counter)")
		assert.Contains(t, err.Error(), "unexpected status code: 500")

		mtx.Lock()
		assert.Equal(t, 2, callCount, "should stop sending immediately on first error")
		mtx.Unlock()
	})

	t.Run("returns nil when metrics slice is empty or nil", func(t *testing.T) {
		sender := NewMetricSender("http://localhost:8080", &mockCompressor{})

		assert.NoError(t, sender.SendAll([]domain.Metrics{}))
		assert.NoError(t, sender.SendAll(nil))
	})
}

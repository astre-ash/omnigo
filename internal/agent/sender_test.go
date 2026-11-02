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

func decompressBody(r io.Reader) ([]byte, error) {
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
			decompressedBody, err = decompressBody(r.Body)
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
		require.NoError(t, err, "send should succeed when server returns 200 OK")

		assert.Equal(t, http.MethodPost, receivedMethod, "HTTP method must be POST")
		assert.Equal(t, "/update/", receivedPath, "request path mismatch")
		assert.Equal(t, "application/json", receivedContentType, "content-type header must be application/json")
		assert.Equal(t, "gzip", receivedContentEncoding, "content-encoding header must be gzip")
		assert.Equal(t, "gzip", receivedAcceptEncoding, "accept-encoding header must be gzip")

		var actualMetric domain.Metrics
		err = json.Unmarshal(decompressedBody, &actualMetric)
		require.NoError(t, err, "decompressed body should be valid JSON")
		assert.Equal(t, metric, actualMetric, "sent and received metrics must match")
	})

	t.Run("successful request without compressor sends uncompressed body", func(t *testing.T) {
		var receivedBody []byte

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		require.Error(t, err, "expected error when server responds with 400")
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
		require.Error(t, err, "expected error when server is down")
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

			zr, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			defer zr.Close()

			var m domain.Metrics
			if err := json.NewDecoder(zr).Decode(&m); err != nil {
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
		require.NoError(t, err, "SendAll should succeed without errors")

		mtx.Lock()
		defer mtx.Unlock()

		require.Len(t, receivedMetrics, 2, "server must receive exactly 2 requests")
		assert.Equal(t, []string{"/update/", "/update/"}, receivedPaths)
		assert.Equal(t, metrics, receivedMetrics, "received metrics must match sent metrics")
	})

	t.Run("returns error when one of metrics fails to send", func(t *testing.T) {
		var callCount int

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			if callCount == 2 {
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
		}

		err := sender.SendAll(metrics)
		require.Error(t, err, "SendAll should return an error if a request fails")
		assert.Contains(t, err.Error(), "failed to send metric PollCount (counter)")
		assert.Contains(t, err.Error(), "unexpected status code: 500")
	})

	t.Run("returns nil when metrics slice is empty", func(t *testing.T) {
		sender := NewMetricSender("http://localhost:8080", &mockCompressor{})
		err := sender.SendAll([]domain.Metrics{})
		assert.NoError(t, err)
	})
}

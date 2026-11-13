package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

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

type mockNetError struct {
	timeout   bool
	temporary bool
}

func (e mockNetError) Error() string {
	return "mock network error"
}

func (e mockNetError) Timeout() bool {
	return e.timeout
}

func (e mockNetError) Temporary() bool {
	return e.temporary
}

func TestMetricSender_SendBatch(t *testing.T) {
	ctx := context.Background()

	testMetrics := []domain.Metrics{
		{
			ID:    "Alloc",
			MType: domain.TypeGauge,
			Value: float64Ptr(123.45),
		},
		{
			ID:    "PollCount",
			MType: domain.TypeCounter,
			Delta: int64Ptr(10),
		},
	}

	t.Run("successful batch request with gzip compression, JSON body, and headers", func(t *testing.T) {
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

		sender := NewMetricSender(server.URL+"/", &mockCompressor{})

		err := sender.SendBatch(ctx, testMetrics)
		require.NoError(t, err)

		assert.Equal(t, http.MethodPost, receivedMethod)
		assert.Equal(t, "/updates/", receivedPath)
		assert.Equal(t, "application/json", receivedContentType)
		assert.Equal(t, "gzip", receivedContentEncoding)
		assert.Equal(t, "gzip", receivedAcceptEncoding)

		var actualMetrics []domain.Metrics
		err = json.Unmarshal(decompressedBody, &actualMetrics)
		require.NoError(t, err)
		assert.Equal(t, testMetrics, actualMetrics)
	})

	t.Run("successful batch request without compressor sends uncompressed body", func(t *testing.T) {
		var (
			receivedMethod          string
			receivedPath            string
			receivedContentType     string
			receivedContentEncoding string
			receivedAcceptEncoding  string
			receivedBody            []byte
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedMethod = r.Method
			receivedPath = r.URL.Path
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

		err := sender.SendBatch(ctx, testMetrics)
		require.NoError(t, err)

		assert.Equal(t, http.MethodPost, receivedMethod)
		assert.Equal(t, "/updates/", receivedPath)
		assert.Equal(t, "application/json", receivedContentType)
		assert.Empty(t, receivedContentEncoding, "Content-Encoding should not be set when compressor is nil")
		assert.Equal(t, "gzip", receivedAcceptEncoding)

		var actualMetrics []domain.Metrics
		err = json.Unmarshal(receivedBody, &actualMetrics)
		require.NoError(t, err)
		assert.Equal(t, testMetrics, actualMetrics)
	})

	t.Run("empty or nil batch returns nil without sending HTTP request", func(t *testing.T) {
		var serverCalled atomic.Bool
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			serverCalled.Store(true)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		sender := NewMetricSender(server.URL, &mockCompressor{})

		assert.NoError(t, sender.SendBatch(ctx, []domain.Metrics{}))
		assert.NoError(t, sender.SendBatch(ctx, nil))
		assert.False(t, serverCalled.Load(), "HTTP request must not be sent for empty metrics batch")
	})

	t.Run("returns error when compressor fails", func(t *testing.T) {
		errCompression := errors.New("compression failed")
		compressor := &mockCompressor{
			compressFn: func(data []byte) ([]byte, error) {
				return nil, errCompression
			},
		}

		sender := NewMetricSender("http://localhost:8080", compressor)

		err := sender.SendBatch(ctx, testMetrics)
		require.Error(t, err)
		assert.ErrorIs(t, err, errCompression)
		assert.Contains(t, err.Error(), "failed to prepare batch payload")
		assert.Contains(t, err.Error(), "failed to compress payload")
	})

	t.Run("returns error when metric serialization fails", func(t *testing.T) {
		sender := NewMetricSender("http://localhost:8080", &mockCompressor{})
		nanVal := math.NaN()
		invalidMetrics := []domain.Metrics{
			{
				ID:    "Alloc",
				MType: domain.TypeGauge,
				Value: &nanVal,
			},
		}

		err := sender.SendBatch(ctx, invalidMetrics)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to prepare batch payload")
		assert.Contains(t, err.Error(), "failed to marshal JSON")
	})

	t.Run("returns error without retries when request creation fails", func(t *testing.T) {
		sender := NewMetricSender("http://localhost\x7f:8080", nil)

		err := sender.SendBatch(ctx, testMetrics)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create request")
	})

	t.Run("returns error without retries when server responds with 4xx status", func(t *testing.T) {
		var attempts atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts.Add(1)
			http.Error(w, "invalid payload", http.StatusBadRequest)
		}))
		defer server.Close()

		sender := NewMetricSender(server.URL, &mockCompressor{})

		err := sender.SendBatch(ctx, testMetrics)
		require.Error(t, err)
		assert.Equal(t, int32(1), attempts.Load(), "4xx errors must not be retried")
		assert.Contains(t, err.Error(), "unexpected status 400 from /updates/: invalid payload")
	})

	t.Run("retries on 5xx server error and succeeds on subsequent attempt", func(t *testing.T) {
		var attempts atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if attempts.Add(1) == 1 {
				http.Error(w, "temporary failure", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		sender := NewMetricSender(server.URL, &mockCompressor{})

		err := sender.SendBatch(ctx, testMetrics)
		require.NoError(t, err)
		assert.Equal(t, int32(2), attempts.Load())
	})

	t.Run("stops retrying when context expires during backoff on unreachable server", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		server.Close()

		sender := NewMetricSender(server.URL, &mockCompressor{})

		shortCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		err := sender.SendBatch(shortCtx, testMetrics)
		require.Error(t, err)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("returns error on canceled context without calling server", func(t *testing.T) {
		var serverCalled atomic.Bool
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			serverCalled.Store(true)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		sender := NewMetricSender(server.URL, &mockCompressor{})

		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()

		err := sender.SendBatch(canceledCtx, testMetrics)
		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
		assert.False(t, serverCalled.Load())
	})
}

func TestMetricSender_sendRequest(t *testing.T) {
	ctx := context.Background()

	t.Run("returns error when server is unreachable", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		server.Close()

		sender := NewMetricSender(server.URL, nil)

		err := sender.sendRequest(ctx, "/updates/", []byte(`[]`), false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "http request failed")
	})

	t.Run("wraps 5xx response in serverError and limits response body to 512 bytes", func(t *testing.T) {
		longBody := strings.Repeat("x", 1024)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(longBody))
		}))
		defer server.Close()

		sender := NewMetricSender(server.URL, nil)

		err := sender.sendRequest(ctx, "/updates/", []byte(`[]`), false)
		require.Error(t, err)

		var srvErr *serverError
		assert.ErrorAs(t, err, &srvErr)
		assert.Contains(t, err.Error(), "unexpected status 503 from /updates/: "+strings.Repeat("x", 512))
		assert.NotContains(t, err.Error(), strings.Repeat("x", 513))
	})
}

func TestMetricSender_isRetryable(t *testing.T) {
	sender := NewMetricSender("http://localhost:8080", nil)

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
			name:     "serverError",
			err:      &serverError{err: errors.New("500 internal server error")},
			expected: true,
		},
		{
			name:     "wrapped serverError",
			err:      fmt.Errorf("wrapped: %w", &serverError{err: errors.New("503")}),
			expected: true,
		},
		{
			name:     "net.Error",
			err:      mockNetError{timeout: true},
			expected: true,
		},
		{
			name:     "net.OpError",
			err:      &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("dial failed")},
			expected: true,
		},
		{
			name:     "ECONNREFUSED",
			err:      syscall.ECONNREFUSED,
			expected: true,
		},
		{
			name:     "wrapped ECONNRESET",
			err:      fmt.Errorf("http request failed: %w", syscall.ECONNRESET),
			expected: true,
		},
		{
			name:     "generic non-retryable error",
			err:      errors.New("unexpected status 400"),
			expected: false,
		},
		{
			name:     "context canceled",
			err:      context.Canceled,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, sender.isRetryable(tt.err))
		})
	}
}

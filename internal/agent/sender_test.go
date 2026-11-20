package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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

type mockHTTPDoer struct {
	doFn func(req *http.Request) (*http.Response, error)
}

func (m *mockHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	return m.doFn(req)
}

func TestNewMetricSender(t *testing.T) {
	t.Run("trims trailing slashes and creates default http.Client when client is nil", func(t *testing.T) {
		sender := NewMetricSender("http://localhost:8080///", nil, nil)

		assert.Equal(t, "http://localhost:8080", sender.serverAddr)
		httpClient, ok := sender.client.(*http.Client)
		require.True(t, ok, "expected default client to be *http.Client")
		assert.Equal(t, defaultClientTimeout, httpClient.Timeout)
	})

	t.Run("uses provided custom HTTPDoer client", func(t *testing.T) {
		customClient := &mockHTTPDoer{}
		sender := NewMetricSender("http://localhost:8080", customClient, nil)

		assert.Same(t, customClient, sender.client)
	})
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
			decompressedBody        []byte
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedMethod = r.Method
			receivedPath = r.URL.Path
			receivedContentType = r.Header.Get("Content-Type")
			receivedContentEncoding = r.Header.Get("Content-Encoding")

			var err error
			decompressedBody, err = decompressGzip(r.Body)
			if err != nil {
				http.Error(w, "failed to decompress gzip body", http.StatusBadRequest)
				return
			}

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		sender := NewMetricSender(server.URL+"/", nil, &mockCompressor{})

		err := sender.SendBatch(ctx, testMetrics)
		require.NoError(t, err)

		assert.Equal(t, http.MethodPost, receivedMethod)
		assert.Equal(t, "/updates/", receivedPath)
		assert.Equal(t, "application/json", receivedContentType)
		assert.Equal(t, "gzip", receivedContentEncoding)

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
			receivedBody            []byte
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedMethod = r.Method
			receivedPath = r.URL.Path
			receivedContentType = r.Header.Get("Content-Type")
			receivedContentEncoding = r.Header.Get("Content-Encoding")

			var err error
			receivedBody, err = io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "failed to read body", http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		sender := NewMetricSender(server.URL, nil, nil)

		err := sender.SendBatch(ctx, testMetrics)
		require.NoError(t, err)

		assert.Equal(t, http.MethodPost, receivedMethod)
		assert.Equal(t, "/updates/", receivedPath)
		assert.Equal(t, "application/json", receivedContentType)
		assert.Empty(t, receivedContentEncoding, "Content-Encoding should not be set when compressor is nil")

		var actualMetrics []domain.Metrics
		err = json.Unmarshal(receivedBody, &actualMetrics)
		require.NoError(t, err)
		assert.Equal(t, testMetrics, actualMetrics)
	})

	t.Run("delegates request execution to custom HTTPDoer", func(t *testing.T) {
		var called atomic.Bool
		customClient := &mockHTTPDoer{
			doFn: func(req *http.Request) (*http.Response, error) {
				called.Store(true)
				assert.Equal(t, "http://example.com/updates/", req.URL.String())
				assert.Equal(t, http.MethodPost, req.Method)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil
			},
		}

		sender := NewMetricSender("http://example.com", customClient, nil)

		err := sender.SendBatch(ctx, testMetrics)
		require.NoError(t, err)
		assert.True(t, called.Load())
	})

	t.Run("empty or nil batch returns nil without sending HTTP request", func(t *testing.T) {
		var serverCalled atomic.Bool
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			serverCalled.Store(true)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		sender := NewMetricSender(server.URL, nil, &mockCompressor{})

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

		sender := NewMetricSender("http://localhost:8080", nil, compressor)

		err := sender.SendBatch(ctx, testMetrics)
		require.Error(t, err)
		assert.ErrorIs(t, err, errCompression)
		assert.Contains(t, err.Error(), "failed to prepare batch payload")
		assert.Contains(t, err.Error(), "failed to compress payload")
	})

	t.Run("returns error when metric serialization fails", func(t *testing.T) {
		sender := NewMetricSender("http://localhost:8080", nil, &mockCompressor{})
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

	t.Run("returns error when URL building fails", func(t *testing.T) {
		sender := NewMetricSender("http://localhost\x7f:8080", nil, nil)

		err := sender.SendBatch(ctx, testMetrics)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to build request url")
	})

	t.Run("returns error on non-200 status without retries", func(t *testing.T) {
		for _, tc := range []struct {
			name   string
			status int
			body   string
		}{
			{name: "4xx status", status: http.StatusBadRequest, body: "invalid payload"},
			{name: "5xx status", status: http.StatusInternalServerError, body: "internal failure"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				var attempts atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					attempts.Add(1)
					http.Error(w, tc.body, tc.status)
				}))
				defer server.Close()

				sender := NewMetricSender(server.URL, nil, &mockCompressor{})

				err := sender.SendBatch(ctx, testMetrics)
				require.Error(t, err)
				assert.Equal(t, int32(1), attempts.Load())
				assert.Contains(t, err.Error(), tc.body)
			})
		}
	})

	t.Run("returns error on canceled context without calling server", func(t *testing.T) {
		var serverCalled atomic.Bool
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			serverCalled.Store(true)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		sender := NewMetricSender(server.URL, nil, &mockCompressor{})

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

		sender := NewMetricSender(server.URL, nil, nil)

		err := sender.sendRequest(ctx, "/updates/", []byte(`[]`), false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "http request failed")
	})

	t.Run("returns wrapped error when HTTPDoer.Do fails", func(t *testing.T) {
		errDo := errors.New("network timeout")
		customClient := &mockHTTPDoer{
			doFn: func(req *http.Request) (*http.Response, error) {
				return nil, errDo
			},
		}

		sender := NewMetricSender("http://localhost:8080", customClient, nil)

		err := sender.sendRequest(ctx, "/updates/", []byte(`[]`), false)
		require.Error(t, err)
		assert.ErrorIs(t, err, errDo)
		assert.Contains(t, err.Error(), "http request failed")
	})

	t.Run("limits error response body to 512 bytes", func(t *testing.T) {
		longBody := strings.Repeat("x", 1024)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(longBody))
		}))
		defer server.Close()

		sender := NewMetricSender(server.URL, nil, nil)

		err := sender.sendRequest(ctx, "/updates/", []byte(`[]`), false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected status 503 from /updates/: "+strings.Repeat("x", 512))
		assert.NotContains(t, err.Error(), strings.Repeat("x", 513))
	})
}

package agent

import (
	"encoding/json"
	"io"
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

func TestMetricSender_Send(t *testing.T) {
	t.Run("successful request with correct JSON body, URL and headers", func(t *testing.T) {
		var (
			receivedMethod      string
			receivedPath        string
			receivedContentType string
			receivedBody        []byte
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedMethod = r.Method
			receivedPath = r.URL.Path
			receivedContentType = r.Header.Get("Content-Type")

			var err error
			receivedBody, err = io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
		}))

		defer server.Close()

		sender := NewMetricSender(server.URL)
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

		var actualMetric domain.Metrics
		err = json.Unmarshal(receivedBody, &actualMetric)

		require.NoError(t, err, "request body should be valid JSON")
		assert.Equal(t, metric, actualMetric, "sent and received metrics must match")
	})

	t.Run("returns error when server responds with non-200 status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		sender := NewMetricSender(server.URL)
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

		sender := NewMetricSender(server.URL)
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
	t.Run("successful sending all metrics", func(t *testing.T) {
		var (
			mtx             sync.Mutex
			receivedMetrics []domain.Metrics
			receivedPaths   []string
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mtx.Lock()
			defer mtx.Unlock()

			receivedPaths = append(receivedPaths, r.URL.Path)

			var m domain.Metrics
			if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			receivedMetrics = append(receivedMetrics, m)

			w.WriteHeader(http.StatusOK)
		}))

		defer server.Close()

		sender := NewMetricSender(server.URL)
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

		sender := NewMetricSender(server.URL)
		metrics := []domain.Metrics{
			{ID: "Alloc", MType: "gauge", Value: float64Ptr(100.5)},
			{ID: "PollCount", MType: "counter", Delta: int64Ptr(1)},
		}

		err := sender.SendAll(metrics)
		require.Error(t, err, "SendAll should return an error if a request fails")
		assert.Contains(t, err.Error(), "failed to send metric PollCount (counter)")
		assert.Contains(t, err.Error(), "unexpected status code: 500")
	})
}

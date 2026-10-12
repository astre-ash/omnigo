package agent

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetricSender_Send(t *testing.T) {
	t.Run("successful request with correct URL and headers", func(t *testing.T) {
		var (
			receivedMethod      string
			receivedPath        string
			receivedContentType string
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedMethod = r.Method
			receivedPath = r.URL.Path
			receivedContentType = r.Header.Get("Content-Type")

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		sender := NewMetricSender(server.URL)
		metric := MetricData{
			Type:  "gauge",
			Name:  "Alloc",
			Value: "123.45",
		}

		err := sender.Send(metric)
		require.NoError(t, err, "send should succeed when server returns 200 OK")

		assert.Equal(t, http.MethodPost, receivedMethod, "HTTP method must be POST")
		assert.Equal(t, "/update/gauge/Alloc/123.45", receivedPath, "request path mismatch")
		assert.Equal(t, "text/plain", receivedContentType, "content-type header must be text/plain")
	})

	t.Run("returns error when server responds with non-200 status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		sender := NewMetricSender(server.URL)
		metric := MetricData{Type: "gauge", Name: "Alloc", Value: "100"}

		err := sender.Send(metric)
		require.Error(t, err, "expected error when server responds with 400")
		assert.Contains(t, err.Error(), "unexpected status code: 400")
	})

	t.Run("returns error when server is unreachable", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		server.Close()

		sender := NewMetricSender(server.URL)
		metric := MetricData{Type: "gauge", Name: "Alloc", Value: "100"}

		err := sender.Send(metric)
		require.Error(t, err, "expected error when server is down")
		assert.Contains(t, err.Error(), "request failed")
	})
}

func TestMetricSender_SendAll(t *testing.T) {
	var (
		mtx           sync.Mutex
		receivedPaths []string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mtx.Lock()
		defer mtx.Unlock()

		receivedPaths = append(receivedPaths, r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))

	defer server.Close()

	sender := NewMetricSender(server.URL)
	metrics := []MetricData{
		{
			Type:  "gauge",
			Name:  "Alloc",
			Value: "100",
		},
		{
			Type:  "counter",
			Name:  "PollCount",
			Value: "1",
		},
	}

	sender.SendAll(metrics)

	mtx.Lock()
	defer mtx.Unlock()

	require.Len(t, receivedPaths, 2, "server must receive exactly 2 requests")
	expectedPaths := []string{
		"/update/gauge/Alloc/100",
		"/update/counter/PollCount/1",
	}

	assert.Equal(t, expectedPaths, receivedPaths, "paths received by server do not match expected order")
}

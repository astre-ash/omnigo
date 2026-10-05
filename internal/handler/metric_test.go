package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/go-chi/chi/v5"
)

type mockMetricStorage struct {
	gaugeName     string
	gaugeVal      float64
	gaugeCalled   bool
	counterName   string
	counterVal    int64
	counterCalled bool

	getGaugeVal   float64
	getGaugeOk    bool
	getCounterVal int64
	getCounterOk  bool

	allGauges   map[string]float64
	allCounters map[string]int64
}

func (m *mockMetricStorage) UpdateGauge(name string, val float64) {
	m.gaugeName = name
	m.gaugeVal = val
	m.gaugeCalled = true
}

func (m *mockMetricStorage) UpdateCounter(name string, val int64) {
	m.counterName = name
	m.counterVal = val
	m.counterCalled = true
}

func (m *mockMetricStorage) GetGauge(name string) (float64, bool) {
	return m.getGaugeVal, m.getGaugeOk
}

func (m *mockMetricStorage) GetCounter(name string) (int64, bool) {
	return m.getCounterVal, m.getCounterOk
}

func (m *mockMetricStorage) GetAllGauges() map[string]float64 {
	return m.allGauges
}

func (m *mockMetricStorage) GetAllCounters() map[string]int64 {
	return m.allCounters
}

func setupTestRouter(h *MetricHandler) http.Handler {
	r := chi.NewRouter()
	r.Post("/update/{type}/{name}/{value}", h.Update)
	r.Get("/value/{type}/{name}", h.GetValue)
	r.Get("/", h.GetAll)
	return r
}

func TestMetricHandler_Update_Success(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		validateMock func(t *testing.T, m *mockMetricStorage)
	}{
		{
			name: "valid gauge update",
			url:  "/update/gauge/alloc/123.45",
			validateMock: func(t *testing.T, m *mockMetricStorage) {
				assert.True(t, m.gaugeCalled, "expected UpdateGauge to be called")
				assert.Equal(t, "alloc", m.gaugeName, "gauge name mismatch")
				assert.Equal(t, 123.45, m.gaugeVal, "gauge value mismatch")
			},
		},
		{
			name: "valid counter update",
			url:  "/update/counter/poll/10",
			validateMock: func(t *testing.T, m *mockMetricStorage) {
				assert.True(t, m.counterCalled, "expected UpdateCounter to be called")
				assert.Equal(t, "poll", m.counterName, "counter name mismatch")
				assert.Equal(t, int64(10), m.counterVal, "counter value mismatch")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockMetricStorage{}
			handler := NewMericHandler(mock)
			router := setupTestRouter(handler)

			r := httptest.NewRequest(http.MethodPost, tt.url, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, r)

			assert.Equal(t, http.StatusOK, w.Code, "expected HTTP 200 OK")
			tt.validateMock(t, mock)
		})
	}
}

func TestMetricHandler_Update_Failures(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "unknown metric type",
			url:            "/update/unknown_type/test/100",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "unknown metric type",
		},
		{
			name:           "invalid gauge value (text)",
			url:            "/update/gauge/alloc/abc",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "invalid gauge value",
		},
		{
			name:           "invalid gauge value (NaN)",
			url:            "/update/gauge/alloc/NaN",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "invalid gauge value",
		},
		{
			name:           "invalid counter value (float)",
			url:            "/update/counter/poll/12.34",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "invalid counter value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockMetricStorage{}
			handler := NewMericHandler(mock)
			router := setupTestRouter(handler)

			req := httptest.NewRequest(http.MethodPost, tt.url, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code, "status code mismatch")
			assert.Contains(t, strings.TrimSpace(rec.Body.String()), tt.expectedBody, "error message body mismatch")

			assert.False(t, mock.gaugeCalled, "storage should not be called on error")
			assert.False(t, mock.counterCalled, "storage should not be called on error")
		})
	}
}

func TestMetricHandler_GetValue(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		setupMock      func(m *mockMetricStorage)
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "gauge found",
			url:  "/value/gauge/alloc",
			setupMock: func(m *mockMetricStorage) {
				m.getGaugeVal = 123.45
				m.getGaugeOk = true
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "123.45",
		},
		{
			name: "gauge not found",
			url:  "/value/gauge/unknown",
			setupMock: func(m *mockMetricStorage) {
				m.getGaugeOk = false
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   "metric not found",
		},
		{
			name: "counter found",
			url:  "/value/counter/poll",
			setupMock: func(m *mockMetricStorage) {
				m.getCounterVal = 42
				m.getCounterOk = true
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "42",
		},
		{
			name: "unknown metric type",
			url:  "/value/unsupported_type/test",
			setupMock: func(m *mockMetricStorage) {
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   "unknown metric type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockMetricStorage{}
			tt.setupMock(mock)

			handler := NewMericHandler(mock)
			router := setupTestRouter(handler)

			r := httptest.NewRequest(http.MethodGet, tt.url, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, r)

			assert.Equal(t, tt.expectedStatus, w.Code, "status code mismatch")
			assert.Equal(t, tt.expectedBody, strings.TrimSpace(w.Body.String()), "response body mismatch")

			if tt.expectedStatus == http.StatusOK {
				assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
			}
		})
	}
}

func TestMetricHandler_GetAll(t *testing.T) {
	mock := &mockMetricStorage{
		allGauges:   map[string]float64{"alloc": 10.5},
		allCounters: map[string]int64{"poll": 3},
	}

	handler := NewMericHandler(mock)
	router := setupTestRouter(handler)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code, "expected HTTP 200 OK")
	assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))

	body := w.Body.String()
	require.NotEmpty(t, body, "HTML body should not be empty")

	assert.Contains(t, body, "alloc", "HTML should contain gauge name")
	assert.Contains(t, body, "poll", "HTML should contain counter name")
}

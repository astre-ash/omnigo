package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/astre-ash/omnigo/internal/domain"
	"github.com/stretchr/testify/assert"
)

type mockMetricStorage struct {
	gaugeName   string
	gaugeVal    float64
	gaugeCalled bool

	counterName   string
	counterVal    int64
	counterCalled bool
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
	panic("unexpected call to GetGauge in UpdateHandler test")
}

func (m *mockMetricStorage) GetCounter(name string) (int64, bool) {
	panic("unexpected call to GetCounter in UpdateHandler test")
}

func TestUpdateHandler_Success(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		validateMock func(t *testing.T, m *mockMetricStorage)
	}{
		{
			name: "valid gauge update",
			url:  "/update/" + domain.TypeGauge + "/alloc/123.456",
			validateMock: func(t *testing.T, m *mockMetricStorage) {
				assert.True(t, m.gaugeCalled, "expected UpdateGauge to be called")
				assert.Equal(t, "alloc", m.gaugeName, "gauge metric name mismatch")
				assert.Equal(t, 123.456, m.gaugeVal, "gauge metric value mismatch")
				assert.False(t, m.counterCalled, "UpdateCounter should not be called for gauge")
			},
		},
		{
			name: "valid counter update",
			url:  "/update/" + domain.TypeCounter + "/poll_count/32",
			validateMock: func(t *testing.T, m *mockMetricStorage) {
				assert.True(t, m.counterCalled, "expected UpdateCounter to be called")
				assert.Equal(t, "poll_count", m.counterName, "counter metric name mismatch")
				assert.Equal(t, int64(32), m.counterVal, "counter metric value mismatch")
				assert.False(t, m.gaugeCalled, "UpdateGauge should not be called for counter")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockMetricStorage{}
			handler := NewUpdateHandler(mock)

			r := httptest.NewRequest(http.MethodPost, tt.url, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, r)
			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, http.StatusOK, res.StatusCode, "expected status 200 OK")

			tt.validateMock(t, mock)

		})
	}
}

func TestUpdateHandler_Failures(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		url            string
		expectedStatus int
		expectedHeader map[string]string
	}{
		{
			name:           "method GET not allowed",
			method:         http.MethodGet,
			url:            "/update/gauge/alloc/100",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedHeader: map[string]string{"Allow": http.MethodPost},
		},
		{
			name:           "missing metric value in path",
			method:         http.MethodPost,
			url:            "/update/gauge/alloc",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "wrong prefix in path",
			method:         http.MethodPost,
			url:            "/metrics/gauge/alloc/100",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "empty metric name",
			method:         http.MethodPost,
			url:            "/update/gauge//100",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "unknown metric type",
			method:         http.MethodPost,
			url:            "/update/unknown_type/alloc/100",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid gauge: text instead of float",
			method:         http.MethodPost,
			url:            "/update/" + domain.TypeGauge + "/alloc/not_a_number",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid gauge: NaN",
			method:         http.MethodPost,
			url:            "/update/" + domain.TypeGauge + "/alloc/NaN",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid gauge: +Inf",
			method:         http.MethodPost,
			url:            "/update/" + domain.TypeGauge + "/alloc/+Inf",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid counter: float instead of int",
			method:         http.MethodPost,
			url:            "/update/" + domain.TypeCounter + "/poll_count/12.34",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid counter: string value",
			method:         http.MethodPost,
			url:            "/update/" + domain.TypeCounter + "/poll_count/invalid_val",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockMetricStorage{}
			handler := NewUpdateHandler(mock)

			r := httptest.NewRequest(tt.method, tt.url, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, r)
			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, tt.expectedStatus, res.StatusCode, "status code mismatch")

			for headerKey, headerVal := range tt.expectedHeader {
				assert.Equal(t, headerVal, res.Header.Get(headerKey), "header mismatch")
			}

			assert.False(t, mock.gaugeCalled, "UpdateGauge must NOT be called on validation error")
			assert.False(t, mock.counterCalled, "UpdateCounter must NOT be called on validation error")

		})
	}

}

package handler

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/astre-ash/omnigo/internal/domain"
	"github.com/go-chi/chi/v5"
)

type mockMetricService struct {
	updateType   string
	updateName   string
	updateVal    string
	updateCalled bool
	updateErr    error

	getType   string
	getName   string
	getCalled bool
	getVal    string
	getErr    error

	allGauges   map[string]float64
	allCounters map[string]int64

	updateMetricReq    domain.Metrics
	updateMetricRes    domain.Metrics
	updateMetricErr    error
	updateMetricCalled bool

	getMetricReq    domain.Metrics
	getMetricRes    domain.Metrics
	getMetricErr    error
	getMetricCalled bool
}

func (m *mockMetricService) Update(metricType, name, valueStr string) error {
	m.updateType = metricType
	m.updateName = name
	m.updateVal = valueStr
	m.updateCalled = true
	return m.updateErr
}

func (m *mockMetricService) Get(metricType, name string) (string, error) {
	m.getType = metricType
	m.getName = name
	m.getCalled = true
	return m.getVal, m.getErr
}

func (m *mockMetricService) GetAll() (map[string]float64, map[string]int64) {
	return m.allGauges, m.allCounters
}

func (m *mockMetricService) UpdateMetric(req domain.Metrics) (domain.Metrics, error) {
	m.updateMetricReq = req
	m.updateMetricCalled = true
	return m.updateMetricRes, m.updateMetricErr
}

func (m *mockMetricService) GetMetric(req domain.Metrics) (domain.Metrics, error) {
	m.getMetricReq = req
	m.getMetricCalled = true
	return m.getMetricRes, m.getMetricErr
}

func setupTestRouter(h *MetricHandler) http.Handler {
	r := chi.NewRouter()

	r.Post("/update/{type}/{name}/{value}", h.Update)
	r.Get("/value/{type}/{name}", h.GetValue)
	r.Get("/", h.GetAll)

	r.Post("/update/", h.UpdateJSON)
	r.Post("/update", h.UpdateJSON)
	r.Post("/value/", h.GetValueJSON)
	r.Post("/value", h.GetValueJSON)

	return r
}

func float64Ptr(v float64) *float64 {
	return &v
}

func int64Ptr(v int64) *int64 {
	return &v
}

func TestMetricHandler_UpdateJSON(t *testing.T) {
	tests := []struct {
		name           string
		reqBody        string
		serviceRes     domain.Metrics
		serviceErr     error
		expectedStatus int
		expectedJSON   string
		validateMock   func(t *testing.T, m *mockMetricService)
	}{
		{
			name:    "success: gauge updated successfully",
			reqBody: `{"id":"alloc","type":"gauge","value":100.5}`,
			serviceRes: domain.Metrics{
				ID:    "alloc",
				MType: "gauge",
				Value: float64Ptr(100.5),
			},
			expectedStatus: http.StatusOK,
			expectedJSON:   `{"id":"alloc","type":"gauge","value":100.5}`,
			validateMock: func(t *testing.T, m *mockMetricService) {
				assert.True(t, m.updateMetricCalled)
				assert.Equal(t, "alloc", m.updateMetricReq.ID)
				assert.Equal(t, "gauge", m.updateMetricReq.MType)
				require.NotNil(t, m.updateMetricReq.Value)
				assert.Equal(t, 100.5, *m.updateMetricReq.Value)
			},
		},
		{
			name:    "success: counter updated successfully",
			reqBody: `{"id":"pollCount","type":"counter","delta":5}`,
			serviceRes: domain.Metrics{
				ID:    "pollCount",
				MType: "counter",
				Delta: int64Ptr(15),
			},
			expectedStatus: http.StatusOK,
			expectedJSON:   `{"id":"pollCount","type":"counter","delta":15}`,
			validateMock: func(t *testing.T, m *mockMetricService) {
				assert.True(t, m.updateMetricCalled)
				assert.Equal(t, "pollCount", m.updateMetricReq.ID)
				assert.Equal(t, "counter", m.updateMetricReq.MType)
				require.NotNil(t, m.updateMetricReq.Delta)
				assert.Equal(t, int64(5), *m.updateMetricReq.Delta)
			},
		},
		{
			name:           "failure: invalid JSON body",
			reqBody:        `{"id":"alloc", "type": }`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "failure: empty metric ID mapped to 404",
			reqBody:        `{"id":"","type":"gauge","value":100.5}`,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "failure: unknown metric type mapped to 400",
			reqBody:        `{"id":"alloc","type":"unknown","value":100}`,
			serviceErr:     domain.ErrUnknownMetricType,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "failure: invalid value mapped to 400",
			reqBody:        `{"id":"alloc","type":"gauge"}`,
			serviceErr:     domain.ErrInvalidValue,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "failure: internal server error mapped to 500",
			reqBody:        `{"id":"alloc","type":"gauge","value":100.5}`,
			serviceErr:     errors.New("db connection failure"),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockMetricService{
				updateMetricRes: tt.serviceRes,
				updateMetricErr: tt.serviceErr,
			}
			handler := NewMetricHandler(mock, zap.NewNop())
			router := setupTestRouter(handler)

			req := httptest.NewRequest(http.MethodPost, "/update/", bytes.NewBufferString(tt.reqBody))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)

			if tt.expectedStatus == http.StatusOK {
				assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
				assert.JSONEq(t, tt.expectedJSON, rec.Body.String())
			}

			if tt.validateMock != nil {
				tt.validateMock(t, mock)
			}
		})
	}
}

func TestMetricHandler_GetValueJSON(t *testing.T) {
	tests := []struct {
		name           string
		reqBody        string
		serviceRes     domain.Metrics
		serviceErr     error
		expectedStatus int
		expectedJSON   string
		validateMock   func(t *testing.T, m *mockMetricService)
	}{
		{
			name:    "success: gauge found and returned",
			reqBody: `{"id":"alloc","type":"gauge"}`,
			serviceRes: domain.Metrics{
				ID:    "alloc",
				MType: "gauge",
				Value: float64Ptr(250.75),
			},
			expectedStatus: http.StatusOK,
			expectedJSON:   `{"id":"alloc","type":"gauge","value":250.75}`,
			validateMock: func(t *testing.T, m *mockMetricService) {
				assert.True(t, m.getMetricCalled)
				assert.Equal(t, "alloc", m.getMetricReq.ID)
				assert.Equal(t, "gauge", m.getMetricReq.MType)
			},
		},
		{
			name:           "failure: metric not found mapped to 404",
			reqBody:        `{"id":"missing","type":"gauge"}`,
			serviceErr:     domain.ErrMetricNotFound,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "failure: empty metric ID mapped to 404",
			reqBody:        `{"id":"","type":"gauge"}`,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "failure: invalid JSON body mapped to 400",
			reqBody:        `{bad-json}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "failure: internal error mapped to 500",
			reqBody:        `{"id":"alloc","type":"gauge"}`,
			serviceErr:     errors.New("storage error"),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockMetricService{
				getMetricRes: tt.serviceRes,
				getMetricErr: tt.serviceErr,
			}
			handler := NewMetricHandler(mock, zap.NewNop())
			router := setupTestRouter(handler)

			req := httptest.NewRequest(http.MethodPost, "/value/", bytes.NewBufferString(tt.reqBody))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)

			if tt.expectedStatus == http.StatusOK {
				assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
				assert.JSONEq(t, tt.expectedJSON, rec.Body.String())
			}

			if tt.validateMock != nil {
				tt.validateMock(t, mock)
			}
		})
	}
}

func TestMetricHandler_Update(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		serviceErr     error
		expectedStatus int
		validateMock   func(t *testing.T, m *mockMetricService)
	}{
		{
			name:           "success: service updates metric successfully",
			url:            "/update/gauge/alloc/100.5",
			serviceErr:     nil,
			expectedStatus: http.StatusOK,
			validateMock: func(t *testing.T, m *mockMetricService) {
				assert.True(t, m.updateCalled, "service.Update must be called")
				assert.Equal(t, "gauge", m.updateType, "metric type mismatch")
				assert.Equal(t, "alloc", m.updateName, "metric name mismatch")
				assert.Equal(t, "100.5", m.updateVal, "metric value mismatch")
			},
		},
		{
			name:           "failure: invalid metric value mapped to 400",
			url:            "/update/gauge/alloc/invalid",
			serviceErr:     domain.ErrInvalidValue,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "failure: unknown metric type mapped to 400",
			url:            "/update/unknown/alloc/100",
			serviceErr:     domain.ErrUnknownMetricType,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "failure: unexpected internal error mapped to 500",
			url:            "/update/gauge/alloc/100",
			serviceErr:     errors.New("db connection lost"),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockMetricService{updateErr: tt.serviceErr}
			handler := NewMetricHandler(mock, zap.NewNop())
			router := setupTestRouter(handler)

			req := httptest.NewRequest(http.MethodPost, tt.url, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code, "status code mismatch")

			if tt.validateMock != nil {
				tt.validateMock(t, mock)
			}
		})
	}
}

func TestMetricHandler_GetValue(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		serviceVal     string
		serviceErr     error
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "success: metric found",
			url:            "/value/gauge/alloc",
			serviceVal:     "123.45",
			serviceErr:     nil,
			expectedStatus: http.StatusOK,
			expectedBody:   "123.45",
		},
		{
			name:           "failure: metric not found mapped to 404",
			url:            "/value/gauge/missing_metric",
			serviceVal:     "",
			serviceErr:     domain.ErrMetricNotFound,
			expectedStatus: http.StatusNotFound,
			expectedBody:   domain.ErrMetricNotFound.Error(),
		},
		{
			name:           "failure: unexpected error mapped to 500",
			url:            "/value/gauge/alloc",
			serviceVal:     "",
			serviceErr:     errors.New("unexpected disk failure"),
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   http.StatusText(http.StatusInternalServerError),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockMetricService{
				getVal: tt.serviceVal,
				getErr: tt.serviceErr,
			}
			handler := NewMetricHandler(mock, zap.NewNop())
			router := setupTestRouter(handler)

			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code, "status code mismatch")
			assert.Contains(t, strings.TrimSpace(rec.Body.String()), tt.expectedBody, "response body mismatch")

			if tt.expectedStatus == http.StatusOK {
				assert.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type"))
			}
		})
	}
}

func TestMetricHandler_GetAll(t *testing.T) {
	mock := &mockMetricService{
		allGauges:   map[string]float64{"alloc": 10.5},
		allCounters: map[string]int64{"poll": 3},
	}

	handler := NewMetricHandler(mock, zap.NewNop())
	router := setupTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code, "expected HTTP 200 OK")
	assert.Equal(t, "text/html; charset=utf-8", rec.Header().Get("Content-Type"))

	body := rec.Body.String()
	require.NotEmpty(t, body, "HTML body must not be empty")
	assert.Contains(t, body, "alloc", "HTML should contain gauge name")
	assert.Contains(t, body, "poll", "HTML should contain counter name")
}

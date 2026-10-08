package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/astre-ash/omnigo/internal/domain"
	"github.com/go-chi/chi/v5"
)

// mockMetricService implements MetricService for testing.
type mockMetricService struct {
	// Update
	updateType   string
	updateName   string
	updateVal    string
	updateCalled bool
	updateErr    error

	// Get
	getType   string
	getName   string
	getCalled bool
	getVal    string
	getErr    error

	// GetAll
	allGauges   map[string]float64
	allCounters map[string]int64
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

func setupTestRouter(h *MetricHandler) http.Handler {
	r := chi.NewRouter()
	r.Post("/update/{type}/{name}/{value}", h.Update)
	r.Get("/value/{type}/{name}", h.GetValue)
	r.Get("/", h.GetAll)
	return r
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
			handler := NewMetricHandler(mock)
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
			handler := NewMetricHandler(mock)
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

	handler := NewMetricHandler(mock)
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

package service

import (
	"errors"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/astre-ash/omnigo/internal/domain"
)

type mockStorage struct {
	// Spy. Stub fields for UpdateGauge.
	gaugeName      string
	gaugeVal       float64
	gaugeCalled    bool
	updateGaugeErr error

	// Spy. Stub fields for UpdateCounter.
	counterName      string
	counterVal       int64
	counterCalled    bool
	updateCounterErr error

	// Stub fields for GetGauge.
	getGaugeVal float64
	getGaugeErr error

	// Stub fields for GetCounter.
	getCounterVal int64
	getCounterErr error

	// Stub fields for GetAll.
	allGauges   map[string]float64
	allCounters map[string]int64
}

func (m *mockStorage) UpdateGauge(name string, value float64) error {
	m.gaugeName = name
	m.gaugeVal = value
	m.gaugeCalled = true
	return m.updateGaugeErr
}

func (m *mockStorage) UpdateCounter(name string, value int64) error {
	m.counterName = name
	m.counterVal = value
	m.counterCalled = true
	return m.updateCounterErr
}

func (m *mockStorage) GetGauge(name string) (float64, error) {
	return m.getGaugeVal, m.getGaugeErr
}

func (m *mockStorage) GetCounter(name string) (int64, error) {
	return m.getCounterVal, m.getCounterErr
}

func (m *mockStorage) GetAllGauges() map[string]float64 {
	return m.allGauges
}

func (m *mockStorage) GetAllCounters() map[string]int64 {
	return m.allCounters
}

func TestMetricService_UpdateMetric(t *testing.T) {
	errStorage := errors.New("storage update failed")

	t.Run("success: gauge metric updated", func(t *testing.T) {
		mock := &mockStorage{}
		svc := NewMetricService(mock)

		req := domain.Metrics{
			ID:    "alloc",
			MType: domain.TypeGauge,
			Value: float64Ptr(123.45),
		}

		res, err := svc.UpdateMetric(req)
		require.NoError(t, err)
		assert.Equal(t, req.ID, res.ID)
		assert.Equal(t, req.MType, res.MType)
		require.NotNil(t, res.Value)
		assert.Equal(t, 123.45, *res.Value)

		assert.True(t, mock.gaugeCalled)
		assert.Equal(t, "alloc", mock.gaugeName)
		assert.Equal(t, 123.45, mock.gaugeVal)
	})

	t.Run("success: counter metric updated and returned accumulated value", func(t *testing.T) {
		mock := &mockStorage{
			getCounterVal: 15,
		}
		svc := NewMetricService(mock)

		req := domain.Metrics{
			ID:    "pollCount",
			MType: domain.TypeCounter,
			Delta: int64Ptr(5),
		}

		res, err := svc.UpdateMetric(req)
		require.NoError(t, err)
		assert.Equal(t, req.ID, res.ID)
		assert.Equal(t, req.MType, res.MType)
		require.NotNil(t, res.Delta)

		assert.Equal(t, int64(15), *res.Delta)

		assert.True(t, mock.counterCalled)
		assert.Equal(t, "pollCount", mock.counterName)
		assert.Equal(t, int64(5), mock.counterVal)
	})

	t.Run("failure: gauge validation errors", func(t *testing.T) {
		cases := []struct {
			name  string
			value *float64
		}{
			{name: "nil value", value: nil},
			{name: "NaN value", value: float64Ptr(math.NaN())},
			{name: "+Inf value", value: float64Ptr(math.Inf(1))},
			{name: "-Inf value", value: float64Ptr(math.Inf(-1))},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				mock := &mockStorage{}
				svc := NewMetricService(mock)

				_, err := svc.UpdateMetric(domain.Metrics{
					ID:    "alloc",
					MType: domain.TypeGauge,
					Value: tc.value,
				})

				assert.ErrorIs(t, err, domain.ErrInvalidValue)
				assert.False(t, mock.gaugeCalled)
			})
		}
	})

	t.Run("failure: counter validation error (nil delta)", func(t *testing.T) {
		mock := &mockStorage{}
		svc := NewMetricService(mock)

		_, err := svc.UpdateMetric(domain.Metrics{
			ID:    "pollCount",
			MType: domain.TypeCounter,
			Delta: nil,
		})

		assert.ErrorIs(t, err, domain.ErrInvalidValue)
		assert.False(t, mock.counterCalled)
	})

	t.Run("failure: unknown metric type", func(t *testing.T) {
		mock := &mockStorage{}
		svc := NewMetricService(mock)

		_, err := svc.UpdateMetric(domain.Metrics{
			ID:    "temp",
			MType: "histogram",
		})

		assert.ErrorIs(t, err, domain.ErrUnknownMetricType)
	})

	t.Run("failure: storage update error on gauge", func(t *testing.T) {
		mock := &mockStorage{updateGaugeErr: errStorage}
		svc := NewMetricService(mock)

		_, err := svc.UpdateMetric(domain.Metrics{
			ID:    "alloc",
			MType: domain.TypeGauge,
			Value: float64Ptr(10.0),
		})

		assert.ErrorIs(t, err, errStorage)
	})

	t.Run("failure: storage get error on counter after update", func(t *testing.T) {
		mock := &mockStorage{getCounterErr: errors.New("read error")}
		svc := NewMetricService(mock)

		_, err := svc.UpdateMetric(domain.Metrics{
			ID:    "pollCount",
			MType: domain.TypeCounter,
			Delta: int64Ptr(5),
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "read error")
	})
}

func TestMetricService_GetMetric(t *testing.T) {
	errStorageFailure := errors.New("storage read failed")

	t.Run("success: gauge metric found and filled", func(t *testing.T) {
		mock := &mockStorage{getGaugeVal: 123.456}
		svc := NewMetricService(mock)

		res, err := svc.GetMetric(domain.Metrics{ID: "alloc", MType: domain.TypeGauge})
		require.NoError(t, err)
		assert.Equal(t, "alloc", res.ID)
		assert.Equal(t, domain.TypeGauge, res.MType)
		require.NotNil(t, res.Value)
		assert.Equal(t, 123.456, *res.Value)
	})

	t.Run("success: counter metric found and filled", func(t *testing.T) {
		mock := &mockStorage{getCounterVal: 42}
		svc := NewMetricService(mock)

		res, err := svc.GetMetric(domain.Metrics{ID: "pollCount", MType: domain.TypeCounter})
		require.NoError(t, err)
		assert.Equal(t, "pollCount", res.ID)
		assert.Equal(t, domain.TypeCounter, res.MType)
		require.NotNil(t, res.Delta)
		assert.Equal(t, int64(42), *res.Delta)
	})

	t.Run("failure: storage error on gauge", func(t *testing.T) {
		mock := &mockStorage{getGaugeErr: errStorageFailure}
		svc := NewMetricService(mock)

		_, err := svc.GetMetric(domain.Metrics{ID: "alloc", MType: domain.TypeGauge})
		assert.ErrorIs(t, err, errStorageFailure)
	})

	t.Run("failure: storage error on counter", func(t *testing.T) {
		mock := &mockStorage{getCounterErr: errStorageFailure}
		svc := NewMetricService(mock)

		_, err := svc.GetMetric(domain.Metrics{ID: "pollCount", MType: domain.TypeCounter})
		assert.ErrorIs(t, err, errStorageFailure)
	})

	t.Run("failure: unknown metric type", func(t *testing.T) {
		mock := &mockStorage{}
		svc := NewMetricService(mock)

		_, err := svc.GetMetric(domain.Metrics{ID: "test", MType: "unknown"})
		assert.ErrorIs(t, err, domain.ErrUnknownMetricType)
	})
}

func TestMetricService_Update(t *testing.T) {
	tests := []struct {
		name         string
		metricType   string
		metricName   string
		metricValue  string
		expectedErr  error
		validateMock func(t *testing.T, m *mockStorage)
	}{
		{
			name:        "success: valid gauge with float",
			metricType:  domain.TypeGauge,
			metricName:  "alloc",
			metricValue: "123.456",
			expectedErr: nil,
			validateMock: func(t *testing.T, m *mockStorage) {
				assert.True(t, m.gaugeCalled, "UpdateGauge must be called")
				assert.Equal(t, "alloc", m.gaugeName)
				assert.Equal(t, 123.456, m.gaugeVal)
				assert.False(t, m.counterCalled)
			},
		},
		{
			name:        "success: valid gauge with negative float",
			metricType:  domain.TypeGauge,
			metricName:  "temperature",
			metricValue: "-15.5",
			expectedErr: nil,
			validateMock: func(t *testing.T, m *mockStorage) {
				assert.True(t, m.gaugeCalled)
				assert.Equal(t, -15.5, m.gaugeVal)
			},
		},
		{
			name:        "success: valid counter with int",
			metricType:  domain.TypeCounter,
			metricName:  "poll_count",
			metricValue: "42",
			expectedErr: nil,
			validateMock: func(t *testing.T, m *mockStorage) {
				assert.True(t, m.counterCalled, "UpdateCounter must be called")
				assert.Equal(t, "poll_count", m.counterName)
				assert.Equal(t, int64(42), m.counterVal)
				assert.False(t, m.gaugeCalled)
			},
		},
		{
			name:        "failure: gauge with text value",
			metricType:  domain.TypeGauge,
			metricName:  "alloc",
			metricValue: "not_a_number",
			expectedErr: domain.ErrInvalidValue,
		},
		{
			name:        "failure: gauge with NaN",
			metricType:  domain.TypeGauge,
			metricName:  "alloc",
			metricValue: "NaN",
			expectedErr: domain.ErrInvalidValue,
		},
		{
			name:        "failure: gauge with +Inf",
			metricType:  domain.TypeGauge,
			metricName:  "alloc",
			metricValue: "+Inf",
			expectedErr: domain.ErrInvalidValue,
		},
		{
			name:        "failure: gauge with -Inf",
			metricType:  domain.TypeGauge,
			metricName:  "alloc",
			metricValue: "-Inf",
			expectedErr: domain.ErrInvalidValue,
		},
		{
			name:        "failure: counter with float value",
			metricType:  domain.TypeCounter,
			metricName:  "poll_count",
			metricValue: "12.34",
			expectedErr: domain.ErrInvalidValue,
		},
		{
			name:        "failure: counter with text value",
			metricType:  domain.TypeCounter,
			metricName:  "poll_count",
			metricValue: "invalid",
			expectedErr: domain.ErrInvalidValue,
		},
		{
			name:        "failure: unknown metric type",
			metricType:  "histogram",
			metricName:  "response_time",
			metricValue: "100",
			expectedErr: domain.ErrUnknownMetricType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockStorage{}
			svc := NewMetricService(mock)

			err := svc.Update(tt.metricType, tt.metricName, tt.metricValue)

			if tt.expectedErr != nil {
				assert.ErrorIs(t, err, tt.expectedErr, "returned error does not match expected sentinel error")
				assert.False(t, mock.gaugeCalled, "UpdateGauge must not be called on validation error")
				assert.False(t, mock.counterCalled, "UpdateCounter must not be called on validation error")
			} else {
				assert.NoError(t, err, "expected no error for valid input")
				if tt.validateMock != nil {
					tt.validateMock(t, mock)
				}
			}
		})
	}
}

func TestMetricService_Get(t *testing.T) {
	errStorageFailure := errors.New("storage read failed")

	tests := []struct {
		name        string
		metricType  string
		metricName  string
		setupMock   func(m *mockStorage)
		expectedVal string
		expectedErr error
	}{
		{
			name:       "success: gauge found and formatted",
			metricType: domain.TypeGauge,
			metricName: "alloc",
			setupMock: func(m *mockStorage) {
				m.getGaugeVal = 123.456
				m.getGaugeErr = nil
			},
			expectedVal: "123.456",
			expectedErr: nil,
		},
		{
			name:       "failure: gauge storage returns error",
			metricType: domain.TypeGauge,
			metricName: "alloc",
			setupMock: func(m *mockStorage) {
				m.getGaugeErr = errStorageFailure
			},
			expectedVal: "",
			expectedErr: errStorageFailure,
		},
		{
			name:       "success: counter found and formatted",
			metricType: domain.TypeCounter,
			metricName: "poll_count",
			setupMock: func(m *mockStorage) {
				m.getCounterVal = 42
				m.getCounterErr = nil
			},
			expectedVal: "42",
			expectedErr: nil,
		},
		{
			name:       "failure: counter storage returns error",
			metricType: domain.TypeCounter,
			metricName: "poll_count",
			setupMock: func(m *mockStorage) {
				m.getCounterErr = errStorageFailure
			},
			expectedVal: "",
			expectedErr: errStorageFailure,
		},
		{
			name:        "failure: unknown metric type",
			metricType:  "unsupported",
			metricName:  "test",
			setupMock:   func(m *mockStorage) {},
			expectedVal: "",
			expectedErr: domain.ErrUnknownMetricType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockStorage{}
			tt.setupMock(mock)

			svc := NewMetricService(mock)
			val, err := svc.Get(tt.metricType, tt.metricName)

			if tt.expectedErr != nil {
				assert.ErrorIs(t, err, tt.expectedErr, "expected matching error")
				assert.Empty(t, val, "value must be empty on error")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedVal, val, "formatted value mismatch")
			}
		})
	}
}

func TestMetricService_GetAll(t *testing.T) {
	expectedGauges := map[string]float64{"alloc": 10.5, "free": 20.0}
	expectedCounters := map[string]int64{"poll": 5}

	mock := &mockStorage{
		allGauges:   expectedGauges,
		allCounters: expectedCounters,
	}

	svc := NewMetricService(mock)
	gauges, counters := svc.GetAll()

	assert.Equal(t, expectedGauges, gauges, "gauges map mismatch")
	assert.Equal(t, expectedCounters, counters, "counters map mismatch")
}

func float64Ptr(v float64) *float64 {
	return &v
}

func int64Ptr(v int64) *int64 {
	return &v
}

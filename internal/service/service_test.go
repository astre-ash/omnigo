package service

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/astre-ash/omnigo/internal/domain"
)

type mockStorage struct {
	// Spy fields for Updates.
	gaugeName   string
	gaugeVal    float64
	gaugeCalled bool

	counterName   string
	counterVal    int64
	counterCalled bool

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

func (m *mockStorage) UpdateGauge(name string, value float64) {
	m.gaugeName = name
	m.gaugeVal = value
	m.gaugeCalled = true
}

func (m *mockStorage) UpdateCounter(name string, value int64) {
	m.counterName = name
	m.counterVal = value
	m.counterCalled = true
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

func TestMetricService_Update(t *testing.T) {
	tests := []struct {
		name         string
		metricType   string
		metricName   string
		metricValue  string
		expectedErr  error
		validateMock func(t *testing.T, m *mockStorage)
	}{
		// Success cases.
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

		// Gauge validation errors.
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

		// Counter validation errors.
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

		// Unknown metric type.
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

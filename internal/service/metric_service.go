package service

import (
	"context"
	"fmt"
	"math"
	"strconv"

	"github.com/astre-ash/omnigo/internal/domain"
)

type Storage interface {
	UpdateGauge(ctx context.Context, name string, value float64) error
	UpdateCounter(ctx context.Context, name string, value int64) error
	GetGauge(ctx context.Context, name string) (float64, error)
	GetCounter(ctx context.Context, name string) (int64, error)

	GetAll(ctx context.Context) (map[string]float64, map[string]int64, error)
	UpdateBatch(ctx context.Context, metrics []domain.Metrics) error
}

type MetricService struct {
	storage Storage
}

func NewMetricService(storage Storage) *MetricService {
	return &MetricService{storage: storage}
}

func (s *MetricService) GetMetric(ctx context.Context, m domain.Metrics) (domain.Metrics, error) {
	switch m.MType {
	case domain.TypeGauge:
		val, err := s.storage.GetGauge(ctx, m.ID)
		if err != nil {
			return domain.Metrics{}, err
		}
		m.Value = &val
		return m, nil

	case domain.TypeCounter:
		val, err := s.storage.GetCounter(ctx, m.ID)
		if err != nil {
			return domain.Metrics{}, err
		}
		m.Delta = &val
		return m, nil

	default:
		return domain.Metrics{}, fmt.Errorf("%w: '%s'", domain.ErrUnknownMetricType, m.MType)
	}
}

func (s *MetricService) UpdateMetric(ctx context.Context, m domain.Metrics) (domain.Metrics, error) {
	switch m.MType {
	case domain.TypeGauge:
		if m.Value == nil || math.IsNaN(*m.Value) || math.IsInf(*m.Value, 0) {
			return domain.Metrics{}, domain.ErrInvalidValue
		}
		if err := s.storage.UpdateGauge(ctx, m.ID, *m.Value); err != nil {
			return domain.Metrics{}, fmt.Errorf("storage update failed: %w", err)
		}
		return m, nil

	case domain.TypeCounter:
		if m.Delta == nil {
			return domain.Metrics{}, domain.ErrInvalidValue
		}

		if err := s.storage.UpdateCounter(ctx, m.ID, *m.Delta); err != nil {
			return domain.Metrics{}, fmt.Errorf("storage update failed: %w", err)
		}

		actualVal, err := s.storage.GetCounter(ctx, m.ID)
		if err != nil {
			return domain.Metrics{}, err
		}

		m.Delta = &actualVal
		return m, nil

	default:
		return domain.Metrics{}, fmt.Errorf("%w: '%s'", domain.ErrUnknownMetricType, m.MType)
	}
}

func (s *MetricService) Update(ctx context.Context, metricType, name, valueStr string) error {
	switch metricType {
	case domain.TypeGauge:
		val, err := strconv.ParseFloat(valueStr, 64)
		if err != nil || math.IsNaN(val) || math.IsInf(val, 0) {
			return fmt.Errorf("%w: invalid gauge value '%s'", domain.ErrInvalidValue, valueStr)
		}
		return s.storage.UpdateGauge(ctx, name, val)

	case domain.TypeCounter:
		val, err := strconv.ParseInt(valueStr, 10, 64)
		if err != nil {
			return fmt.Errorf("%w: invalid counter value '%s'", domain.ErrInvalidValue, valueStr)
		}
		return s.storage.UpdateCounter(ctx, name, val)

	default:
		return fmt.Errorf("%w: '%s'", domain.ErrUnknownMetricType, metricType)
	}
}

func (s *MetricService) Get(ctx context.Context, metricType, name string) (string, error) {
	switch metricType {
	case domain.TypeGauge:
		val, err := s.storage.GetGauge(ctx, name)
		if err != nil {
			return "", err
		}
		return strconv.FormatFloat(val, 'f', -1, 64), nil

	case domain.TypeCounter:
		val, err := s.storage.GetCounter(ctx, name)
		if err != nil {
			return "", err
		}
		return strconv.FormatInt(val, 10), nil

	default:
		return "", fmt.Errorf("%w: '%s'", domain.ErrUnknownMetricType, metricType)
	}
}

func (s *MetricService) GetAll(ctx context.Context) (map[string]float64, map[string]int64, error) {
	return s.storage.GetAll(ctx)

}

func (s *MetricService) UpdateMetrics(ctx context.Context, metrics []domain.Metrics) error {
	if len(metrics) == 0 {
		return nil
	}

	for _, m := range metrics {
		if err := m.Validate(); err != nil {
			return fmt.Errorf("%w: metric ID '%s'", err, m.ID)
		}
	}

	if err := s.storage.UpdateBatch(ctx, metrics); err != nil {
		return fmt.Errorf("failed to update metrics batch: %w", err)
	}

	return nil
}

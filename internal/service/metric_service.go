package service

import (
	"fmt"
	"math"
	"strconv"

	"github.com/astre-ash/omnigo/internal/domain"
)

type Storage interface {
	UpdateGauge(name string, value float64)
	UpdateCounter(name string, value int64)
	GetGauge(name string) (float64, error)
	GetCounter(name string) (int64, error)
	GetAllGauges() map[string]float64
	GetAllCounters() map[string]int64
}

type MetricService struct {
	storage Storage
}

func NewMetricService(storage Storage) *MetricService {
	return &MetricService{storage: storage}

}

func (s *MetricService) GetMetric(m domain.Metrics) (domain.Metrics, error) {
	switch m.MType {
	case domain.TypeGauge:
		val, err := s.storage.GetGauge(m.ID)
		if err != nil {
			return domain.Metrics{}, err
		}
		m.Value = &val
		return m, nil

	case domain.TypeCounter:
		val, err := s.storage.GetCounter(m.ID)
		if err != nil {
			return domain.Metrics{}, err
		}
		m.Delta = &val
		return m, nil

	default:
		return domain.Metrics{}, fmt.Errorf("%w: '%s'", domain.ErrUnknownMetricType, m.MType)
	}
}

func (s *MetricService) UpdateMetric(m domain.Metrics) (domain.Metrics, error) {
	switch m.MType {
	case domain.TypeGauge:
		if m.Value == nil || math.IsNaN(*m.Value) || math.IsInf(*m.Value, 0) {
			return domain.Metrics{}, domain.ErrInvalidValue
		}
		s.storage.UpdateGauge(m.ID, *m.Value)
		return m, nil

	case domain.TypeCounter:
		if m.Delta == nil {
			return domain.Metrics{}, domain.ErrInvalidValue
		}

		s.storage.UpdateCounter(m.ID, *m.Delta)

		actualVal, err := s.storage.GetCounter(m.ID)
		if err != nil {
			return domain.Metrics{}, err
		}

		m.Delta = &actualVal
		return m, nil

	default:
		return domain.Metrics{}, fmt.Errorf("%w: '%s'", domain.ErrUnknownMetricType, m.MType)

	}

}

func (s *MetricService) Update(metricType, name, valueStr string) error {
	switch metricType {
	case domain.TypeGauge:
		val, err := strconv.ParseFloat(valueStr, 64)
		if err != nil || math.IsNaN(val) || math.IsInf(val, 0) {
			return fmt.Errorf("%w: invalid gauge value '%s'", domain.ErrInvalidValue, valueStr)
		}
		s.storage.UpdateGauge(name, val)

	case domain.TypeCounter:
		val, err := strconv.ParseInt(valueStr, 10, 64)
		if err != nil {
			return fmt.Errorf("%w: invalid counter value '%s'", domain.ErrInvalidValue, valueStr)
		}
		s.storage.UpdateCounter(name, val)

	default:
		return fmt.Errorf("%w: '%s'", domain.ErrUnknownMetricType, metricType)
	}

	return nil
}

func (s *MetricService) Get(metricType, name string) (string, error) {
	switch metricType {
	case domain.TypeGauge:
		val, err := s.storage.GetGauge(name)
		if err != nil {
			return "", err
		}
		return strconv.FormatFloat(val, 'f', -1, 64), nil

	case domain.TypeCounter:
		val, err := s.storage.GetCounter(name)
		if err != nil {
			return "", err
		}
		return strconv.FormatInt(val, 10), nil

	default:
		return "", fmt.Errorf("%w: '%s'", domain.ErrUnknownMetricType, metricType)
	}
}

func (s *MetricService) GetAll() (map[string]float64, map[string]int64) {
	return s.storage.GetAllGauges(), s.storage.GetAllCounters()
}

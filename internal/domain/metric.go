package domain

import (
	"errors"
	"math"
)

const (
	TypeGauge   = "gauge"
	TypeCounter = "counter"
)

var (
	ErrUnknownMetricType = errors.New("unknown metric type")
	ErrMetricNotFound    = errors.New("metric not found")
	ErrInvalidValue      = errors.New("invalid metric value")
)

type Metrics struct {
	ID    string   `json:"id"`
	MType string   `json:"type"`
	Delta *int64   `json:"delta,omitempty"`
	Value *float64 `json:"value,omitempty"`
}

func (m Metrics) Validate() error {
	if m.ID == "" {
		return ErrInvalidValue
	}
	switch m.MType {
	case TypeGauge:
		if m.Value == nil || math.IsNaN(*m.Value) || math.IsInf(*m.Value, 0) {
			return ErrInvalidValue
		}
	case TypeCounter:
		if m.Delta == nil {
			return ErrInvalidValue
		}
	default:
		return ErrUnknownMetricType
	}
	return nil
}

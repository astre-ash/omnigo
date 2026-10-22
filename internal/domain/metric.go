package domain

import "errors"

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

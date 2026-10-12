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

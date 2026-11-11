package storage

import (
	"context"
	"sync"

	"github.com/astre-ash/omnigo/internal/domain"
)

type MemStorage struct {
	mtx      sync.Mutex
	gauges   map[string]float64
	counters map[string]int64
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

func (s *MemStorage) UpdateGauge(ctx context.Context, name string, value float64) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.gauges[name] = value
	return nil
}

func (s *MemStorage) UpdateCounter(ctx context.Context, name string, value int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.counters[name] += value
	return nil
}

func (s *MemStorage) GetGauge(ctx context.Context, name string) (float64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	val, ok := s.gauges[name]
	if !ok {
		return 0, domain.ErrMetricNotFound
	}
	return val, nil
}

func (s *MemStorage) GetCounter(ctx context.Context, name string) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	val, ok := s.counters[name]
	if !ok {
		return 0, domain.ErrMetricNotFound
	}
	return val, nil
}

func (s *MemStorage) GetAll(ctx context.Context) (map[string]float64, map[string]int64, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	gauges := make(map[string]float64, len(s.gauges))
	for k, v := range s.gauges {
		gauges[k] = v
	}

	counters := make(map[string]int64, len(s.counters))
	for k, v := range s.counters {
		counters[k] = v
	}

	return gauges, counters, nil
}

func (s *MemStorage) UpdateBatch(ctx context.Context, metrics []domain.Metrics) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	for _, m := range metrics {
		switch m.MType {
		case domain.TypeGauge:
			if m.Value != nil {
				s.gauges[m.ID] = *m.Value
			}
		case domain.TypeCounter:
			if m.Delta != nil {
				s.counters[m.ID] += *m.Delta
			}
		}
	}
	return nil
}

func (s *MemStorage) Snapshot(ctx context.Context) ([]domain.Metrics, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	metrics := make([]domain.Metrics, 0, len(s.gauges)+len(s.counters))
	for name, val := range s.gauges {
		v := val
		metrics = append(metrics, domain.Metrics{
			ID:    name,
			MType: domain.TypeGauge,
			Value: &v,
		})
	}
	for name, delta := range s.counters {
		d := delta
		metrics = append(metrics, domain.Metrics{
			ID:    name,
			MType: domain.TypeCounter,
			Delta: &d,
		})
	}
	return metrics, nil
}

func (s *MemStorage) Restore(ctx context.Context, metrics []domain.Metrics) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	for _, m := range metrics {
		switch m.MType {
		case domain.TypeGauge:
			if m.Value != nil {
				s.gauges[m.ID] = *m.Value
			}
		case domain.TypeCounter:
			if m.Delta != nil {
				s.counters[m.ID] = *m.Delta
			}
		}
	}
	return nil
}

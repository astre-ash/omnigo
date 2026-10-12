package storage

import (
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

func (s *MemStorage) UpdateGauge(name string, value float64) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.gauges[name] = value
}

func (s *MemStorage) UpdateCounter(name string, value int64) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.counters[name] += value
}

func (s *MemStorage) GetGauge(name string) (float64, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	val, ok := s.gauges[name]
	if !ok {
		return 0, domain.ErrMetricNotFound
	}
	return val, nil
}

func (s *MemStorage) GetCounter(name string) (int64, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	val, ok := s.counters[name]
	if !ok {
		return 0, domain.ErrMetricNotFound
	}
	return val, nil
}

func (s *MemStorage) GetAllGauges() map[string]float64 {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	copyMap := make(map[string]float64, len(s.gauges))
	for k, v := range s.gauges {
		copyMap[k] = v
	}
	return copyMap
}

func (s *MemStorage) GetAllCounters() map[string]int64 {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	copyMap := make(map[string]int64, len(s.counters))
	for k, v := range s.counters {
		copyMap[k] = v
	}
	return copyMap
}

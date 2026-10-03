package storage

import (
	"sync"
)

type MemStorage struct {
	mtx      sync.RWMutex
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

func (s *MemStorage) GetGauge(name string) (float64, bool) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	val, ok := s.gauges[name]
	return val, ok
}

func (s *MemStorage) GetCounter(name string) (int64, bool) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	val, ok := s.counters[name]
	return val, ok
}

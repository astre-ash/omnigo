package agent

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"runtime"
	"sync"

	"github.com/astre-ash/omnigo/internal/domain"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

type MetricSource interface {
	Name() string
	Poll(ctx context.Context) error
	Collect() (metrics []domain.Metrics, ack func(ok bool))
}

func gaugeMetric(name string, v float64) domain.Metrics {
	return domain.Metrics{
		ID:    name,
		MType: domain.TypeGauge,
		Value: new(v),
	}
}

func counterMetric(name string, d int64) domain.Metrics {
	return domain.Metrics{
		ID:    name,
		MType: domain.TypeCounter,
		Delta: new(d),
	}
}

type RuntimeSource struct {
	mtx         sync.Mutex
	pollCount   int64
	randomValue float64
	memStats    runtime.MemStats
	polled      bool
}

func NewRuntimeSource() *RuntimeSource {
	return &RuntimeSource{}
}

func (s *RuntimeSource) Name() string {
	return "runtime"
}

func (s *RuntimeSource) Poll(_ context.Context) error {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.memStats = ms
	s.pollCount++
	s.randomValue = rand.Float64()
	s.polled = true
	return nil
}

func (s *RuntimeSource) Collect() ([]domain.Metrics, func(bool)) {
	s.mtx.Lock()
	if !s.polled {
		s.mtx.Unlock()
		return nil, func(bool) {}
	}

	ms := s.memStats
	rnd := s.randomValue
	delta := s.pollCount

	s.pollCount = 0
	s.mtx.Unlock()

	metrics := []domain.Metrics{
		gaugeMetric("Alloc", float64(ms.Alloc)),
		gaugeMetric("BuckHashSys", float64(ms.BuckHashSys)),
		gaugeMetric("Frees", float64(ms.Frees)),
		gaugeMetric("GCCPUFraction", ms.GCCPUFraction),
		gaugeMetric("GCSys", float64(ms.GCSys)),
		gaugeMetric("HeapAlloc", float64(ms.HeapAlloc)),
		gaugeMetric("HeapIdle", float64(ms.HeapIdle)),
		gaugeMetric("HeapInuse", float64(ms.HeapInuse)),
		gaugeMetric("HeapObjects", float64(ms.HeapObjects)),
		gaugeMetric("HeapReleased", float64(ms.HeapReleased)),
		gaugeMetric("HeapSys", float64(ms.HeapSys)),
		gaugeMetric("LastGC", float64(ms.LastGC)),
		gaugeMetric("Lookups", float64(ms.Lookups)),
		gaugeMetric("MCacheInuse", float64(ms.MCacheInuse)),
		gaugeMetric("MCacheSys", float64(ms.MCacheSys)),
		gaugeMetric("MSpanInuse", float64(ms.MSpanInuse)),
		gaugeMetric("MSpanSys", float64(ms.MSpanSys)),
		gaugeMetric("Mallocs", float64(ms.Mallocs)),
		gaugeMetric("NextGC", float64(ms.NextGC)),
		gaugeMetric("NumForcedGC", float64(ms.NumForcedGC)),
		gaugeMetric("NumGC", float64(ms.NumGC)),
		gaugeMetric("OtherSys", float64(ms.OtherSys)),
		gaugeMetric("PauseTotalNs", float64(ms.PauseTotalNs)),
		gaugeMetric("StackInuse", float64(ms.StackInuse)),
		gaugeMetric("StackSys", float64(ms.StackSys)),
		gaugeMetric("Sys", float64(ms.Sys)),
		gaugeMetric("TotalAlloc", float64(ms.TotalAlloc)),
		gaugeMetric("RandomValue", rnd),
		counterMetric("PollCount", delta),
	}

	ack := func(ok bool) {
		if !ok && delta > 0 {
			s.mtx.Lock()
			s.pollCount += delta
			s.mtx.Unlock()
		}
	}

	return metrics, ack
}

type SystemReader interface {
	VirtualMemory(ctx context.Context) (total, free uint64, err error)
	CPUPercent(ctx context.Context) ([]float64, error)
}

type GopsutilReader struct{}

func (GopsutilReader) VirtualMemory(ctx context.Context) (uint64, uint64, error) {
	v, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("virtual memory: %w", err)
	}

	return v.Total, v.Free, nil
}

func (GopsutilReader) CPUPercent(ctx context.Context) ([]float64, error) {
	percents, err := cpu.PercentWithContext(ctx, 0, true)
	if err != nil {
		return nil, fmt.Errorf("cpu percent: %w", err)
	}

	return percents, nil
}

type SystemSource struct {
	reader SystemReader

	mtx         sync.RWMutex
	totalMemory float64
	freeMemory  float64
	hasMemory   bool
	cpuPercents []float64
}

func NewSystemSource(reader SystemReader) *SystemSource {
	if reader == nil {
		reader = GopsutilReader{}
	}
	return &SystemSource{
		reader: reader,
	}
}

func (s *SystemSource) Name() string {
	return "system"
}

func (s *SystemSource) Poll(ctx context.Context) error {
	total, free, memErr := s.reader.VirtualMemory(ctx)
	cpus, cpuErr := s.reader.CPUPercent(ctx)

	s.mtx.Lock()
	if memErr == nil {
		s.totalMemory = float64(total)
		s.freeMemory = float64(free)
		s.hasMemory = true
	}

	if cpuErr == nil {
		s.cpuPercents = append(s.cpuPercents[:0], cpus...)
	}
	s.mtx.Unlock()

	return errors.Join(memErr, cpuErr)
}

func (s *SystemSource) Collect() ([]domain.Metrics, func(bool)) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	if !s.hasMemory && len(s.cpuPercents) == 0 {
		return nil, func(bool) {}
	}

	metrics := make([]domain.Metrics, 0, 2+len(s.cpuPercents))
	if s.hasMemory {
		metrics = append(metrics,
			gaugeMetric("TotalMemory", s.totalMemory),
			gaugeMetric("FreeMemory", s.freeMemory),
		)
	}
	for i, pct := range s.cpuPercents {
		metrics = append(metrics, gaugeMetric(fmt.Sprintf("CPUutilization%d", i+1), pct))
	}

	return metrics, func(bool) {}

}

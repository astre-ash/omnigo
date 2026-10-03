package agent

import (
	"math/rand/v2"
	"runtime"
	"sync"

	"github.com/astre-ash/omnigo/internal/domain"
)

type MetricData struct {
	Type  string
	Name  string
	Value string
}

type Collector struct {
	mtx         sync.RWMutex
	pollCount   int64
	randomValue float64
	memStats    runtime.MemStats
}

func NewCollector() *Collector {
	return &Collector{}
}

func (c *Collector) Poll() {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	runtime.ReadMemStats(&c.memStats)
	c.pollCount++
	c.randomValue = rand.Float64()
}

func (c *Collector) GetMetrics() []MetricData {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	metrics := []MetricData{
		// Gauge runtime metrics (27 items).
		{Type: domain.TypeGauge, Name: "Alloc", Value: formatFloat(float64(c.memStats.Alloc))},
		{Type: domain.TypeGauge, Name: "BuckHashSys", Value: formatFloat(float64(c.memStats.BuckHashSys))},
		{Type: domain.TypeGauge, Name: "Frees", Value: formatFloat(float64(c.memStats.Frees))},
		{Type: domain.TypeGauge, Name: "GCCPUFraction", Value: formatFloat(c.memStats.GCCPUFraction)},
		{Type: domain.TypeGauge, Name: "GCSys", Value: formatFloat(float64(c.memStats.GCSys))},
		{Type: domain.TypeGauge, Name: "HeapAlloc", Value: formatFloat(float64(c.memStats.HeapAlloc))},
		{Type: domain.TypeGauge, Name: "HeapIdle", Value: formatFloat(float64(c.memStats.HeapIdle))},
		{Type: domain.TypeGauge, Name: "HeapInuse", Value: formatFloat(float64(c.memStats.HeapInuse))},
		{Type: domain.TypeGauge, Name: "HeapObjects", Value: formatFloat(float64(c.memStats.HeapObjects))},
		{Type: domain.TypeGauge, Name: "HeapReleased", Value: formatFloat(float64(c.memStats.HeapReleased))},
		{Type: domain.TypeGauge, Name: "HeapSys", Value: formatFloat(float64(c.memStats.HeapSys))},
		{Type: domain.TypeGauge, Name: "LastGC", Value: formatFloat(float64(c.memStats.LastGC))},
		{Type: domain.TypeGauge, Name: "Lookups", Value: formatFloat(float64(c.memStats.Lookups))},
		{Type: domain.TypeGauge, Name: "MCacheInuse", Value: formatFloat(float64(c.memStats.MCacheInuse))},
		{Type: domain.TypeGauge, Name: "MCacheSys", Value: formatFloat(float64(c.memStats.MCacheSys))},
		{Type: domain.TypeGauge, Name: "MSpanInuse", Value: formatFloat(float64(c.memStats.MSpanInuse))},
		{Type: domain.TypeGauge, Name: "MSpanSys", Value: formatFloat(float64(c.memStats.MSpanSys))},
		{Type: domain.TypeGauge, Name: "Mallocs", Value: formatFloat(float64(c.memStats.Mallocs))},
		{Type: domain.TypeGauge, Name: "NextGC", Value: formatFloat(float64(c.memStats.NextGC))},
		{Type: domain.TypeGauge, Name: "NumForcedGC", Value: formatFloat(float64(c.memStats.NumForcedGC))},
		{Type: domain.TypeGauge, Name: "NumGC", Value: formatFloat(float64(c.memStats.NumGC))},
		{Type: domain.TypeGauge, Name: "OtherSys", Value: formatFloat(float64(c.memStats.OtherSys))},
		{Type: domain.TypeGauge, Name: "PauseTotalNs", Value: formatFloat(float64(c.memStats.PauseTotalNs))},
		{Type: domain.TypeGauge, Name: "StackInuse", Value: formatFloat(float64(c.memStats.StackInuse))},
		{Type: domain.TypeGauge, Name: "StackSys", Value: formatFloat(float64(c.memStats.StackSys))},
		{Type: domain.TypeGauge, Name: "Sys", Value: formatFloat(float64(c.memStats.Sys))},
		{Type: domain.TypeGauge, Name: "TotalAlloc", Value: formatFloat(float64(c.memStats.TotalAlloc))},

		// Custom metrics.
		{Type: domain.TypeGauge, Name: "RandomValue", Value: formatFloat(c.randomValue)},
		{Type: domain.TypeCounter, Name: "PollCount", Value: formatInt(c.pollCount)},
	}

	return metrics
}

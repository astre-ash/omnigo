package agent

import (
	"math/rand/v2"
	"runtime"
	"sync"

	"github.com/astre-ash/omnigo/internal/domain"
)

type Collector struct {
	mtx         sync.Mutex
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

func (c *Collector) GetMetrics() []domain.Metrics {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	metrics := []domain.Metrics{
		// Gauge runtime metrics (27 items).
		gaugeMetric("Alloc", float64(c.memStats.Alloc)),
		gaugeMetric("BuckHashSys", float64(c.memStats.BuckHashSys)),
		gaugeMetric("Frees", float64(c.memStats.Frees)),
		gaugeMetric("GCCPUFraction", c.memStats.GCCPUFraction),
		gaugeMetric("GCSys", float64(c.memStats.GCSys)),
		gaugeMetric("HeapAlloc", float64(c.memStats.HeapAlloc)),
		gaugeMetric("HeapIdle", float64(c.memStats.HeapIdle)),
		gaugeMetric("HeapInuse", float64(c.memStats.HeapInuse)),
		gaugeMetric("HeapObjects", float64(c.memStats.HeapObjects)),
		gaugeMetric("HeapReleased", float64(c.memStats.HeapReleased)),
		gaugeMetric("HeapSys", float64(c.memStats.HeapSys)),
		gaugeMetric("LastGC", float64(c.memStats.LastGC)),
		gaugeMetric("Lookups", float64(c.memStats.Lookups)),
		gaugeMetric("MCacheInuse", float64(c.memStats.MCacheInuse)),
		gaugeMetric("MCacheSys", float64(c.memStats.MCacheSys)),
		gaugeMetric("MSpanInuse", float64(c.memStats.MSpanInuse)),
		gaugeMetric("MSpanSys", float64(c.memStats.MSpanSys)),
		gaugeMetric("Mallocs", float64(c.memStats.Mallocs)),
		gaugeMetric("NextGC", float64(c.memStats.NextGC)),
		gaugeMetric("NumForcedGC", float64(c.memStats.NumForcedGC)),
		gaugeMetric("NumGC", float64(c.memStats.NumGC)),
		gaugeMetric("OtherSys", float64(c.memStats.OtherSys)),
		gaugeMetric("PauseTotalNs", float64(c.memStats.PauseTotalNs)),
		gaugeMetric("StackInuse", float64(c.memStats.StackInuse)),
		gaugeMetric("StackSys", float64(c.memStats.StackSys)),
		gaugeMetric("Sys", float64(c.memStats.Sys)),
		gaugeMetric("TotalAlloc", float64(c.memStats.TotalAlloc)),

		// Custom metrics.
		gaugeMetric("RandomValue", c.randomValue),
		counterMetric("PollCount", c.pollCount),
	}

	return metrics
}

func (c *Collector) ResetPollCount() {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	c.pollCount = 0
}

func gaugeMetric(name string, v float64) domain.Metrics {
	return domain.Metrics{
		ID:    name,
		MType: domain.TypeGauge,
		Value: &v,
	}
}

func counterMetric(name string, d int64) domain.Metrics {
	return domain.Metrics{
		ID:    name,
		MType: domain.TypeCounter,
		Delta: &d,
	}
}

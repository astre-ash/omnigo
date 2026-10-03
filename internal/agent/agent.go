package agent

import (
	"context"
	"log"
	"time"
)

type Sender interface {
	SendAll(metrics []MetricData)
}

type AgentConfig struct {
	PollInterval   time.Duration
	ReportInterval time.Duration
}

type Agent struct {
	cfg       AgentConfig
	collector *Collector
	sender    Sender
}

func NewAgent(cfg AgentConfig, collector *Collector, sender Sender) *Agent {
	return &Agent{
		cfg:       cfg,
		collector: collector,
		sender:    sender,
	}
}

func (a *Agent) Run(ctx context.Context) {
	pollTicker := time.NewTicker(a.cfg.PollInterval)
	defer pollTicker.Stop()

	reportTicker := time.NewTicker(a.cfg.ReportInterval)
	defer reportTicker.Stop()

	log.Printf("agent started: poll interval %v, report interval %v", a.cfg.PollInterval, a.cfg.ReportInterval)

	for {
		select {
		case <-ctx.Done():
			log.Println("agent stopped gracefully")
			return

		case <-pollTicker.C:
			a.collector.Poll()

		case <-reportTicker.C:
			metrics := a.collector.GetMetrics()
			log.Printf("reporting %d metrics to server...", len(metrics))
			a.sender.SendAll(metrics)

		}
	}

}

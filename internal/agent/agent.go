package agent

import (
	"context"
	"log"
	"time"
)

type Sender interface {
	SendAll(metrics []MetricData) error
}

type Agent struct {
	pollInterval   time.Duration
	reportInterval time.Duration

	collector *Collector
	sender    Sender
}

func NewAgent(pollInterval time.Duration, reportInterval time.Duration, collector *Collector, sender Sender) *Agent {
	return &Agent{
		pollInterval:   pollInterval,
		reportInterval: reportInterval,
		collector:      collector,
		sender:         sender,
	}
}

func (a *Agent) Run(ctx context.Context) {
	pollTicker := time.NewTicker(a.pollInterval)
	defer pollTicker.Stop()

	reportTicker := time.NewTicker(a.reportInterval)
	defer reportTicker.Stop()

	log.Printf("agent started: poll interval %v, report interval %v", a.pollInterval, a.reportInterval)

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

			if err := a.sender.SendAll(metrics); err != nil {
				log.Printf("reporting failed: %v", err)
			} else {
				a.collector.ResetPollCount()
			}

		}
	}

}

package agent

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/astre-ash/omnigo/internal/domain"
)

type Sender interface {
	SendBatch(ctx context.Context, metrics []domain.Metrics) error
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
	log.Printf("agent started: poll interval %v, report interval %v", a.pollInterval, a.reportInterval)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		a.runPoller(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		a.runReporter(ctx)
	}()

	wg.Wait()
	log.Println("agent stopped gracefully")
}

func (a *Agent) runPoller(ctx context.Context) {
	ticker := time.NewTicker(a.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.collector.Poll()
		}
	}
}

func (a *Agent) runReporter(ctx context.Context) {
	ticker := time.NewTicker(a.reportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.report(ctx)
		}
	}
}

func (a *Agent) report(ctx context.Context) {
	metrics := a.collector.GetMetrics()
	if len(metrics) == 0 {
		return
	}

	var reportedPollCount int64
	for _, m := range metrics {
		if m.ID == "PollCount" && m.Delta != nil {
			reportedPollCount = *m.Delta
			break
		}
	}

	log.Printf("reporting %d metrics to server in batch...", len(metrics))

	if err := a.sender.SendBatch(ctx, metrics); err != nil {
		log.Printf("batch reporting failed: %v", err)
		return
	}

	if reportedPollCount > 0 {
		a.collector.DecreasePollCount(reportedPollCount)
	}
}

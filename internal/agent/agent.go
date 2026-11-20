package agent

import (
	"context"
	"sync"
	"time"

	"github.com/astre-ash/omnigo/internal/domain"
	"go.uber.org/zap"
)

type Sender interface {
	SendBatch(ctx context.Context, metrics []domain.Metrics) error
}

type Agent struct {
	pollInterval   time.Duration
	reportInterval time.Duration

	collector *Collector
	sender    Sender
	logger    *zap.Logger
}

func NewAgent(pollInterval time.Duration,
	reportInterval time.Duration,
	collector *Collector,
	sender Sender,
	logger *zap.Logger,
) *Agent {

	if logger == nil {
		logger = zap.NewNop()
	}

	return &Agent{
		pollInterval:   pollInterval,
		reportInterval: reportInterval,
		collector:      collector,
		sender:         sender,
		logger:         logger,
	}
}

func (a *Agent) Run(ctx context.Context) {
	a.logger.Info("agent started",
		zap.Duration("poll_interval", a.pollInterval),
		zap.Duration("report_interval", a.reportInterval),
	)

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
	a.logger.Info("agent stopped gracefully")
}

func (a *Agent) runPoller(ctx context.Context) {
	// Cold start.
	a.collector.Poll()

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

	a.logger.Info("reporting metrics to server in batch", zap.Int("count", len(metrics)))

	if err := a.sender.SendBatch(ctx, metrics); err != nil {
		a.logger.Error("batch reporting failed", zap.Error(err))
		return
	}

	if reportedPollCount > 0 {
		a.collector.DecreasePollCount(reportedPollCount)
	}
}

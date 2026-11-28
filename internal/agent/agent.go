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
	rateLimit      int

	sources []MetricSource
	sender  Sender
	logger  *zap.Logger
}

type reportJob struct {
	metrics []domain.Metrics
	ack     func(ok bool)
}

func NewAgent(
	pollInterval time.Duration,
	reportInterval time.Duration,
	rateLimit int,
	sender Sender,
	logger *zap.Logger,
	sources ...MetricSource,
) *Agent {

	if logger == nil {
		logger = zap.NewNop()
	}

	if rateLimit <= 0 {
		rateLimit = 1
	}

	return &Agent{
		pollInterval:   pollInterval,
		reportInterval: reportInterval,
		rateLimit:      rateLimit,
		sources:        sources,
		sender:         sender,
		logger:         logger,
	}
}

func (a *Agent) Run(ctx context.Context) {
	a.logger.Info("agent started",
		zap.Duration("poll_interval", a.pollInterval),
		zap.Duration("report_interval", a.reportInterval),
		zap.Int("rate_limit", a.rateLimit),
		zap.Int("sources_count", len(a.sources)),
	)

	jobs := make(chan reportJob, a.rateLimit)

	var workersWG sync.WaitGroup
	for i := range a.rateLimit {
		workersWG.Go(func() {
			a.runWorker(ctx, i+1, jobs)
		})
	}

	var producersWG sync.WaitGroup

	for _, src := range a.sources {
		producersWG.Go(func() {
			a.runPoller(ctx, src)
		})
	}

	producersWG.Go(func() {
		defer close(jobs)
		a.runReporter(ctx, jobs)
	})

	producersWG.Wait()
	workersWG.Wait()
	a.logger.Info("agent stopped gracefully")
}

func (a *Agent) runWorker(ctx context.Context, workerID int, jobs <-chan reportJob) {
	for job := range jobs {
		a.logger.Debug("worker sending batch",
			zap.Int("worker_id", workerID),
			zap.Int("count", len(job.metrics)),
		)

		if err := a.sender.SendBatch(ctx, job.metrics); err != nil {
			job.ack(false)
			if ctx.Err() == nil {
				a.logger.Error("batch reporting failed",
					zap.Int("worker_id", workerID),
					zap.Error(err),
				)
			}
			continue
		}

		job.ack(true)
	}
}

func (a *Agent) buildJob() reportJob {
	var allMetrics []domain.Metrics
	var sourceAcks []func(bool)

	for _, src := range a.sources {
		m, srcAck := src.Collect()
		if len(m) == 0 {
			continue
		}
		allMetrics = append(allMetrics, m...)
		if srcAck != nil {
			sourceAcks = append(sourceAcks, srcAck)
		}
	}

	if len(allMetrics) == 0 {
		return reportJob{}
	}

	return reportJob{
		metrics: allMetrics,
		ack: func(ok bool) {
			for _, fn := range sourceAcks {
				fn(ok)
			}
		},
	}
}

func (a *Agent) runReporter(ctx context.Context, jobs chan<- reportJob) {
	ticker := time.NewTicker(a.reportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			job := a.buildJob()
			if len(job.metrics) == 0 {
				continue
			}
			select {
			case jobs <- job:
			case <-ctx.Done():
				job.ack(false)
				return
			default:
				a.logger.Warn("worker pool is saturated, skipping report batch")
				job.ack(false)
			}

		}
	}
}

func (a *Agent) runPoller(ctx context.Context, src MetricSource) {
	poll := func() {
		if err := src.Poll(ctx); err != nil && ctx.Err() == nil {
			a.logger.Warn("metric poll failed",
				zap.String("source", src.Name()),
				zap.Error(err),
			)
		}
	}

	// Cold start.
	poll()

	ticker := time.NewTicker(a.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			poll()
		}
	}
}

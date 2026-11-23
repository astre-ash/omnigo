package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/astre-ash/omnigo/internal/agent"
	"github.com/astre-ash/omnigo/internal/config"
	"github.com/astre-ash/omnigo/pkg/compress"
	"github.com/astre-ash/omnigo/pkg/retry"
	"github.com/astre-ash/omnigo/pkg/signature"
	"go.uber.org/zap"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "agent terminated with error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger, err := zap.NewProduction()
	if err != nil {
		return fmt.Errorf("failed to initialize zap logger: %w", err)
	}

	defer func() { _ = logger.Sync() }()

	cfg, err := config.LoadAgentConfig(os.Args[1:])
	if err != nil {
		logger.Error("failed to load agent configuration", zap.Error(err))
		return fmt.Errorf("failed to load agent configuration: %w", err)
	}

	var transport http.RoundTripper = http.DefaultTransport
	if cfg.Sender.Key != "" {
		signer := signature.NewHMACSHA256(cfg.Sender.Key)
		transport = signature.NewSigningRoundTripper(transport, signer)
	}

	logger.Info("request signing configured", zap.Bool("enabled", cfg.Sender.Key != ""))

	baseClient := &http.Client{
		Timeout:   cfg.Sender.ClientTimeout,
		Transport: transport,
	}

	retryingClient := retry.NewHTTPClient(baseClient, retry.DefaultDelays(), logger)

	collector := agent.NewCollector()
	gz := compress.NewGzipCompressor()
	sender := agent.NewMetricSender(cfg.Sender.Address, retryingClient, gz)
	app := agent.NewAgent(
		cfg.Loop.PollInterval,
		cfg.Loop.ReportInterval,
		collector,
		sender,
		logger,
	)

	app.Run(ctx)
	return nil
}

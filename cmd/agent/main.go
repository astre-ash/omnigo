package main

import (
	"context"
	"os/signal"
	"syscall"
	"time"

	"github.com/astre-ash/omnigo/internal/agent"
)

const (
	serverAddr     = "http://127.0.0.1:8080"
	pollInterval   = 2 * time.Second
	reportInterval = 10 * time.Second
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT,
		syscall.SIGTERM)
	defer stop()

	cfg := agent.AgentConfig{
		PollInterval:   pollInterval,
		ReportInterval: reportInterval,
	}

	collertor := agent.NewCollector()
	sender := agent.NewMetricSender(serverAddr)

	app := agent.NewAgent(cfg, collertor, sender)
	app.Run(ctx)
}

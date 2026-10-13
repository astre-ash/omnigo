package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/astre-ash/omnigo/internal/agent"
	"github.com/astre-ash/omnigo/internal/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.LoadAgentConfig(os.Args[1:])
	if err != nil {
		log.Fatalf("failed to load agent configuration: %v", err)
	}

	collector := agent.NewCollector()
	sender := agent.NewMetricSender(cfg.Sender.Address)
	app := agent.NewAgent(cfg.Loop.PollInterval, cfg.Loop.ReportInterval, collector, sender)
	app.Run(ctx)
}

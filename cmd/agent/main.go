package main

import (
	"context"
	"os/signal"
	"syscall"
	"time"

	"github.com/astre-ash/omnigo/internal/agent"
	"github.com/astre-ash/omnigo/internal/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT,
		syscall.SIGTERM)
	defer stop()

	rawCfg := config.LoadAgentConfig()

	cfg := agent.AgentConfig{
		PollInterval:   time.Duration(rawCfg.PollInterval) * time.Second,
		ReportInterval: time.Duration(rawCfg.ReportInterval) * time.Second,
	}

	collertor := agent.NewCollector()
	sender := agent.NewMetricSender(rawCfg.Address)

	app := agent.NewAgent(cfg, collertor, sender)
	app.Run(ctx)
}

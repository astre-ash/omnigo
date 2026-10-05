package config

import (
	"flag"
	"strings"
)

type AgentConfig struct {
	Address        string
	ReportInterval int
	PollInterval   int
}

func LoadAgentConfig() *AgentConfig {
	cfg := &AgentConfig{}

	flag.StringVar(&cfg.Address, "a", "127.0.0.1:8080", "endpoint HTTP server address (host:port)")
	flag.IntVar(&cfg.ReportInterval, "r", 10, "report interval to server (in seconds)")
	flag.IntVar(&cfg.PollInterval, "p", 2, "poll runtime metrics interval (in seconds)")

	flag.Parse()

	if !strings.HasPrefix(cfg.Address, "http://") && !strings.HasPrefix(cfg.Address, "https://") {
		cfg.Address = "http://" + cfg.Address
	}

	return cfg
}

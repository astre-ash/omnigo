package config

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
)

const (
	defaultAddress        = "127.0.0.1:8080"
	defaultReportInterval = 10
	defaultPollInterval   = 2
)

type SenderConfig struct {
	Address string
}

type LoopConfig struct {
	PollInterval   time.Duration
	ReportInterval time.Duration
}

type AgentConfig struct {
	Sender SenderConfig
	Loop   LoopConfig
}

type agentRawConfig struct {
	Address        string `env:"ADDRESS"`
	ReportInterval int    `env:"REPORT_INTERVAL"`
	PollInterval   int    `env:"POLL_INTERVAL"`
}

func LoadAgentConfig(args []string) (*AgentConfig, error) {
	var rawCfg agentRawConfig

	fs := flag.NewFlagSet("agent", flag.ContinueOnError)

	fs.StringVar(&rawCfg.Address, "a", defaultAddress, "endpoint HTTP server address (host:port)")
	fs.IntVar(&rawCfg.ReportInterval, "r", defaultReportInterval, "report interval to server (in seconds)")
	fs.IntVar(&rawCfg.PollInterval, "p", defaultPollInterval, "poll runtime metrics interval (in seconds)")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("failed to parse command-line flags: %w", err)
	}

	if err := env.Parse(&rawCfg); err != nil {
		return nil, fmt.Errorf("failed to parse environment variables: %w", err)
	}

	rawCfg.Address = strings.TrimRight(rawCfg.Address, "/")
	if !strings.HasPrefix(rawCfg.Address, "http://") && !strings.HasPrefix(rawCfg.Address, "https://") {
		rawCfg.Address = "http://" + rawCfg.Address
	}

	return &AgentConfig{
		Sender: SenderConfig{Address: rawCfg.Address},
		Loop: LoopConfig{
			ReportInterval: time.Duration(rawCfg.ReportInterval) * time.Second,
			PollInterval:   time.Duration(rawCfg.PollInterval) * time.Second,
		},
	}, nil

}

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
	defaultRateLimit      = 1
	defaultClientTimeout  = 5 * time.Second
)

type SenderConfig struct {
	Address       string
	ClientTimeout time.Duration
	Key           string
	RateLimit     int
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
	Key            string `env:"KEY"`
	RateLimit      int    `env:"RATE_LIMIT"`
}

func LoadAgentConfig(args []string) (*AgentConfig, error) {
	var rawCfg agentRawConfig

	fs := flag.NewFlagSet("agent", flag.ContinueOnError)

	fs.StringVar(&rawCfg.Address, "a", defaultAddress, "endpoint HTTP server address (host:port)")
	fs.IntVar(&rawCfg.ReportInterval, "r", defaultReportInterval, "report interval to server (in seconds)")
	fs.IntVar(&rawCfg.PollInterval, "p", defaultPollInterval, "poll runtime metrics interval (in seconds)")
	fs.StringVar(&rawCfg.Key, "k", "", "secret key for HMAC-SHA256 request signing")
	fs.IntVar(&rawCfg.RateLimit, "l", defaultRateLimit, "max number of concurrent outgoing requests")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("failed to parse command-line flags: %w", err)
	}

	if err := env.Parse(&rawCfg); err != nil {
		return nil, fmt.Errorf("failed to parse environment variables: %w", err)
	}

	if rawCfg.PollInterval <= 0 {
		return nil, fmt.Errorf("poll interval must be positive, got %d", rawCfg.PollInterval)
	}

	if rawCfg.ReportInterval <= 0 {
		return nil, fmt.Errorf("report interval must be positive, got %d", rawCfg.ReportInterval)
	}

	if rawCfg.RateLimit <= 0 {
		return nil, fmt.Errorf("rate limit must be positive, got %d", rawCfg.RateLimit)
	}

	rawCfg.Address = strings.TrimRight(rawCfg.Address, "/")
	if !strings.HasPrefix(rawCfg.Address, "http://") && !strings.HasPrefix(rawCfg.Address, "https://") {
		rawCfg.Address = "http://" + rawCfg.Address
	}

	return &AgentConfig{
		Sender: SenderConfig{
			Address:       rawCfg.Address,
			ClientTimeout: defaultClientTimeout,
			Key:           rawCfg.Key,
			RateLimit:     rawCfg.RateLimit,
		},
		Loop: LoopConfig{
			ReportInterval: time.Duration(rawCfg.ReportInterval) * time.Second,
			PollInterval:   time.Duration(rawCfg.PollInterval) * time.Second,
		},
	}, nil
}

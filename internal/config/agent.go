package config

import (
	"flag"
	"strings"
	"time"
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

type Config struct {
	Sender SenderConfig
	Loop   LoopConfig
}

func LoadAgentConfig() *Config {
	var (
		address        string
		reportInterval int
		pollInterval   int
	)

	flag.StringVar(&address, "a", defaultAddress, "endpoint HTTP server address (host:port)")
	flag.IntVar(&reportInterval, "r", defaultReportInterval, "report interval to server (in seconds)")
	flag.IntVar(&pollInterval, "p", defaultPollInterval, "poll runtime metrics interval (in seconds)")

	flag.Parse()

	address = strings.TrimRight(address, "/")
	if !strings.HasPrefix(address, "http://") && !strings.HasPrefix(address, "https://") {
		address = "http://" + address
	}

	return &Config{
		Sender: SenderConfig{
			Address: address,
		},
		Loop: LoopConfig{
			ReportInterval: time.Duration(reportInterval) * time.Second,
			PollInterval:   time.Duration(pollInterval) * time.Second,
		},
	}
}

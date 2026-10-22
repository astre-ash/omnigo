package config

import (
	"flag"
	"fmt"

	"github.com/caarlos0/env/v11"
)

const defaultServerAddress = "localhost:8080"

type ServerConfig struct {
	Address string `env:"ADDRESS"`
}

func LoadServerConfig(args []string) (*ServerConfig, error) {
	var cfg ServerConfig

	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	fs.StringVar(&cfg.Address, "a", defaultServerAddress, "endpoint HTTP server address (host:port)")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("failed to parse server command-line flags: %w", err)
	}

	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse server environment variables: %w", err)
	}

	return &cfg, nil
}

package config

import (
	"flag"
	"fmt"

	"github.com/caarlos0/env/v11"
)

const (
	defaultServerAddress   = "localhost:8080"
	defaultStoreInterval   = 300
	defaultFileStoragePath = "/tmp/metrics-db.json"
	defaultRestore         = true
)

type ServerConfig struct {
	Address         string `env:"ADDRESS"`
	StoreInterval   int    `env:"STORE_INTERVAL"`
	FileStoragePath string `env:"FILE_STORAGE_PATH"`
	Restore         bool   `env:"RESTORE"`
}

func LoadServerConfig(args []string) (*ServerConfig, error) {
	var cfg ServerConfig

	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	fs.StringVar(&cfg.Address, "a", defaultServerAddress, "endpoint HTTP server address (host:port)")
	fs.IntVar(&cfg.StoreInterval, "i", defaultStoreInterval, "interval in seconds to dump metrics to disk (0 for synchronous)")
	fs.StringVar(&cfg.FileStoragePath, "f", defaultFileStoragePath, "path to file for metrics storage")
	fs.BoolVar(&cfg.Restore, "r", defaultRestore, "restore previously saved values on startup")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("failed to parse server command-line flags: %w", err)
	}

	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse server environment variables: %w", err)
	}

	return &cfg, nil
}

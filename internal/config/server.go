package config

import (
	"flag"
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

const (
	defaultServerAddress   = "localhost:8080"
	defaultStoreInterval   = 300
	defaultFileStoragePath = "/tmp/metrics-db.json"
	defaultRestore         = true
)

type ServerConfig struct {
	Address         string        `env:"ADDRESS"`
	StoreInterval   time.Duration `env:"STORE_INTERVAL"`
	FileStoragePath string        `env:"FILE_STORAGE_PATH"`
	Restore         bool          `env:"RESTORE"`
	DatabaseDSN     string        `env:"DATABASE_DSN"`
}

func LoadServerConfig(args []string) (*ServerConfig, error) {
	var raw struct {
		Address         string `env:"ADDRESS"`
		StoreInterval   int    `env:"STORE_INTERVAL"`
		FileStoragePath string `env:"FILE_STORAGE_PATH"`
		Restore         bool   `env:"RESTORE"`
		DatabaseDSN     string `env:"DATABASE_DSN"`
	}

	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	fs.StringVar(&raw.Address, "a", defaultServerAddress, "endpoint HTTP server address (host:port)")
	fs.IntVar(&raw.StoreInterval, "i", defaultStoreInterval, "interval in seconds to dump metrics to disk (0 for synchronous)")
	fs.StringVar(&raw.FileStoragePath, "f", defaultFileStoragePath, "path to file for metrics storage")
	fs.BoolVar(&raw.Restore, "r", defaultRestore, "restore previously saved values on startup")
	fs.StringVar(&raw.DatabaseDSN, "d", "", "database DSN (e.g. postgres://user:password@localhost:5432/db)")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("failed to parse server command-line flags: %w", err)
	}

	if err := env.Parse(&raw); err != nil {
		return nil, fmt.Errorf("failed to parse server environment variables: %w", err)
	}

	if raw.StoreInterval < 0 {
		return nil, fmt.Errorf("store interval must be non-negative, got %d", raw.StoreInterval)
	}

	cfg := &ServerConfig{
		Address:         raw.Address,
		StoreInterval:   time.Duration(raw.StoreInterval) * time.Second,
		FileStoragePath: raw.FileStoragePath,
		Restore:         raw.Restore,
		DatabaseDSN:     raw.DatabaseDSN,
	}

	return cfg, nil
}

package config

import (
	"flag"
)

type ServerConfig struct {
	Address string
}

func LoadServerConfig() *ServerConfig {
	cfg := &ServerConfig{}

	flag.StringVar(&cfg.Address, "a", ":8080", "endpoint HTTP server address (short)")

	flag.Parse()

	return cfg
}

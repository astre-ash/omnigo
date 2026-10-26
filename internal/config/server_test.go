package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadServerConfig(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		envs        map[string]string
		wantAddress string
		wantErr     bool
	}{
		{
			name:        "default configuration",
			args:        []string{},
			envs:        map[string]string{},
			wantAddress: "localhost:8080",
			wantErr:     false,
		},
		{
			name: "command line flag overrides default",
			args: []string{
				"-a", ":9090",
			},
			envs:        map[string]string{},
			wantAddress: ":9090",
			wantErr:     false,
		},
		{
			name: "environment variable overrides flag",
			args: []string{
				"-a", ":9090",
			},
			envs: map[string]string{
				"ADDRESS": "127.0.0.1:7070",
			},
			wantAddress: "127.0.0.1:7070",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envs {
				t.Setenv(k, v)
			}

			cfg, err := LoadServerConfig(tt.args)

			if tt.wantErr {
				require.Error(t, err, "expected error when loading config")
				assert.Nil(t, cfg, "config must be nil on error")
				return
			}

			require.NoError(t, err, "unexpected error during config load")
			require.NotNil(t, cfg, "config must not be nil")
			assert.Equal(t, tt.wantAddress, cfg.Address, "unexpected server address")
		})
	}
}

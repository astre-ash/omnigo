package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadAgentConfig(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		envs        map[string]string
		wantAddress string
		wantReport  time.Duration
		wantPoll    time.Duration
		wantErr     bool
	}{
		{
			name:        "default configuration",
			args:        []string{},
			envs:        map[string]string{},
			wantAddress: "http://127.0.0.1:8080",
			wantReport:  10 * time.Second,
			wantPoll:    2 * time.Second,
			wantErr:     false,
		},
		{
			name: "command line flags override defaults",
			args: []string{
				"-a", "127.0.0.1:9090",
				"-r", "20",
				"-p", "4",
			},
			envs:        map[string]string{},
			wantAddress: "http://127.0.0.1:9090",
			wantReport:  20 * time.Second,
			wantPoll:    4 * time.Second,
			wantErr:     false,
		},
		{
			name: "environment variables override flags",
			args: []string{
				"-a", "127.0.0.1:9090",
				"-r", "20",
				"-p", "4",
			},
			envs: map[string]string{
				"ADDRESS":         "example.com:8080",
				"REPORT_INTERVAL": "30",
				"POLL_INTERVAL":   "5",
			},
			wantAddress: "http://example.com:8080",
			wantReport:  30 * time.Second,
			wantPoll:    5 * time.Second,
			wantErr:     false,
		},
		{
			name: "address normalization preserves existing schema",
			args: []string{},
			envs: map[string]string{
				"ADDRESS": "secure.server.local:8443/",
			},
			wantAddress: "http://secure.server.local:8443",
			wantReport:  10 * time.Second,
			wantPoll:    2 * time.Second,
			wantErr:     false,
		},
		{
			name: "invalid environment variable format",
			args: []string{},
			envs: map[string]string{
				"REPORT_INTERVAL": "invalid_number",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envs {
				t.Setenv(k, v)
			}

			cfg, err := LoadAgentConfig(tt.args)

			if tt.wantErr {
				require.Error(t, err, "expected error when parsing invalid configuration")
				assert.Nil(t, cfg, "config must be nil on error")
				return
			}

			require.NoError(t, err, "unexpected error during configuration load")
			require.NotNil(t, cfg, "config must not be nil")

			assert.Equal(t, tt.wantAddress, cfg.Sender.Address, "unexpected sender address")
			assert.Equal(t, tt.wantReport, cfg.Loop.ReportInterval, "unexpected report interval")
			assert.Equal(t, tt.wantPoll, cfg.Loop.PollInterval, "unexpected poll interval")
		})
	}
}

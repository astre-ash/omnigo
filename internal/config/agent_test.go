package config

import (
	"os"
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
		wantTimeout time.Duration
		wantKey     string
		wantReport  time.Duration
		wantPoll    time.Duration
		wantErr     bool
	}{
		{
			name:        "default configuration",
			args:        []string{},
			envs:        map[string]string{},
			wantAddress: "http://127.0.0.1:8080",
			wantTimeout: defaultClientTimeout,
			wantKey:     "",
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
				"-k", "flag-secret-key",
			},
			envs:        map[string]string{},
			wantAddress: "http://127.0.0.1:9090",
			wantTimeout: defaultClientTimeout,
			wantKey:     "flag-secret-key",
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
				"-k", "flag-secret-key",
			},
			envs: map[string]string{
				"ADDRESS":         "example.com:8080",
				"REPORT_INTERVAL": "30",
				"POLL_INTERVAL":   "5",
				"KEY":             "env-secret-key",
			},
			wantAddress: "http://example.com:8080",
			wantTimeout: defaultClientTimeout,
			wantKey:     "env-secret-key",
			wantReport:  30 * time.Second,
			wantPoll:    5 * time.Second,
			wantErr:     false,
		},
		{
			name: "environment variable sets Key without flag",
			args: []string{},
			envs: map[string]string{
				"KEY": "hmac-secret",
			},
			wantAddress: "http://127.0.0.1:8080",
			wantTimeout: defaultClientTimeout,
			wantKey:     "hmac-secret",
			wantReport:  10 * time.Second,
			wantPoll:    2 * time.Second,
			wantErr:     false,
		},
		{
			name: "address normalization trims trailing slashes and adds http schema",
			args: []string{},
			envs: map[string]string{
				"ADDRESS": "secure.server.local:8443///",
			},
			wantAddress: "http://secure.server.local:8443",
			wantTimeout: defaultClientTimeout,
			wantKey:     "",
			wantReport:  10 * time.Second,
			wantPoll:    2 * time.Second,
			wantErr:     false,
		},
		{
			name: "address normalization preserves existing https schema",
			args: []string{},
			envs: map[string]string{
				"ADDRESS": "https://secure.server.local:8443/",
			},
			wantAddress: "https://secure.server.local:8443",
			wantTimeout: defaultClientTimeout,
			wantKey:     "",
			wantReport:  10 * time.Second,
			wantPoll:    2 * time.Second,
			wantErr:     false,
		},
		{
			name:    "invalid command line flag returns error",
			args:    []string{"-unknown-flag"},
			envs:    map[string]string{},
			wantErr: true,
		},
		{
			name:    "non-positive poll interval in flag returns error",
			args:    []string{"-p", "0"},
			envs:    map[string]string{},
			wantErr: true,
		},
		{
			name:    "negative poll interval in flag returns error",
			args:    []string{"-p", "-2"},
			envs:    map[string]string{},
			wantErr: true,
		},
		{
			name:    "non-positive report interval in flag returns error",
			args:    []string{"-r", "0"},
			envs:    map[string]string{},
			wantErr: true,
		},
		{
			name: "negative report interval in environment variable returns error",
			args: []string{},
			envs: map[string]string{
				"REPORT_INTERVAL": "-10",
			},
			wantErr: true,
		},
		{
			name: "non-positive poll interval in environment variable returns error",
			args: []string{},
			envs: map[string]string{
				"POLL_INTERVAL": "0",
			},
			wantErr: true,
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
			knownEnvs := []string{"ADDRESS", "REPORT_INTERVAL", "POLL_INTERVAL", "KEY"}
			for _, envKey := range knownEnvs {
				if _, ok := tt.envs[envKey]; !ok {
					if origVal, exists := os.LookupEnv(envKey); exists {
						os.Unsetenv(envKey)
						t.Cleanup(func() { os.Setenv(envKey, origVal) })
					}
				}
			}

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
			assert.Equal(t, tt.wantTimeout, cfg.Sender.ClientTimeout, "unexpected client timeout")
			assert.Equal(t, tt.wantKey, cfg.Sender.Key, "unexpected sender key")
			assert.Equal(t, tt.wantReport, cfg.Loop.ReportInterval, "unexpected report interval")
			assert.Equal(t, tt.wantPoll, cfg.Loop.PollInterval, "unexpected poll interval")
		})
	}
}

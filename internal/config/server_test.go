package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadServerConfig(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		envs       map[string]string
		wantConfig *ServerConfig
		wantErr    bool
	}{
		{
			name: "default configuration for all fields",
			args: []string{},
			envs: map[string]string{},
			wantConfig: &ServerConfig{
				Address:         defaultServerAddress,
				StoreInterval:   defaultStoreInterval * time.Second,
				FileStoragePath: defaultFileStoragePath,
				Restore:         defaultRestore,
				DatabaseDSN:     "",
				Key:             "",
			},
			wantErr: false,
		},
		{
			name: "command line flags override defaults",
			args: []string{
				"-a", ":9090",
				"-i", "10",
				"-f", "/custom/path/storage.json",
				"-r=false",
				"-d", "postgres://user:pass@localhost:5432/metrics",
				"-k", "flag-secret-key",
			},
			envs: map[string]string{},
			wantConfig: &ServerConfig{
				Address:         ":9090",
				StoreInterval:   10 * time.Second,
				FileStoragePath: "/custom/path/storage.json",
				Restore:         false,
				DatabaseDSN:     "postgres://user:pass@localhost:5432/metrics",
				Key:             "flag-secret-key",
			},
			wantErr: false,
		},
		{
			name: "environment variables override command line flags",
			args: []string{
				"-a", ":9090",
				"-i", "10",
				"-f", "/flag/path.json",
				"-r=true",
				"-d", "postgres://flag:pass@localhost:5432/db",
				"-k", "flag-secret",
			},
			envs: map[string]string{
				"ADDRESS":           "127.0.0.1:7070",
				"STORE_INTERVAL":    "60",
				"FILE_STORAGE_PATH": "/env/path.json",
				"RESTORE":           "false",
				"DATABASE_DSN":      "postgres://env:pass@localhost:5432/db",
				"KEY":               "env-secret",
			},
			wantConfig: &ServerConfig{
				Address:         "127.0.0.1:7070",
				StoreInterval:   60 * time.Second,
				FileStoragePath: "/env/path.json",
				Restore:         false,
				DatabaseDSN:     "postgres://env:pass@localhost:5432/db",
				Key:             "env-secret",
			},
			wantErr: false,
		},
		{
			name: "environment variables set DatabaseDSN and Key without flags",
			args: []string{},
			envs: map[string]string{
				"DATABASE_DSN": "postgres://user:pass@db:5432/app",
				"KEY":          "hmac-key",
			},
			wantConfig: &ServerConfig{
				Address:         defaultServerAddress,
				StoreInterval:   defaultStoreInterval * time.Second,
				FileStoragePath: defaultFileStoragePath,
				Restore:         defaultRestore,
				DatabaseDSN:     "postgres://user:pass@db:5432/app",
				Key:             "hmac-key",
			},
			wantErr: false,
		},
		{
			name: "synchronous store interval (0) and empty storage path",
			args: []string{
				"-i", "0",
				"-f", "",
			},
			envs: map[string]string{},
			wantConfig: &ServerConfig{
				Address:         defaultServerAddress,
				StoreInterval:   0 * time.Second,
				FileStoragePath: "",
				Restore:         defaultRestore,
				DatabaseDSN:     "",
				Key:             "",
			},
			wantErr: false,
		},
		{
			name: "failure: unknown command line flag",
			args: []string{
				"-unknown", "value",
			},
			envs:    map[string]string{},
			wantErr: true,
		},
		{
			name: "failure: invalid integer flag value",
			args: []string{
				"-i", "not_a_number",
			},
			envs:    map[string]string{},
			wantErr: true,
		},
		{
			name: "failure: negative store interval in flag",
			args: []string{
				"-i", "-5",
			},
			envs:    map[string]string{},
			wantErr: true,
		},
		{
			name: "failure: negative store interval in environment variable",
			args: []string{},
			envs: map[string]string{
				"STORE_INTERVAL": "-1",
			},
			wantErr: true,
		},
		{
			name: "failure: invalid integer in environment variable",
			args: []string{},
			envs: map[string]string{
				"STORE_INTERVAL": "invalid_int",
			},
			wantErr: true,
		},
		{
			name: "failure: invalid boolean in environment variable",
			args: []string{},
			envs: map[string]string{
				"RESTORE": "not_a_bool",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			knownEnvs := []string{
				"ADDRESS",
				"STORE_INTERVAL",
				"FILE_STORAGE_PATH",
				"RESTORE",
				"DATABASE_DSN",
				"KEY",
			}
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

			cfg, err := LoadServerConfig(tt.args)

			if tt.wantErr {
				require.Error(t, err, "expected error when loading invalid config")
				assert.Nil(t, cfg, "config must be nil on error")
				return
			}

			require.NoError(t, err, "unexpected error during config load")
			require.NotNil(t, cfg, "config must not be nil")
			assert.Equal(t, tt.wantConfig, cfg, "config fields do not match expected")
		})
	}
}

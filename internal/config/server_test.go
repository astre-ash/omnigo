package config

import (
	"testing"

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
				StoreInterval:   defaultStoreInterval,
				FileStoragePath: defaultFileStoragePath,
				Restore:         defaultRestore,
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
			},
			envs: map[string]string{},
			wantConfig: &ServerConfig{
				Address:         ":9090",
				StoreInterval:   10,
				FileStoragePath: "/custom/path/storage.json",
				Restore:         false,
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
			},
			envs: map[string]string{
				"ADDRESS":           "127.0.0.1:7070",
				"STORE_INTERVAL":    "60",
				"FILE_STORAGE_PATH": "/env/path.json",
				"RESTORE":           "false",
			},
			wantConfig: &ServerConfig{
				Address:         "127.0.0.1:7070",
				StoreInterval:   60,
				FileStoragePath: "/env/path.json",
				Restore:         false,
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
				StoreInterval:   0,
				FileStoragePath: "",
				Restore:         defaultRestore,
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

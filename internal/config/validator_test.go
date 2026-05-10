package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		wantErr     bool
		errContains string
	}{
		{
			name: "debug mode passes without password",
			config: &Config{
				Server: ServerConfig{
					Mode: "debug",
				},
			},
			wantErr: false,
		},
		{
			name: "production mode with empty redis password fails",
			config: &Config{
				Server: ServerConfig{
					Mode: "release",
				},
				Redis: RedisConfig{
					Password: "",
				},
			},
			wantErr:     true,
			errContains: "redis password is required in production",
		},
		{
			name: "production mode with all credentials passes",
			config: &Config{
				Server: ServerConfig{
					Mode: "release",
				},
				Redis: RedisConfig{
					Password: "secure-redis-password",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

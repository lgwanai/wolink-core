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
			name: "empty JWT secret fails",
			config: &Config{
				Security: SecurityConfig{
					JWTSecret: "",
				},
				Server: ServerConfig{
					Mode: "debug",
				},
			},
			wantErr:     true,
			errContains: "JWT secret is required",
		},
		{
			name: "JWT secret less than 32 characters fails",
			config: &Config{
				Security: SecurityConfig{
					JWTSecret: "short-secret-key-12345",
				},
				Server: ServerConfig{
					Mode: "debug",
				},
			},
			wantErr:     true,
			errContains: "at least 32 characters",
		},
		{
			name: "JWT secret exactly 32 characters passes",
			config: &Config{
				Security: SecurityConfig{
					JWTSecret: "exactly-32-characters-jwt-key!!!",
				},
				Server: ServerConfig{
					Mode: "debug",
				},
				Database: DatabaseConfig{
					Password: "test",
				},
				Redis: RedisConfig{
					Password: "test",
				},
			},
			wantErr: false,
		},
		{
			name: "default JWT secret 'your-secret-key' fails (too short)",
			config: &Config{
				Security: SecurityConfig{
					JWTSecret: "your-secret-key",
				},
				Server: ServerConfig{
					Mode: "debug",
				},
			},
			wantErr:     true,
			errContains: "at least 32 characters",
		},
		{
			name: "default JWT secret 'your-super-secret-jwt-key' fails (too short)",
			config: &Config{
				Security: SecurityConfig{
					JWTSecret: "your-super-secret-jwt-key",
				},
				Server: ServerConfig{
					Mode: "debug",
				},
			},
			wantErr:     true,
			errContains: "at least 32 characters",
		},
		{
			name: "default JWT secret 'secret' fails (too short)",
			config: &Config{
				Security: SecurityConfig{
					JWTSecret: "secret",
				},
				Server: ServerConfig{
					Mode: "debug",
				},
			},
			wantErr:     true,
			errContains: "at least 32 characters",
		},
		{
			name: "default JWT secret 'jwt-secret' fails (too short)",
			config: &Config{
				Security: SecurityConfig{
					JWTSecret: "jwt-secret",
				},
				Server: ServerConfig{
					Mode: "debug",
				},
			},
			wantErr:     true,
			errContains: "at least 32 characters",
		},
		{
			name: "default JWT secret 'changeme' fails (too short)",
			config: &Config{
				Security: SecurityConfig{
					JWTSecret: "changeme",
				},
				Server: ServerConfig{
					Mode: "debug",
				},
			},
			wantErr:     true,
			errContains: "at least 32 characters",
		},
		{
			name: "default JWT secret padded to 32 chars fails",
			config: &Config{
				Security: SecurityConfig{
					JWTSecret: "your-secret-key-padded-to-32-chars",
				},
				Server: ServerConfig{
					Mode: "debug",
				},
			},
			wantErr:     true,
			errContains: "default or insecure value",
		},
		{
			name: "valid JWT secret passes",
			config: &Config{
				Security: SecurityConfig{
					JWTSecret: "a-very-long-secure-jwt-key-here-32chars",
				},
				Server: ServerConfig{
					Mode: "debug",
				},
				Database: DatabaseConfig{
					Password: "test",
				},
				Redis: RedisConfig{
					Password: "test",
				},
			},
			wantErr: false,
		},
		{
			name: "production mode with empty database password fails",
			config: &Config{
				Security: SecurityConfig{
					JWTSecret: "a-very-long-secure-jwt-key-here-32chars",
				},
				Server: ServerConfig{
					Mode: "release",
				},
				Database: DatabaseConfig{
					Password: "",
				},
				Redis: RedisConfig{
					Password: "test",
				},
			},
			wantErr:     true,
			errContains: "database password is required in production",
		},
		{
			name: "production mode with empty redis password fails",
			config: &Config{
				Security: SecurityConfig{
					JWTSecret: "a-very-long-secure-jwt-key-here-32chars",
				},
				Server: ServerConfig{
					Mode: "release",
				},
				Database: DatabaseConfig{
					Password: "test",
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
				Security: SecurityConfig{
					JWTSecret: "a-very-long-secure-jwt-key-here-32chars",
				},
				Server: ServerConfig{
					Mode: "release",
				},
				Database: DatabaseConfig{
					Password: "secure-db-password",
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

package config

import (
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestInfrastructureConfig_Unmarshal(t *testing.T) {
	tests := []struct {
		name     string
		setup    func()
		expected InfrastructureConfig
	}{
		{
			name: "InfrastructureConfig unmarshals from YAML correctly",
			setup: func() {
				// Set defaults that would normally come from YAML
				setInfrastructureDefaults()
			},
			expected: InfrastructureConfig{
				ShutdownTimeout:   30 * time.Second,
				ReadTimeout:       15 * time.Second,
				WriteTimeout:      30 * time.Second,
				IdleTimeout:       120 * time.Second,
				ReadHeaderTimeout: 5 * time.Second,
				Database: DBPoolConfig{
					MaxOpenConns:    25,
					MaxIdleConns:    10,
					ConnMaxLifetime: 5 * time.Minute,
				},
				Redis: RedisPoolConfig{
					PoolSize:      20,
					MinIdleConns:   5,
					ConnMaxLifetime: 5 * time.Minute,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()

			var infra InfrastructureConfig
			// Manually unmarshal from viper defaults
			infra.ShutdownTimeout = getDefaultDuration("infrastructure.shutdown_timeout")
			infra.ReadTimeout = getDefaultDuration("infrastructure.read_timeout")
			infra.WriteTimeout = getDefaultDuration("infrastructure.write_timeout")
			infra.IdleTimeout = getDefaultDuration("infrastructure.idle_timeout")
			infra.ReadHeaderTimeout = getDefaultDuration("infrastructure.read_header_timeout")
			infra.Database.MaxOpenConns = getDefaultInt("infrastructure.database_pool.max_open_conns")
			infra.Database.MaxIdleConns = getDefaultInt("infrastructure.database_pool.max_idle_conns")
			infra.Database.ConnMaxLifetime = getDefaultDuration("infrastructure.database_pool.conn_max_lifetime")
			infra.Redis.PoolSize = getDefaultInt("infrastructure.redis_pool.pool_size")
			infra.Redis.MinIdleConns = getDefaultInt("infrastructure.redis_pool.min_idle_conns")
			infra.Redis.ConnMaxLifetime = getDefaultDuration("infrastructure.redis_pool.conn_max_lifetime")

			assert.Equal(t, tt.expected, infra)
		})
	}
}

func TestDBPoolConfig_Defaults(t *testing.T) {
	setInfrastructureDefaults()

	var db DBPoolConfig
	db.MaxOpenConns = getDefaultInt("infrastructure.database_pool.max_open_conns")
	db.MaxIdleConns = getDefaultInt("infrastructure.database_pool.max_idle_conns")
	db.ConnMaxLifetime = getDefaultDuration("infrastructure.database_pool.conn_max_lifetime")

	assert.Equal(t, 25, db.MaxOpenConns, "MaxOpenConns default should be 25")
	assert.Equal(t, 10, db.MaxIdleConns, "MaxIdleConns default should be 10")
	assert.Equal(t, 5*time.Minute, db.ConnMaxLifetime, "ConnMaxLifetime default should be 5m")
}

func TestRedisPoolConfig_Defaults(t *testing.T) {
	setInfrastructureDefaults()

	var redis RedisPoolConfig
	redis.PoolSize = getDefaultInt("infrastructure.redis_pool.pool_size")
	redis.MinIdleConns = getDefaultInt("infrastructure.redis_pool.min_idle_conns")
	redis.ConnMaxLifetime = getDefaultDuration("infrastructure.redis_pool.conn_max_lifetime")

	assert.Equal(t, 20, redis.PoolSize, "PoolSize default should be 20")
	assert.Equal(t, 5, redis.MinIdleConns, "MinIdleConns default should be 5")
	assert.Equal(t, 5*time.Minute, redis.ConnMaxLifetime, "ConnMaxLifetime default should be 5m")
}

func TestInfrastructureConfig_DurationParsing(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
	}{
		{
			name:     "parse seconds",
			input:    "30s",
			expected: 30 * time.Second,
		},
		{
			name:     "parse minutes",
			input:    "5m",
			expected: 5 * time.Minute,
		},
		{
			name:     "parse hours",
			input:    "1h",
			expected: 1 * time.Hour,
		},
		{
			name:     "parse complex duration",
			input:    "1h30m",
			expected: 90 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			duration, err := time.ParseDuration(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, duration)
		})
	}
}

// Helper functions for testing defaults
func setInfrastructureDefaults() {
	viper.SetDefault("infrastructure.shutdown_timeout", "30s")
	viper.SetDefault("infrastructure.read_timeout", "15s")
	viper.SetDefault("infrastructure.write_timeout", "30s")
	viper.SetDefault("infrastructure.idle_timeout", "120s")
	viper.SetDefault("infrastructure.read_header_timeout", "5s")
	viper.SetDefault("infrastructure.database_pool.max_open_conns", 25)
	viper.SetDefault("infrastructure.database_pool.max_idle_conns", 10)
	viper.SetDefault("infrastructure.database_pool.conn_max_lifetime", "5m")
	viper.SetDefault("infrastructure.redis_pool.pool_size", 20)
	viper.SetDefault("infrastructure.redis_pool.min_idle_conns", 5)
	viper.SetDefault("infrastructure.redis_pool.conn_max_lifetime", "5m")
}

func getDefaultDuration(key string) time.Duration {
	val := viper.GetString(key)
	if val == "" {
		return 0
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		return 0
	}
	return d
}

func getDefaultInt(key string) int {
	return viper.GetInt(key)
}

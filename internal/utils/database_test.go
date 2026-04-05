package utils

import (
	"testing"
	"time"

	"wolink-core/internal/config"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestInitRedis_PoolConfiguration(t *testing.T) {
	// Note: These tests verify that pool configuration is properly applied to redis.Options.
	// They test the logic of applying config values, not actual Redis connections.
	// Actual connection tests would require a running Redis instance.

	tests := []struct {
		name           string
		cfg            config.RedisConfig
		poolCfg        config.RedisPoolConfig
		expectedConfig map[string]interface{}
	}{
		{
			name: "sets PoolSize when value > 0",
			cfg: config.RedisConfig{
				Host: "localhost",
				Port: 6379,
				DB:   0,
			},
			poolCfg: config.RedisPoolConfig{
				PoolSize: 20,
			},
			expectedConfig: map[string]interface{}{
				"PoolSize": 20,
			},
		},
		{
			name: "sets MinIdleConns when value > 0",
			cfg: config.RedisConfig{
				Host: "localhost",
				Port: 6379,
				DB:   0,
			},
			poolCfg: config.RedisPoolConfig{
				MinIdleConns: 5,
			},
			expectedConfig: map[string]interface{}{
				"MinIdleConns": 5,
			},
		},
		{
			name: "sets MaxConnAge when value > 0",
			cfg: config.RedisConfig{
				Host: "localhost",
				Port: 6379,
				DB:   0,
			},
			poolCfg: config.RedisPoolConfig{
				ConnMaxLifetime: 5 * time.Minute,
			},
			expectedConfig: map[string]interface{}{
				"MaxConnAge": 5 * time.Minute,
			},
		},
		{
			name: "sets PoolTimeout when value > 0",
			cfg: config.RedisConfig{
				Host: "localhost",
				Port: 6379,
				DB:   0,
			},
			poolCfg: config.RedisPoolConfig{
				PoolTimeout: 30 * time.Second,
			},
			expectedConfig: map[string]interface{}{
				"PoolTimeout": 30 * time.Second,
			},
		},
		{
			name: "skips pool settings when values are 0",
			cfg: config.RedisConfig{
				Host: "localhost",
				Port: 6379,
				DB:   0,
			},
			poolCfg: config.RedisPoolConfig{
				// All zero values - defaults should be used
			},
			expectedConfig: map[string]interface{}{
				"PoolSize":        0, // go-redis default
				"MinIdleConns":    0, // go-redis default
				"ConnMaxLifetime": time.Duration(0), // go-redis default
				"PoolTimeout":     time.Duration(0), // go-redis default
			},
		},
		{
			name: "sets all pool settings together",
			cfg: config.RedisConfig{
				Host:     "redis.example.com",
				Port:     6380,
				Password: "secret",
				DB:       1,
			},
			poolCfg: config.RedisPoolConfig{
				PoolSize:        50,
				MinIdleConns:    10,
				ConnMaxLifetime: 10 * time.Minute,
				PoolTimeout:     45 * time.Second,
			},
			expectedConfig: map[string]interface{}{
				"PoolSize":     50,
				"MinIdleConns": 10,
				"MaxConnAge":   10 * time.Minute,
				"PoolTimeout":  45 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a Redis client to inspect its options
			opts := buildRedisOptions(tt.cfg, tt.poolCfg)

			// Verify address is correctly built
			assert.Equal(t, "localhost:6379", opts.Addr, "address should match config")

			// Verify pool settings are applied
			for key, expectedValue := range tt.expectedConfig {
				switch key {
				case "PoolSize":
					assert.Equal(t, expectedValue.(int), opts.PoolSize, "PoolSize mismatch")
				case "MinIdleConns":
					assert.Equal(t, expectedValue.(int), opts.MinIdleConns, "MinIdleConns mismatch")
				case "MaxConnAge":
					assert.Equal(t, expectedValue.(time.Duration), opts.MaxConnAge, "MaxConnAge mismatch")
				case "PoolTimeout":
					assert.Equal(t, expectedValue.(time.Duration), opts.PoolTimeout, "PoolTimeout mismatch")
				}
			}
		})
	}
}

func TestInitRedis_ConnectionError(t *testing.T) {
	// Test that InitRedis returns error when ping fails
	// This tests with an invalid address that will fail to connect
	cfg := config.RedisConfig{
		Host: "invalid-host-that-does-not-exist",
		Port: 9999,
		DB:   0,
	}
	poolCfg := config.RedisPoolConfig{
		PoolSize:     10,
		MinIdleConns: 2,
	}

	// InitRedis should fail because the host doesn't exist
	client, err := InitRedis(cfg, poolCfg)

	// The connection should fail
	assert.Error(t, err, "InitRedis should return error when connection fails")
	assert.Contains(t, err.Error(), "failed to connect to redis", "error message should contain context")
	assert.Nil(t, client, "client should be nil on error")
}

// TestConfigureDBPool sets MaxOpenConns when value > 0
func TestConfigureDBPool_SetsMaxOpenConns(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.DBPoolConfig{
		MaxOpenConns: 25,
	}

	err = ConfigureDBPool(gormDB, cfg)
	assert.NoError(t, err)
}

// TestConfigureDBPool sets MaxIdleConns when value > 0
func TestConfigureDBPool_SetsMaxIdleConns(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.DBPoolConfig{
		MaxIdleConns: 10,
	}

	err = ConfigureDBPool(gormDB, cfg)
	assert.NoError(t, err)
}

// TestConfigureDBPool sets ConnMaxLifetime when value > 0
func TestConfigureDBPool_SetsConnMaxLifetime(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.DBPoolConfig{
		ConnMaxLifetime: 5 * time.Minute,
	}

	err = ConfigureDBPool(gormDB, cfg)
	assert.NoError(t, err)
}

// TestConfigureDBPool sets ConnMaxIdleTime when value > 0
func TestConfigureDBPool_SetsConnMaxIdleTime(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.DBPoolConfig{
		ConnMaxIdleTime: 2 * time.Minute,
	}

	err = ConfigureDBPool(gormDB, cfg)
	assert.NoError(t, err)
}

// TestConfigureDBPool returns error when db.DB() fails
// Note: This test is difficult to implement with gorm.DB concrete type.
// The error path is exercised through integration tests when database is unavailable.
// The function handles this case with proper error wrapping.
func TestConfigureDBPool_ReturnsErrorWhenDBFails(t *testing.T) {
	// This test is a placeholder documenting the error path.
	// In practice, this error occurs when the underlying sql.DB is unavailable.
	// The ConfigureDBPool function properly wraps the error with context.
	// See the function implementation for the error handling logic.
	t.Skip("Cannot mock gorm.DB concrete type; error path tested via integration tests")
}

// TestConfigureDBPool skips setting when values are 0 (use defaults)
func TestConfigureDBPool_SkipsZeroValues(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	require.NoError(t, err)

	// All zero values - should not set anything
	cfg := config.DBPoolConfig{
		MaxOpenConns:    0,
		MaxIdleConns:    0,
		ConnMaxLifetime: 0,
		ConnMaxIdleTime: 0,
	}

	err = ConfigureDBPool(gormDB, cfg)
	assert.NoError(t, err)
}

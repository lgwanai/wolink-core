package utils

import (
	"context"
	"fmt"
	"strings"

	"wolink-core/internal/config"
	"wolink-core/internal/models"

	"github.com/go-redis/redis/v8"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ConfigureDBPool configures the underlying database connection pool.
// Values of 0 are skipped, allowing Go's defaults to apply.
// This function is used to apply DBPoolConfig settings after database connection.
func ConfigureDBPool(db *gorm.DB, cfg config.DBPoolConfig) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}

	return nil
}

// buildRedisOptions creates redis.Options from the given configuration.
// It applies pool settings only when they have non-zero values, allowing
// go-redis defaults to be used when not explicitly configured.
func buildRedisOptions(cfg config.RedisConfig, poolCfg config.RedisPoolConfig) *redis.Options {
	opts := &redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	}

	// Apply pool configuration (INFRA-05)
	// Only override defaults when values are explicitly set (>0)
	if poolCfg.PoolSize > 0 {
		opts.PoolSize = poolCfg.PoolSize
	}
	if poolCfg.MinIdleConns > 0 {
		opts.MinIdleConns = poolCfg.MinIdleConns
	}
	if poolCfg.ConnMaxLifetime > 0 {
		// go-redis v8 uses MaxConnAge for connection max lifetime
		opts.MaxConnAge = poolCfg.ConnMaxLifetime
	}
	if poolCfg.PoolTimeout > 0 {
		opts.PoolTimeout = poolCfg.PoolTimeout
	}

	return opts
}

// InitDB initializes a database connection with the given configuration.
// The poolCfg parameter allows configuration of connection pool settings
// including MaxOpenConns, MaxIdleConns, ConnMaxLifetime, and ConnMaxIdleTime.
// Zero values in poolCfg will use Go's database/sql defaults.
func InitDB(cfg config.DatabaseConfig, poolCfg config.DBPoolConfig) (*gorm.DB, error) {
	var dsn string
	var dialector gorm.Dialector

	switch strings.ToLower(cfg.Type) {
	case "mysql":
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=%t&loc=%s",
			cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName,
			cfg.Charset, cfg.ParseTime, cfg.Loc)
		dialector = mysql.Open(dsn)
	case "postgres", "postgresql":
		dsn = fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s",
			cfg.Host, cfg.User, cfg.Password, cfg.DBName, cfg.Port, cfg.SSLMode)
		dialector = postgres.Open(dsn)
	case "sqlite":
		dialector = sqlite.Open(cfg.DBName)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", cfg.Type)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s database: %w", cfg.Type, err)
	}

	// Configure connection pool (INFRA-04)
	if err := ConfigureDBPool(db, poolCfg); err != nil {
		return nil, fmt.Errorf("failed to configure connection pool: %w", err)
	}

	// 自动迁移 (表已存在时忽略错误)
	if err := db.AutoMigrate(
		&models.APIKey{},
		&models.ModelRegistry{},
		&models.APIKeyModelMapping{},
		&models.Conversation{},
		&models.UsageLog{},
	); err != nil {
		// Log but don't fail - tables might already exist
		fmt.Printf("Warning: AutoMigrate failed (tables may already exist): %v\n", err)
	}

	return db, nil
}

// InitRedis initializes a Redis client with the given configuration.
// The poolCfg parameter allows configuration of connection pool settings
// including PoolSize, MinIdleConns, ConnMaxLifetime, and PoolTimeout.
// Zero values in poolCfg will use go-redis defaults.
func InitRedis(cfg config.RedisConfig, poolCfg config.RedisPoolConfig) (*redis.Client, error) {
	opts := buildRedisOptions(cfg, poolCfg)
	rdb := redis.NewClient(opts)

	// 测试连接
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return rdb, nil
}

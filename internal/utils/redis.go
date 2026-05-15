package utils

import (
	"context"
	"fmt"

	"wolink-core/internal/config"

	"github.com/go-redis/redis/v8"
)

func InitRedis(cfg config.RedisConfig, poolCfg config.RedisPoolConfig) (*redis.Client, error) {
	opts := buildRedisOptions(cfg, poolCfg)
	rdb := redis.NewClient(opts)

	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return rdb, nil
}

func buildRedisOptions(cfg config.RedisConfig, poolCfg config.RedisPoolConfig) *redis.Options {
	opts := &redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	}

	if poolCfg.PoolSize > 0 {
		opts.PoolSize = poolCfg.PoolSize
	}
	if poolCfg.MinIdleConns > 0 {
		opts.MinIdleConns = poolCfg.MinIdleConns
	}
	if poolCfg.ConnMaxLifetime > 0 {
		opts.MaxConnAge = poolCfg.ConnMaxLifetime
	}
	if poolCfg.PoolTimeout > 0 {
		opts.PoolTimeout = poolCfg.PoolTimeout
	}

	return opts
}

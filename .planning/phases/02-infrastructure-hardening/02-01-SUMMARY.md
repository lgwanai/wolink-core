# Plan 02-01: Infrastructure Configuration Structs

## Summary

Added infrastructure configuration structs for graceful shutdown timeout, HTTP server timeouts, and connection pool settings. This provides the configuration foundation that main.go and utility functions will consume.

## Changes

### Configuration Structs

**File:** `internal/config/config.go`

- `InfrastructureConfig`:
  - `ShutdownTimeout` - Duration for graceful shutdown (default: 30s)
  - `ReadTimeout` - HTTP read timeout (default: 15s)
  - `WriteTimeout` - HTTP write timeout (default: 30s)
  - `IdleTimeout` - HTTP idle timeout (default: 120s)
  - `ReadHeaderTimeout` - HTTP header read timeout (default: 5s)
  - `Database` - DBPoolConfig embedded
  - `Redis` - RedisPoolConfig embedded

- `DBPoolConfig`:
  - `MaxOpenConns` - Maximum open connections (default: 25)
  - `MaxIdleConns` - Maximum idle connections (default: 10)
  - `ConnMaxLifetime` - Connection lifetime (default: 5m)
  - `ConnMaxIdleTime` - Idle connection lifetime

- `RedisPoolConfig`:
  - `PoolSize` - Connection pool size (default: 20)
  - `MinIdleConns` - Minimum idle connections (default: 5)
  - `ConnMaxLifetime` - Connection lifetime (default: 5m)
  - `PoolTimeout` - Pool timeout

### Tests

**File:** `internal/config/infrastructure_test.go`

- `TestInfrastructureConfig_Unmarshal` - Tests YAML unmarshaling
- `TestDBPoolConfig_Defaults` - Tests database pool defaults
- `TestRedisPoolConfig_Defaults` - Tests Redis pool defaults
- `TestInfrastructureConfig_DurationParsing` - Tests duration parsing

## Requirements Met

- INFRA-01: Graceful shutdown configuration
- INFRA-04: Database connection pool configuration
- INFRA-05: Redis connection pool configuration
- INFRA-06: HTTP server timeout configuration

## Verification

```bash
go test -v ./internal/config/... -run TestInfrastructure
```

All tests pass.

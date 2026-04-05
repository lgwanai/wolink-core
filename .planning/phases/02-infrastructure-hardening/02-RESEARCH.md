# Phase 2: Infrastructure Hardening - Research

**Researched:** 2026-04-05
**Domain:** Go HTTP server hardening, graceful shutdown, health endpoints, connection pooling
**Confidence:** HIGH (based on codebase analysis and established Go standard library patterns)

## Summary

Phase 2 focuses on infrastructure hardening for the wolink-core AI Gateway to handle production traffic with proper resource management and health visibility. The current codebase has a basic graceful shutdown implementation but lacks proper health/readiness endpoints, connection pool configuration, and HTTP server timeouts. This research identifies the standard stack, patterns, and pitfalls for implementing INFRA-01 through INFRA-06.

**Primary recommendation:** Extend the existing graceful shutdown with configurable drain timeout, add separate /health and /ready endpoints for Kubernetes probes, configure connection pools for both database and Redis, and set HTTP server timeouts to prevent resource exhaustion.

<phase_requirements>

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| INFRA-01 | Server implements graceful shutdown with configurable drain timeout (30s default) | Go `signal.Notify` + `context.WithTimeout`; current main.go has basic implementation |
| INFRA-02 | `/health` endpoint returns liveness status (server running) | Simple endpoint returning 200 OK; Gin handler |
| INFRA-03 | `/ready` endpoint checks database and Redis connectivity | Ping both DB and Redis; return 200 or 503 |
| INFRA-04 | Database connection pool configured with max open/idle connections and lifetime | GORM wraps `database/sql`; use `sql.DB` methods `SetMaxOpenConns`, `SetMaxIdleConns`, `SetConnMaxLifetime` |
| INFRA-05 | Redis connection pool configured with pool size and idle connections | go-redis `Options` struct: `PoolSize`, `MinIdleConns`, `ConnMaxLifetime` |
| INFRA-06 | HTTP server configured with read/write/idle timeouts | `http.Server` struct fields: `ReadTimeout`, `WriteTimeout`, `IdleTimeout`, `ReadHeaderTimeout` |

</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| net/http | Go 1.21 stdlib | HTTP server with timeouts | Standard library; no dependencies |
| database/sql | Go 1.21 stdlib | Connection pooling interface | Standard interface wrapped by GORM |
| GORM | v1.25.4 | Database ORM with pool access | Already in use; `DB.DB()` exposes `*sql.DB` |
| go-redis | v8.11.5 | Redis client with built-in pooling | Already in use; `Options` struct |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| testify | v1.11.1 | Testing assertions | Unit tests for health endpoints |
| context | Go stdlib | Timeout/cancellation | Graceful shutdown with deadline |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Manual health endpoints | grpc-health-check | Overkill for HTTP API; keep simple |
| go-redis v9 | go-redis v8 | v8 already in use; migration unnecessary for pool config |
| Custom shutdown manager | uber-go/fx | Adds complexity; current pattern works |

**Installation:**
```bash
# No new dependencies required
# All features use existing libraries or Go standard library
```

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── config/
│   └── config.go           # Add InfrastructureConfig struct
├── api/
│   ├── routes.go           # Add /health and /ready routes
│   └── handlers/
│       └── health_handler.go  # NEW - health/readiness handlers
└── utils/
    ├── database.go         # Add connection pool configuration
    └── redis.go            # NEW - Redis connection pool configuration (or extend database.go)

cmd/
└── main.go                 # Add timeout config, enhance shutdown
```

### Pattern 1: Configurable Graceful Shutdown

**What:** Server handles SIGTERM/SIGINT with configurable drain timeout, allowing in-flight requests to complete before shutdown.

**When to use:** All production servers - mandatory for Kubernetes deployments.

**Example:**
```go
// Source: Go standard library pattern + Kubernetes best practices
// cmd/main.go

func main() {
    cfg, err := config.Load()
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }
    cfg.MustValidate()  // From Phase 1

    // ... initialization ...

    srv := &http.Server{
        Addr:              ":" + cfg.Server.Port,
        Handler:           router,
        ReadTimeout:       cfg.Infrastructure.ReadTimeout,
        WriteTimeout:      cfg.Infrastructure.WriteTimeout,
        IdleTimeout:       cfg.Infrastructure.IdleTimeout,
        ReadHeaderTimeout: cfg.Infrastructure.ReadHeaderTimeout,
    }

    // Start server in goroutine
    go func() {
        logger.Infof("Server starting on port %s", cfg.Server.Port)
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            logger.Fatalf("Failed to start server: %v", err)
        }
    }()

    // Wait for interrupt signal
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    logger.Info("Shutting down server...")

    // Stop accepting new requests (service manager)
    serviceManager.Stop()

    // Graceful shutdown with configurable timeout (INFRA-01)
    ctx, cancel := context.WithTimeout(context.Background(), cfg.Infrastructure.ShutdownTimeout)
    defer cancel()

    if err := srv.Shutdown(ctx); err != nil {
        // Timeout exceeded, force close
        logger.Warnf("Server forced to shutdown: %v", err)
        srv.Close()
    }

    logger.Info("Server exited")
}
```

**Configuration addition:**
```go
// internal/config/config.go

type InfrastructureConfig struct {
    ShutdownTimeout  time.Duration `mapstructure:"shutdown_timeout"`
    ReadTimeout      time.Duration `mapstructure:"read_timeout"`
    WriteTimeout     time.Duration `mapstructure:"write_timeout"`
    IdleTimeout      time.Duration `mapstructure:"idle_timeout"`
    ReadHeaderTimeout time.Duration `mapstructure:"read_header_timeout"`
    Database         DBPoolConfig  `mapstructure:"database_pool"`
    Redis            RedisPoolConfig `mapstructure:"redis_pool"`
}

type DBPoolConfig struct {
    MaxOpenConns    int           `mapstructure:"max_open_conns"`
    MaxIdleConns    int           `mapstructure:"max_idle_conns"`
    ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
    ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
}

type RedisPoolConfig struct {
    PoolSize       int           `mapstructure:"pool_size"`
    MinIdleConns   int           `mapstructure:"min_idle_conns"`
    MaxIdleConns   int           `mapstructure:"max_idle_conns"`  // go-redis v8 uses this
    ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
    PoolTimeout    time.Duration `mapstructure:"pool_timeout"`
}
```

### Pattern 2: Health and Readiness Endpoints

**What:** Separate `/health` (liveness) and `/ready` (readiness) endpoints for Kubernetes probes.

**When to use:** All services deployed to Kubernetes.

**Example:**
```go
// Source: Kubernetes health check best practices
// internal/api/handlers/health_handler.go

package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "gorm.io/gorm"
    "github.com/go-redis/redis/v8"
)

type HealthHandler struct {
    db    *gorm.DB
    redis *redis.Client
}

func NewHealthHandler(db *gorm.DB, redis *redis.Client) *HealthHandler {
    return &HealthHandler{db: db, redis: redis}
}

// Health returns liveness status (INFRA-02)
// This endpoint only checks if the server is running, not dependencies.
// Used by Kubernetes liveness probe.
func (h *HealthHandler) Health(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
        "status": "ok",
    })
}

// Ready returns readiness status (INFRA-03)
// This endpoint checks if the server can handle requests by verifying
// critical dependencies (database and Redis).
// Used by Kubernetes readiness probe.
func (h *HealthHandler) Ready(c *gin.Context) {
    ctx := c.Request.Context()
    checks := make(map[string]bool)
    allHealthy := true

    // Check database connectivity
    sqlDB, err := h.db.DB()
    if err != nil {
        checks["database"] = false
        allHealthy = false
    } else if err := sqlDB.PingContext(ctx); err != nil {
        checks["database"] = false
        allHealthy = false
    } else {
        checks["database"] = true
    }

    // Check Redis connectivity
    if err := h.redis.Ping(ctx).Err(); err != nil {
        checks["redis"] = false
        allHealthy = false
    } else {
        checks["redis"] = true
    }

    if allHealthy {
        c.JSON(http.StatusOK, gin.H{
            "status": "ready",
            "checks": checks,
        })
    } else {
        c.JSON(http.StatusServiceUnavailable, gin.H{
            "status": "not_ready",
            "checks": checks,
        })
    }
}
```

**Routes integration:**
```go
// internal/api/routes.go

func SetupRoutes(serviceManager *services.ServiceManager, logger *logrus.Logger, cfg *config.Config) *gin.Engine {
    router := gin.New()

    // Health endpoints (not protected, for Kubernetes probes)
    healthHandler := handlers.NewHealthHandler(serviceManager.DB, serviceManager.Redis)

    // INFRA-02: Liveness probe
    router.GET("/health", healthHandler.Health)

    // INFRA-03: Readiness probe
    router.GET("/ready", healthHandler.Ready)

    // ... rest of routes ...

    return router
}
```

**Kubernetes deployment example:**
```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
```

### Pattern 3: Database Connection Pool Configuration

**What:** Configure GORM's underlying `*sql.DB` connection pool with appropriate limits.

**When to use:** All database connections in production.

**Example:**
```go
// Source: database/sql best practices + GORM documentation
// internal/utils/database.go

import (
    "time"
    "gorm.io/gorm"
)

func ConfigureDBPool(db *gorm.DB, cfg config.DBPoolConfig) error {
    sqlDB, err := db.DB()
    if err != nil {
        return fmt.Errorf("failed to get underlying sql.DB: %w", err)
    }

    // INFRA-04: Connection pool configuration

    // MaxOpenConns: Maximum number of open connections to the database
    // Default: 0 (unlimited) - MUST set for production
    // Recommendation: Start with 25 for typical workloads
    if cfg.MaxOpenConns > 0 {
        sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
    }

    // MaxIdleConns: Maximum number of connections in the idle connection pool
    // Default: 2 - often too low for production
    // Recommendation: Set to same as MaxOpenConns for steady workloads
    if cfg.MaxIdleConns > 0 {
        sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
    }

    // ConnMaxLifetime: Maximum amount of time a connection may be reused
    // Default: 0 (unlimited) - SHOULD set for MySQL/PostgreSQL
    // Recommendation: 5-30 minutes to handle database-side timeouts
    if cfg.ConnMaxLifetime > 0 {
        sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
    }

    // ConnMaxIdleTime: Maximum amount of time a connection may be idle
    // Default: 0 (unlimited)
    // Recommendation: Set to 5-15 minutes to release unused connections
    if cfg.ConnMaxIdleTime > 0 {
        sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
    }

    return nil
}

// Update InitDB to accept pool config
func InitDB(cfg config.DatabaseConfig, poolCfg config.DBPoolConfig) (*gorm.DB, error) {
    // ... existing connection code ...

    // Apply pool configuration (INFRA-04)
    if err := ConfigureDBPool(db, poolCfg); err != nil {
        return nil, fmt.Errorf("failed to configure connection pool: %w", err)
    }

    return db, nil
}
```

### Pattern 4: Redis Connection Pool Configuration

**What:** Configure go-redis connection pool with appropriate limits.

**When to use:** All Redis connections in production.

**Example:**
```go
// Source: go-redis documentation + production best practices
// internal/utils/redis.go

import (
    "time"
    "github.com/go-redis/redis/v8"
)

// InitRedis creates a Redis client with connection pool configuration (INFRA-05)
func InitRedis(cfg config.RedisConfig, poolCfg config.RedisPoolConfig) (*redis.Client, error) {
    opts := &redis.Options{
        Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
        Password: cfg.Password,
        DB:       cfg.DB,

        // INFRA-05: Connection pool configuration
    }

    // PoolSize: Maximum number of socket connections
    // Default: 10 * runtime.NumCPU() for v8
    // Recommendation: 10-50 depending on load
    if poolCfg.PoolSize > 0 {
        opts.PoolSize = poolCfg.PoolSize
    }

    // MinIdleConns: Minimum number of idle connections
    // Default: 0
    // Recommendation: Set to a fraction of PoolSize for steady load
    if poolCfg.MinIdleConns > 0 {
        opts.MinIdleConns = poolCfg.MinIdleConns
    }

    // MaxIdleConns (v8): Maximum idle connections
    // Note: In v8, this limits total idle connections
    if poolCfg.MaxIdleConns > 0 {
        opts.MaxIdleConns = poolCfg.MaxIdleConns
    }

    // ConnMaxLifetime: Connection age before closing
    // Recommendation: 10-30 minutes
    if poolCfg.ConnMaxLifetime > 0 {
        opts.ConnMaxLifetime = poolCfg.ConnMaxLifetime
    }

    // PoolTimeout: Time to wait for a connection from pool
    // Default: 4 seconds + ReadTimeout
    // Recommendation: 1-5 seconds for production
    if poolCfg.PoolTimeout > 0 {
        opts.PoolTimeout = poolCfg.PoolTimeout
    }

    rdb := redis.NewClient(opts)

    // Test connection
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := rdb.Ping(ctx).Err(); err != nil {
        return nil, fmt.Errorf("failed to connect to redis: %w", err)
    }

    return rdb, nil
}
```

### Pattern 5: HTTP Server Timeout Configuration

**What:** Configure HTTP server timeouts to prevent resource exhaustion and slow-loris attacks.

**When to use:** All production HTTP servers.

**Example:**
```go
// Source: Cloudflare blog + Go net/http best practices
// cmd/main.go

import (
    "net/http"
    "time"
)

// INFRA-06: HTTP server with timeouts
srv := &http.Server{
    Addr:              ":" + cfg.Server.Port,
    Handler:           router,

    // ReadTimeout: Maximum duration for reading the entire request
    // Protection against slowloris attacks, large request bodies
    // Recommendation: 5-30 seconds depending on expected request sizes
    ReadTimeout:       cfg.Infrastructure.ReadTimeout,       // e.g., 15 * time.Second

    // WriteTimeout: Maximum duration before timing out writes
    // Protection against slow clients, streaming timeouts
    // Recommendation: 30-60 seconds, longer for streaming endpoints
    WriteTimeout:      cfg.Infrastructure.WriteTimeout,      // e.g., 30 * time.Second

    // IdleTimeout: Maximum amount of time to wait for the next request
    // Controls keep-alive behavior, frees resources
    // Recommendation: 60-120 seconds
    IdleTimeout:       cfg.Infrastructure.IdleTimeout,       // e.g., 120 * time.Second

    // ReadHeaderTimeout: Maximum duration for reading request headers
    // Protection against slowloris attacks on headers
    // Recommendation: 1-5 seconds
    ReadHeaderTimeout: cfg.Infrastructure.ReadHeaderTimeout, // e.g., 5 * time.Second
}

// Default values in config.setDefaults():
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
```

### Anti-Patterns to Avoid

- **Missing timeouts on HTTP server:** Allows slowloris attacks and resource exhaustion. Always set `ReadTimeout`, `WriteTimeout`, `IdleTimeout`, and `ReadHeaderTimeout`.
- **Unlimited connection pools:** Default `MaxOpenConns` is 0 (unlimited) in `database/sql`. This can exhaust database connections.
- **Too few idle connections:** Default `MaxIdleConns` is 2, causing connection churn under load.
- **Single health endpoint:** Using `/health` for both liveness and readiness causes unnecessary pod restarts when dependencies are temporarily unavailable.
- **No shutdown timeout:** Without a deadline, shutdown can hang indefinitely. Kubernetes sends SIGKILL after terminationGracePeriodSeconds anyway.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Graceful shutdown | Custom channel logic | `signal.Notify` + `context.WithTimeout` | Standard library handles edge cases |
| Health endpoints | Complex health check system | Simple Gin handlers | Kubernetes expects simple 200/503 responses |
| Connection pooling | Manual connection management | `database/sql` + go-redis built-in pools | Battle-tested, handles reconnection, cleanup |
| HTTP timeouts | Middleware-based timeouts | `http.Server` struct fields | Server-level timeouts protect at TCP level |

**Key insight:** All infrastructure hardening features are built into Go standard library and existing dependencies. No new packages needed.

## Common Pitfalls

### Pitfall 1: Missing ReadHeaderTimeout

**What goes wrong:** Without `ReadHeaderTimeout`, the server is vulnerable to slowloris attacks where clients send headers very slowly, consuming server resources indefinitely.

**Why it happens:** Often overlooked in timeout configuration; older Go tutorials don't mention it.

**How to avoid:**
1. Always set `ReadHeaderTimeout` to 1-5 seconds
2. This is separate from `ReadTimeout` and protects the header reading phase
3. Add to config validation: require non-zero timeouts in production

**Warning signs:**
- `http.Server{}` without `ReadHeaderTimeout`
- Server accepts connections but never reads headers

### Pitfall 2: Health vs Readiness Confusion

**What goes wrong:** Using `/health` for both liveness and readiness causes Kubernetes to restart pods when dependencies (DB, Redis) are temporarily unavailable, even though the pod itself is healthy.

**Why it happens:** Misunderstanding the difference between liveness (is the process running?) and readiness (can it handle traffic?).

**How to avoid:**
1. `/health` (liveness): Only check if the HTTP server is running - return 200 OK always
2. `/ready` (readiness): Check all critical dependencies - return 503 if any fail
3. This allows Kubernetes to stop routing traffic without restarting the pod

**Warning signs:**
- Single `/health` endpoint checking database connectivity
- Kubernetes restarting pods during database maintenance

### Pitfall 3: Default Connection Pool Values

**What goes wrong:** `database/sql` defaults to unlimited open connections and only 2 idle connections. This causes connection churn (thrashing) under load and can exhaust database connection limits.

**Why it happens:** Developers assume defaults are sensible for production.

**How to avoid:**
1. Always set `MaxOpenConns` (start with 25-50 for typical workloads)
2. Set `MaxIdleConns` to at least half of `MaxOpenConns`
3. Set `ConnMaxLifetime` to 5-30 minutes to handle database-side timeouts
4. Set `ConnMaxIdleTime` to release unused connections

**Warning signs:**
- No pool configuration in code
- "too many connections" errors from database
- High latency spikes under load from connection establishment

### Pitfall 4: Shutdown Without Drain

**What goes wrong:** Immediately closing connections on shutdown terminates in-flight requests, causing errors for users.

**Why it happens:** Using `srv.Close()` instead of `srv.Shutdown(ctx)` or not waiting for goroutines.

**How to avoid:**
1. Use `srv.Shutdown(ctx)` with a timeout context
2. Stop accepting new requests first (load balancer/ingress)
3. Wait for in-flight requests with timeout
4. Only call `srv.Close()` if timeout exceeded

**Warning signs:**
- `srv.Close()` called directly
- No context timeout for shutdown
- Client errors during deployments

## Code Examples

Verified patterns from codebase analysis:

### Current State (cmd/main.go:54-57)
```go
// Current HTTP server - NO timeouts configured
srv := &http.Server{
    Addr:    ":" + cfg.Server.Port,
    Handler: router,
}
// PROBLEM: Missing ReadTimeout, WriteTimeout, IdleTimeout, ReadHeaderTimeout
```

### Current State (cmd/main.go:67-85)
```go
// Current graceful shutdown - works but timeout is hardcoded
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
<-quit

logger.Info("Shutting down server...")
serviceManager.Stop()

// Hardcoded 30 second timeout - should be configurable
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

if err := srv.Shutdown(ctx); err != nil {
    logger.Fatalf("Server forced to shutdown: %v", err)
}
```

### Current State (internal/utils/database.go:58-71)
```go
// Current Redis init - NO pool configuration
func InitRedis(cfg config.RedisConfig) (*redis.Client, error) {
    rdb := redis.NewClient(&redis.Options{
        Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
        Password: cfg.Password,
        DB:       cfg.DB,
        // PROBLEM: No PoolSize, MinIdleConns, ConnMaxLifetime
    })
    // ...
}
```

### Current State (internal/api/routes.go:28-30)
```go
// Current health endpoint - basic, but used for both liveness and readiness
router.GET("/health", func(c *gin.Context) {
    c.JSON(200, gin.H{"status": "ok"})
})
// PROBLEM: No /ready endpoint for readiness probe
// PROBLEM: Health doesn't distinguish liveness from readiness
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| No HTTP timeouts | All timeouts configured | Go 1.8+ | Prevents resource exhaustion |
| Single health endpoint | Separate /health and /ready | Kubernetes 1.8+ | Better pod lifecycle management |
| Unlimited connections | Configured pools | Always best practice | Predictable resource usage |
| Hardcoded shutdown timeout | Configurable timeout | Production readiness | Tunable drain period |

**Deprecated/outdated:**
- `srv.Close()` for shutdown: Use `srv.Shutdown(ctx)` for graceful drain
- Health checks only at startup: Use liveness/readiness probes for continuous monitoring
- Default pool settings: Always configure pools for production

## Open Questions

1. **Should the health endpoint check the server's internal state beyond just running?**
   - What we know: Kubernetes uses liveness probes to detect deadlocks
   - What's unclear: Whether to add goroutine count, memory, or other checks
   - Recommendation: Keep `/health` simple (process running). Add internal metrics to `/ready` or metrics endpoint.

2. **What is the appropriate WriteTimeout for streaming responses (SSE/chat completions)?**
   - What we know: Chat completions can stream for minutes
   - What's unclear: How to handle long-lived streaming connections with WriteTimeout
   - Recommendation: Use `http.ResponseWriter` flushing for streaming; consider per-endpoint timeout middleware for streaming endpoints, or use very long WriteTimeout with context-based cancellation for individual requests.

3. **Should connection pool settings be different for MySQL vs PostgreSQL?**
   - What we know: Both support connection pooling, but have different optimal settings
   - What's unclear: Whether to have database-type-specific defaults
   - Recommendation: Use same configuration structure with documentation on tuning per database type. PostgreSQL may benefit from longer `ConnMaxLifetime` due to connection costs.

## Validation Architecture

> nyquist_validation is enabled in .planning/config.json

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go standard testing + testify (already installed) |
| Config file | None - use TestMain for setup |
| Quick run command | `go test -v ./internal/utils/... ./internal/api/handlers/...` |
| Full suite command | `go test -race -cover ./...` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| INFRA-01 | Graceful shutdown completes in-flight requests | unit | `go test -v ./cmd/... -run TestGracefulShutdown` | Wave 0 |
| INFRA-01 | Shutdown timeout forces close | unit | `go test -v ./cmd/... -run TestShutdownTimeout` | Wave 0 |
| INFRA-02 | `/health` returns 200 OK | unit | `go test -v ./internal/api/handlers/... -run TestHealth` | Wave 0 |
| INFRA-03 | `/ready` returns 200 when DB and Redis connected | unit | `go test -v ./internal/api/handlers/... -run TestReady_Healthy` | Wave 0 |
| INFRA-03 | `/ready` returns 503 when DB disconnected | unit | `go test -v ./internal/api/handlers/... -run TestReady_DBDown` | Wave 0 |
| INFRA-03 | `/ready` returns 503 when Redis disconnected | unit | `go test -v ./internal/api/handlers/... -run TestReady_RedisDown` | Wave 0 |
| INFRA-04 | Database pool configured with correct limits | unit | `go test -v ./internal/utils/... -run TestDBPool` | Wave 0 |
| INFRA-05 | Redis pool configured with correct limits | unit | `go test -v ./internal/utils/... -run TestRedisPool` | Wave 0 |
| INFRA-06 | HTTP server has all timeouts set | unit | `go test -v ./cmd/... -run TestServerTimeouts` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test -v ./...`
- **Per wave merge:** `go test -race -cover ./...`
- **Phase gate:** Full suite green with 80%+ coverage for new code

### Wave 0 Gaps
- [ ] `internal/api/handlers/health_handler_test.go` - covers INFRA-02, INFRA-03
- [ ] `internal/utils/database_test.go` - covers INFRA-04 (add pool tests)
- [ ] `internal/utils/redis_test.go` - covers INFRA-05 (new file)
- [ ] `cmd/main_test.go` or integration tests - covers INFRA-01, INFRA-06
- [ ] `internal/config/config_test.go` - extend for InfrastructureConfig validation

*(Existing test files can be extended; new handlers need new test files)*

## Sources

### Primary (HIGH confidence)
- Go standard library: `net/http`, `database/sql`, `context` - built-in timeout and pooling support
- GORM v1.25.4 documentation - `DB.DB()` exposes `*sql.DB` for pool configuration
- go-redis v8 documentation - `Options` struct for pool configuration
- Codebase analysis: `cmd/main.go`, `internal/utils/database.go`, `internal/api/routes.go`

### Secondary (MEDIUM confidence)
- Kubernetes documentation: Liveness, Readiness, and Startup Probes
- Go blog: "So you want to expose Go on the Internet"
- Cloudflare blog: "The complete guide to Golang net/http timeouts"

### Tertiary (LOW confidence)
- Community recommendations for pool size values (context-dependent)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - all features use Go stdlib or existing dependencies
- Architecture: HIGH - straightforward additions to existing patterns
- Pitfalls: HIGH - well-documented production issues with established solutions

**Research date:** 2026-04-05
**Valid until:** 30 days - stable Go patterns with no breaking changes expected

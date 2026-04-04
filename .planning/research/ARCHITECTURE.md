# Architecture Patterns for Production Go Systems

**Domain:** Production Readiness for Go AI Gateway
**Researched:** 2026-04-04
**Confidence:** MEDIUM (Based on training data without web verification)

## Recommended Architecture

### Component Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     External Clients                         │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│                   API Gateway Layer                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   HTTP       │  │  Health      │  │  Metrics     │      │
│  │   Server     │  │  Endpoints   │  │  Endpoint    │      │
│  └──────┬───────┘  └──────────────┘  └──────────────┘      │
│         │                                                    │
│  ┌──────▼────────────────────────────────────────────┐      │
│  │            Middleware Chain                        │      │
│  │  RequestID → Recovery → Logger → Auth → RateLimit │      │
│  └──────┬────────────────────────────────────────────┘      │
└─────────┼───────────────────────────────────────────────────┘
          │
          ▼
┌─────────────────────────────────────────────────────────────┐
│                  Handler Layer (Existing)                    │
│  ChatHandler | AdminHandler | PluginHandler                  │
└───────┬─────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────┐
│                  Service Layer (Existing)                    │
│  AuthService | ModelService | PluginService | QueueService   │
│  ModelConfigService | SecurityService | UsageService         │
└───────┬─────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────┐
│              Infrastructure Layer (Existing)                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   Database   │  │    Redis     │  │   Plugins    │      │
│  │  (GORM)      │  │   (Cache)    │  │  (OpenAI)    │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
└─────────────────────────────────────────────────────────────┘
```

### Component Boundaries

| Component | Responsibility | Communicates With | State |
|-----------|---------------|-------------------|-------|
| **HTTP Server** | Accept connections, route requests | Middleware chain, handlers | Stateless |
| **Middleware Chain** | Cross-cutting concerns | Handlers, services | Stateless |
| **Handlers** | Request validation, response formatting | Services | Stateless |
| **Services** | Business logic orchestration | Database, Redis, plugins | Stateful (workers) |
| **Queue Workers** | Async task processing | Database, Redis | Stateful (goroutines) |
| **Health Checker** | System health monitoring | All components | Stateful (checks) |
| **Graceful Shutdown** | Orchestrate safe termination | All components | Stateful (shutdown state) |

## Production Readiness Components

### 1. Graceful Shutdown (ENHANCEMENT NEEDED)

**Current State:** Basic implementation exists in `cmd/main.go`
**Gaps:** Missing component-level shutdown coordination

**Enhanced Pattern:**

```
Shutdown Sequence:
1. Signal received (SIGINT/SIGTERM)
2. Stop accepting new requests (health check returns unhealthy)
3. Wait for in-flight requests to complete (with timeout)
4. Stop background workers (queue processors)
5. Close database connections
6. Close Redis connections
7. Exit
```

**Implementation Components:**

| Component | Purpose | Location |
|-----------|---------|----------|
| `ShutdownManager` | Coordinates shutdown sequence | New: `internal/utils/shutdown.go` |
| `ServiceManager.Stop()` | Stop all services | Existing: enhance `internal/services/service_manager.go` |
| `QueueService.Stop()` | Stop background workers | Existing: enhance `internal/services/queue_service.go` |
| Context propagation | Cancel in-flight operations | Throughout all layers |

**Context Timeout Strategy:**

```go
// Recommended timeouts for shutdown phases
const (
    DrainTimeout        = 30 * time.Second  // Wait for in-flight requests
    WorkerStopTimeout   = 10 * time.Second  // Stop background workers
    DBCloseTimeout      = 5 * time.Second   // Close database connections
    RedisCloseTimeout   = 3 * time.Second   // Close Redis connections
)
```

### 2. Health Check Endpoints (NEW COMPONENT)

**Purpose:** Enable orchestration systems (Kubernetes, load balancers) to monitor service health

**Required Endpoints:**

| Endpoint | Purpose | When to Use |
|----------|---------|-------------|
| `/health` | Liveness probe | Container restart if failed |
| `/ready` | Readiness probe | Remove from load balancer if failed |
| `/health/detailed` | Detailed status (optional) | Monitoring dashboards |

**Health Check Components:**

```
┌─────────────────────────────────────┐
│         Health Checker              │
│  ┌───────────────────────────────┐  │
│  │  Checkers Registry            │  │
│  │  - DBHealthChecker            │  │
│  │  - RedisHealthChecker         │  │
│  │  - PluginHealthChecker        │  │
│  └───────────────────────────────┘  │
│  ┌───────────────────────────────┐  │
│  │  Aggregator                   │  │
│  │  - Combine results            │  │
│  │  - Determine overall status   │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
```

**Implementation:**

```go
// Location: internal/api/handlers/health_handler.go
type HealthHandler struct {
    checkers []HealthChecker
}

type HealthChecker interface {
    Name() string
    Check(ctx context.Context) HealthStatus
}

type HealthStatus struct {
    Status  string                 // "healthy", "degraded", "unhealthy"
    Details map[string]interface{} // Component-specific details
}
```

**Checkers to Implement:**

1. **DBHealthChecker** - Ping database, check connection pool stats
2. **RedisHealthChecker** - Ping Redis, check connection status
3. **PluginHealthChecker** - Check if at least one model provider is available

### 3. Request Tracing (NEW COMPONENT)

**Purpose:** Enable debugging and monitoring by propagating request IDs through all layers

**Pattern:**

```
Request Flow with Tracing:
1. Middleware generates or extracts Request ID
2. Request ID stored in context
3. All log entries include Request ID
4. All service calls pass context
5. External API calls include Request ID in headers
6. Responses include Request ID header
```

**Implementation Components:**

| Component | Purpose | Location |
|-----------|---------|----------|
| `RequestIDMiddleware` | Generate/extract request ID | New: `internal/api/middleware/request_id.go` |
| `ContextKey` type | Type-safe context keys | New: `internal/utils/context.go` |
| `LoggingMiddleware` enhancement | Include request ID in logs | Existing: enhance logging |
| Plugin calls | Pass request ID to external APIs | Existing: enhance `internal/plugins/*.go` |

**Context Propagation Pattern:**

```go
// Throughout all layers
func (s *Service) DoSomething(ctx context.Context, ...) error {
    requestID := ctx.Value(utils.RequestIDKey).(string)
    s.logger.WithField("request_id", requestID).Info("operation")
    // ...
}

// External API calls
req.Header.Set("X-Request-ID", requestID)
```

### 4. Middleware Architecture (ENHANCEMENT NEEDED)

**Current State:** Auth middleware exists, but missing key production middleware

**Recommended Middleware Chain:**

```
Order matters! Execution order:
1. RequestID      - Generate/extract request ID
2. Recovery       - Panic recovery (prevent crashes)
3. Logger        - Request logging
4. CORS          - CORS handling
5. RateLimit     - Rate limiting
6. Auth          - Authentication
7. Authorization - Authorization checks
```

**Middleware Components to Add:**

| Middleware | Purpose | Priority |
|------------|---------|----------|
| **RequestIDMiddleware** | Generate/propagate request IDs | HIGH - Needed for debugging |
| **RecoveryMiddleware** | Panic recovery | HIGH - Prevent crashes |
| **CORSMiddleware** | CORS handling (enhance existing) | MEDIUM - Security |
| **RateLimitMiddleware** | Request rate limiting | HIGH - Protect resources |

**Implementation Pattern:**

```go
// Location: internal/api/middleware/
// Each middleware follows this pattern:
func SomeMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // 1. Pre-processing
        // 2. Call next handler
        // 3. Post-processing (if needed)
    }
}
```

### 5. Observability Integration (NEW COMPONENT)

**Purpose:** Enable monitoring, alerting, and performance analysis

**Three Pillars:**

```
┌─────────────────────────────────────────────────────────────┐
│                    Observability Stack                       │
├─────────────────────┬───────────────────┬───────────────────┤
│       Metrics       │       Traces      │      Logs         │
│  (Prometheus format)│  (OpenTelemetry)  │  (Structured)     │
├─────────────────────┼───────────────────┼───────────────────┤
│ - Request rate      │ - Request spans   │ - JSON format     │
│ - Latency buckets   │ - DB queries      │ - Request ID      │
│ - Error rate        │ - External calls  │ - Context fields  │
│ - Queue depth       │ - Service calls   │ - Logrus (exists) │
└─────────────────────┴───────────────────┴───────────────────┘
```

**Metrics to Expose:**

| Metric | Type | Purpose |
|--------|------|---------|
| `http_requests_total` | Counter | Total requests by method, path, status |
| `http_request_duration_seconds` | Histogram | Request latency distribution |
| `http_requests_in_flight` | Gauge | Current active requests |
| `queue_depth` | Gauge | Queue backlog size |
| `db_connections_active` | Gauge | Active DB connections |
| `model_provider_requests_total` | Counter | Requests to each model provider |

**Implementation:**

```go
// Location: internal/utils/metrics.go
type MetricsCollector struct {
    registry *prometheus.Registry
    // ... metric definitions
}

// Expose endpoint: GET /metrics (Prometheus scraper)
```

### 6. Enhanced Error Handling (ENHANCEMENT NEEDED)

**Current State:** Basic error handling exists
**Gaps:** No structured error types, no error categorization

**Error Architecture:**

```
┌─────────────────────────────────────┐
│         Error Types                 │
├─────────────────────────────────────┤
│ AppError (base)                     │
│  ├─ ValidationError (400)           │
│  ├─ AuthError (401)                 │
│  ├─ ForbiddenError (403)            │
│  ├─ NotFoundError (404)             │
│  ├─ RateLimitError (429)            │
│  └─ InternalError (500)             │
└─────────────────────────────────────┘
```

**Implementation Pattern:**

```go
// Location: internal/utils/errors.go
type AppError struct {
    Code       string                 // Machine-readable code
    Message    string                 // User-friendly message
    HTTPStatus int                    // HTTP status code
    Details    map[string]interface{} // Additional context
    Cause      error                  // Wrapped error
}

// Handler pattern
func (h *Handler) Handle(c *gin.Context) {
    if err := h.service.DoSomething(c.Request.Context(), ...); err != nil {
        var appErr *utils.AppError
        if errors.As(err, &appErr) {
            c.JSON(appErr.HTTPStatus, appErr)
            return
        }
        // Unknown error - wrap as internal error
        c.JSON(500, utils.NewInternalError(err))
        return
    }
}
```

## Data Flow Patterns

### Request Flow with Production Components

```
Client Request
    │
    ▼
[RequestID Middleware] ─────► Generate X-Request-ID
    │
    ▼
[Recovery Middleware] ──────► Panic recovery wrapper
    │
    ▼
[Logger Middleware] ────────► Log request start
    │
    ▼
[CORS Middleware] ──────────► Handle CORS
    │
    ▼
[RateLimit Middleware] ─────► Check rate limits (Redis)
    │
    ▼
[Auth Middleware] ──────────► Validate API key (Redis/DB)
    │
    ▼
[Handler] ──────────────────► Process request
    │                          │
    │                          ▼
    │                       [Service Layer]
    │                          │
    │                          ▼
    │                       [External Plugin] ──► Call with X-Request-ID
    │                          │
    │                          ▼
    │                       [Queue Service] ──► Async task (with request ID)
    │
    ▼
[Logger Middleware] ────────► Log request end (with duration)
    │
    ▼
Client Response (with X-Request-ID header)
```

### Graceful Shutdown Flow

```
Signal (SIGTERM/SIGINT)
    │
    ▼
[ShutdownManager.Start()]
    │
    ├─► Set health check to "unhealthy"
    │
    ├─► Stop accepting new connections
    │
    ├─► Wait for in-flight requests (30s timeout)
    │   └─► Context cancellation propagates through middleware
    │
    ├─► Stop background workers
    │   ├─► QueueService.Stop() (10s timeout)
    │   └─► Wait for workers to finish
    │
    ├─► Close database connections (5s timeout)
    │
    ├─► Close Redis connections (3s timeout)
    │
    └─► Exit
```

### Health Check Flow

```
Request: GET /ready
    │
    ▼
[HealthHandler.Ready()]
    │
    ├─► DBHealthChecker.Check()
    │   └─► SELECT 1 (with 1s timeout)
    │
    ├─► RedisHealthChecker.Check()
    │   └─► PING (with 1s timeout)
    │
    ├─► PluginHealthChecker.Check()
    │   └─► Check cached health status
    │
    └─► Aggregate results
        │
        ├─► All healthy → 200 OK
        ├─► Some degraded → 200 OK (but log warning)
        └─► Any unhealthy → 503 Service Unavailable
```

## Patterns to Follow

### Pattern 1: Context Propagation

**What:** Pass context through all layers for cancellation and tracing
**When:** Every function that does I/O or calls other services
**Example:**

```go
// Handler
func (h *Handler) Handle(c *gin.Context) {
    ctx := c.Request.Context()
    // Add request ID to context
    ctx = context.WithValue(ctx, utils.RequestIDKey, requestID)

    result, err := h.service.DoSomething(ctx, params)
    // ...
}

// Service
func (s *Service) DoSomething(ctx context.Context, params Params) (Result, error) {
    // Check if context is cancelled
    if err := ctx.Err(); err != nil {
        return Result{}, err
    }

    // Pass context to database
    var model Model
    if err := s.db.WithContext(ctx).First(&model, id).Error; err != nil {
        return Result{}, err
    }

    // Pass context to external calls
    resp, err := s.plugin.Call(ctx, request)
    // ...
}
```

### Pattern 2: Graceful Shutdown with Wait Groups

**What:** Use sync.WaitGroup to wait for goroutines to finish
**When:** Any component that spawns goroutines
**Example:**

```go
type QueueService struct {
    wg     sync.WaitGroup
    stopCh chan struct{}
}

func (qs *QueueService) Start() {
    qs.wg.Add(1)
    go qs.worker()
}

func (qs *QueueService) Stop() {
    close(qs.stopCh)  // Signal workers to stop
    qs.wg.Wait()      // Wait for all workers to finish
}

func (qs *QueueService) worker() {
    defer qs.wg.Done()
    for {
        select {
        case <-qs.stopCh:
            return
        case task := <-qs.queue:
            qs.process(task)
        }
    }
}
```

### Pattern 3: Circuit Breaker for External Calls

**What:** Prevent cascading failures when external services are unhealthy
**When:** Calls to external APIs (model providers)
**Example:**

```go
// Location: internal/plugins/circuit_breaker.go
type CircuitBreaker struct {
    maxFailures   int
    timeout       time.Duration
    state         State // Closed, Open, HalfOpen
    failureCount  int
    lastFailTime  time.Time
}

func (cb *CircuitBreaker) Call(fn func() error) error {
    if cb.state == Open {
        if time.Since(cb.lastFailTime) > cb.timeout {
            cb.state = HalfOpen
        } else {
            return ErrCircuitOpen
        }
    }

    err := fn()
    if err != nil {
        cb.failureCount++
        if cb.failureCount >= cb.maxFailures {
            cb.state = Open
            cb.lastFailTime = time.Now()
        }
        return err
    }

    cb.failureCount = 0
    cb.state = Closed
    return nil
}
```

## Anti-Patterns to Avoid

### Anti-Pattern 1: Ignoring Context Cancellation

**What:** Long-running operations don't check context
**Why bad:** Prevents graceful shutdown, wastes resources
**Instead:**

```go
// BAD
func (s *Service) Process() {
    for i := 0; i < 1000; i++ {
        s.doWork(i)
    }
}

// GOOD
func (s *Service) Process(ctx context.Context) error {
    for i := 0; i < 1000; i++ {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            if err := s.doWork(ctx, i); err != nil {
                return err
            }
        }
    }
    return nil
}
```

### Anti-Pattern 2: Global State for Request Context

**What:** Using global variables to store request-specific data
**Why bad:** Race conditions, impossible to test, breaks concurrency
**Instead:** Use context and dependency injection

### Anti-Pattern 3: Logging Without Request ID

**What:** Log messages that can't be traced back to a request
**Why bad:** Makes debugging production issues nearly impossible
**Instead:** Always include request ID in log entries

### Anti-Pattern 4: Blocking Shutdown on External Calls

**What:** Waiting indefinitely for external API responses during shutdown
**Why bad:** Extends shutdown time, may exceed Kubernetes termination grace period
**Instead:** Use context with timeout for all external calls

## Scalability Considerations

| Concern | At 100 users | At 10K users | At 1M users |
|---------|--------------|--------------|-------------|
| **Health checks** | Single endpoint, simple checks | Add cache for check results | Distributed health aggregation |
| **Request tracing** | In-process context | Add distributed tracing (Jaeger) | Sampling + aggregation |
| **Metrics** | Local Prometheus | Prometheus federation | Remote write to Thanos |
| **Graceful shutdown** | 30s timeout sufficient | Longer timeout for in-flight | Rolling updates with pod disruption budgets |
| **Queue depth** | In-memory queue check | Redis-based monitoring | Separate queue monitoring service |

## Build Order Implications

Based on component dependencies, recommended implementation order:

### Phase 1: Foundation (No external dependencies)
1. **Request ID Middleware** - Enables all other observability
2. **Recovery Middleware** - Prevents crashes, improves stability
3. **Enhanced Error Types** - Improves error handling throughout

### Phase 2: Observability (Depends on Phase 1)
4. **Health Check Endpoints** - Enables orchestration integration
5. **Metrics Collection** - Depends on request ID for correlation
6. **Enhanced Logging** - Include request ID in all logs

### Phase 3: Resilience (Depends on Phase 1, 2)
7. **Enhanced Graceful Shutdown** - Needs health checks to work properly
8. **Circuit Breaker** - For external plugin calls
9. **Rate Limiting Enhancement** - Needs metrics for monitoring

### Phase 4: Advanced (Depends on all above)
10. **Distributed Tracing** (OpenTelemetry) - Requires all foundations
11. **Advanced Monitoring** - Dashboards, alerts

## Component Interaction Matrix

```
                 │ HTTP │ Mid │ Hand │ Svc │ Queue │ DB │ Redis │ Plugin │
─────────────────┼──────┼─────┼──────┼─────┼───────┼────┼───────┼────────┤
GracefulShutdown │  X   │     │      │  X  │   X   │ X  │   X   │        │
HealthCheck      │      │     │  X   │  X  │       │ X  │   X   │   X    │
RequestTracing   │  X   │  X  │  X   │  X  │   X   │ X  │   X   │   X    │
Metrics          │  X   │  X  │      │  X  │   X   │ X  │   X   │   X    │
ErrorHandling    │      │  X  │  X   │  X  │   X   │ X  │   X   │   X    │
```

## Testing Implications

Each production component requires specific testing:

| Component | Unit Tests | Integration Tests | E2E Tests |
|-----------|------------|-------------------|-----------|
| Graceful shutdown | Mock services, verify sequence | Real services, signal handling | Kubernetes pod termination |
| Health checks | Mock checkers, verify aggregation | Real DB/Redis, check timeouts | Load balancer integration |
| Request tracing | Context propagation, ID generation | Middleware chain, log verification | Distributed trace end-to-end |
| Metrics | Counter increments, gauge updates | Real metrics endpoint | Prometheus scrape |
| Error handling | Error type creation, wrapping | Handler error responses | Client error handling |

## Configuration Requirements

New configuration parameters needed:

```yaml
# configs/config.yaml
server:
  shutdown_timeout: 30s
  read_timeout: 30s
  write_timeout: 30s

health:
  enabled: true
  path: /health
  ready_path: /ready
  detailed_path: /health/detailed
  check_timeout: 1s

tracing:
  enabled: true
  header: X-Request-ID

metrics:
  enabled: true
  path: /metrics

circuit_breaker:
  max_failures: 5
  timeout: 30s
```

## Sources

**Confidence: MEDIUM** - Based on training data without web verification. Patterns are well-established in Go community but specific versions and best practices should be verified against:

- Go 1.21+ documentation for context and signal handling
- Gin framework documentation for middleware patterns
- OpenTelemetry Go SDK documentation for tracing
- Prometheus Go client documentation for metrics
- Kubernetes documentation for health probes and graceful shutdown

**Recommended verification:**
- Check Context7 for latest OpenTelemetry Go patterns
- Verify Gin middleware best practices with official docs
- Confirm Prometheus metric naming conventions

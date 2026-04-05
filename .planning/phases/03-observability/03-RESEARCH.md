# Phase 3: Observability - Research

**Researched:** 2026-04-05
**Domain:** Go/Gin HTTP observability (request tracing, structured logging, Prometheus metrics, error standardization)
**Confidence:** HIGH

## Summary

This phase implements comprehensive observability for the AI Gateway, enabling production monitoring and debugging capabilities. The research covers four key areas: request ID middleware for distributed tracing, structured JSON logging enhancement, Prometheus metrics exposure, and error type standardization with consistent HTTP status mapping.

**Primary recommendation:** Use battle-tested libraries (gin-contrib/requestid, prometheus/client_golang) with minimal custom code. Extend existing logrus logger with context-aware fields. Create a domain-specific error type hierarchy for consistent API responses.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/gin-contrib/requestid | v1.0.0 | Request ID middleware for Gin | Official Gin contributor package, integrates seamlessly |
| github.com/prometheus/client_golang | v1.18.0 | Prometheus metrics client | Official Prometheus Go client, production-grade |
| github.com/google/uuid | v1.3.0 | UUID generation | Already in project, no new dependency |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/sirupsen/logrus | v1.9.3 | Structured logging | Already in project - extend with context fields |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| gin-contrib/requestid | Custom middleware | More code to maintain, reinvents wheel |
| prometheus/client_golang | Custom metrics endpoint | Loses Prometheus ecosystem compatibility |
| logrus | zap, zerolog | logrus already in project; switching adds migration cost |

**Installation:**
```bash
go get github.com/gin-contrib/requestid@v1.0.0
go get github.com/prometheus/client_golang@v1.18.0
```

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── api/
│   ├── middleware/
│   │   ├── requestid.go      # Request ID middleware wrapper
│   │   ├── logging.go        # Enhanced structured logging
│   │   ├── metrics.go        # Prometheus metrics middleware
│   │   └── ...existing...
│   ├── handlers/
│   │   ├── metrics_handler.go # /metrics endpoint handler
│   │   └── ...existing...
├── observability/
│   ├── logger.go             # Context-aware logger with request ID
│   ├── metrics.go            # Prometheus metrics definitions
│   └── errors.go             # Domain error types with HTTP mapping
```

### Pattern 1: Request ID Middleware

**What:** Generates or propagates X-Request-ID header through all layers
**When to use:** All HTTP requests - mandatory for production tracing
**Example:**
```go
// Using gin-contrib/requestid - wraps existing functionality
import "github.com/gin-contrib/requestid"

func SetupRoutes(sm *services.ServiceManager, logger *logrus.Logger, cfg *config.Config) *gin.Engine {
    router := gin.New()

    // Request ID must be first middleware
    router.Use(requestid.New())

    // Now Logger can access request ID via c.GetString("X-Request-ID")
    router.Use(middleware.LoggerWithRequestID(logger))
    // ...
}

// Custom wrapper for enhanced logging
func LoggerWithRequestID(logger *logrus.Logger) gin.HandlerFunc {
    return func(c *gin.Context) {
        requestID := c.GetString("X-Request-ID")
        start := time.Now()

        // Process request
        c.Next()

        // Log with request ID
        logger.WithFields(logrus.Fields{
            "request_id":  requestID,
            "status":      c.Writer.Status(),
            "method":      c.Request.Method,
            "path":        c.Request.URL.Path,
            "latency_ms":  time.Since(start).Milliseconds(),
            "client_ip":   c.ClientIP(),
        }).Info("request completed")

        // Add request ID to response header
        c.Header("X-Request-ID", requestID)
    }
}
```

### Pattern 2: Context-Aware Structured Logging

**What:** Logger that extracts request ID from context automatically
**When to use:** All service-layer logging for trace correlation
**Example:**
```go
// internal/observability/logger.go
package observability

import (
    "context"

    "github.com/sirupsen/logrus"
)

type contextKey string

const requestIDKey contextKey = "requestID"

// ContextLogger wraps logrus with context extraction
type ContextLogger struct {
    *logrus.Logger
}

// WithContext creates an entry with request ID from context
func (l *ContextLogger) WithContext(ctx context.Context) *logrus.Entry {
    entry := l.WithFields(logrus.Fields{
        "timestamp": time.Now().UTC().Format(time.RFC3339),
    })

    if requestID, ok := ctx.Value(requestIDKey).(string); ok && requestID != "" {
        entry = entry.WithField("request_id", requestID)
    }

    return entry
}

// WithRequestID adds request ID to context
func WithRequestID(ctx context.Context, requestID string) context.Context {
    return context.WithValue(ctx, requestIDKey, requestID)
}
```

### Pattern 3: Prometheus Metrics Middleware

**What:** HTTP middleware collecting request rate, latency, and error metrics
**When to use:** All API endpoints - standard production monitoring
**Example:**
```go
// internal/observability/metrics.go
package observability

import (
    "strconv"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
    // Request counter by method, path, status
    httpRequestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "http_requests_total",
            Help: "Total number of HTTP requests",
        },
        []string{"method", "path", "status"},
    )

    // Request latency histogram
    httpRequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "http_request_duration_seconds",
            Help:    "HTTP request latency in seconds",
            Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
        },
        []string{"method", "path"},
    )

    // Active requests gauge
    httpRequestsInFlight = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "http_requests_in_flight",
            Help: "Current number of HTTP requests being processed",
        },
        []string{"method"},
    )
)

func init() {
    prometheus.MustRegister(httpRequestsTotal)
    prometheus.MustRegister(httpRequestDuration)
    prometheus.MustRegister(httpRequestsInFlight)
}

// PrometheusMiddleware returns a Gin middleware for collecting metrics
func PrometheusMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        path := c.FullPath()
        if path == "" {
            path = "unknown"
        }

        // Track in-flight requests
        httpRequestsInFlight.WithLabelValues(c.Request.Method).Inc()
        defer httpRequestsInFlight.WithLabelValues(c.Request.Method).Dec()

        start := time.Now()

        c.Next()

        duration := time.Since(start).Seconds()
        status := strconv.Itoa(c.Writer.Status())

        httpRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
        httpRequestDuration.WithLabelValues(c.Request.Method, path).Observe(duration)
    }
}

// Handler returns the Prometheus HTTP handler
func Handler() gin.HandlerFunc {
    return gin.WrapH(promhttp.Handler())
}
```

### Pattern 4: Domain Error Types with HTTP Mapping

**What:** Structured error types that map to consistent HTTP status codes
**When to use:** All service-layer error returns
**Example:**
```go
// internal/observability/errors.go
package observability

import (
    "net/http"
)

// AppError represents a domain error with HTTP status
type AppError struct {
    Code       string // Machine-readable error code
    Message    string // Human-readable message
    HTTPStatus int    // HTTP status code
    Cause      error  // Underlying error (optional)
}

func (e *AppError) Error() string {
    if e.Cause != nil {
        return e.Message + ": " + e.Cause.Error()
    }
    return e.Message
}

func (e *AppError) Unwrap() error {
    return e.Cause
}

// Predefined error types
var (
    ErrUnauthorized = &AppError{
        Code:       "UNAUTHORIZED",
        Message:    "authentication required",
        HTTPStatus: http.StatusUnauthorized,
    }

    ErrForbidden = &AppError{
        Code:       "FORBIDDEN",
        Message:    "access denied",
        HTTPStatus: http.StatusForbidden,
    }

    ErrNotFound = &AppError{
        Code:       "NOT_FOUND",
        Message:    "resource not found",
        HTTPStatus: http.StatusNotFound,
    }

    ErrBadRequest = &AppError{
        Code:       "BAD_REQUEST",
        Message:    "invalid request",
        HTTPStatus: http.StatusBadRequest,
    }

    ErrRateLimited = &AppError{
        Code:       "RATE_LIMITED",
        Message:    "rate limit exceeded",
        HTTPStatus: http.StatusTooManyRequests,
    }

    ErrInternal = &AppError{
        Code:       "INTERNAL_ERROR",
        Message:    "internal server error",
        HTTPStatus: http.StatusInternalServerError,
    }
)

// NewValidationError creates a validation error with details
func NewValidationError(message string) *AppError {
    return &AppError{
        Code:       "VALIDATION_ERROR",
        Message:    message,
        HTTPStatus: http.StatusBadRequest,
    }
}

// NewServiceError creates a service-level error
func NewServiceError(code, message string, status int, cause error) *AppError {
    return &AppError{
        Code:       code,
        Message:    message,
        HTTPStatus: status,
        Cause:      cause,
    }
}

// ErrorHandler middleware converts AppError to JSON responses
func ErrorHandler() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Next()

        // Check for errors
        if len(c.Errors) > 0 {
            err := c.Errors.Last().Err

            var appErr *AppError
            if errors.As(err, &appErr) {
                c.JSON(appErr.HTTPStatus, gin.H{
                    "error": gin.H{
                        "code":    appErr.Code,
                        "message": appErr.Message,
                    },
                })
                return
            }

            // Fallback to internal error
            c.JSON(http.StatusInternalServerError, gin.H{
                "error": gin.H{
                    "code":    "INTERNAL_ERROR",
                    "message": "an unexpected error occurred",
                },
            })
        }
    }
}
```

### Anti-Patterns to Avoid

- **String-based request ID in logs only:** Must propagate via context for service-layer access
- **Global logger without context:** Each log entry must include request ID for correlation
- **Custom metrics format:** Use Prometheus format for ecosystem compatibility
- **HTTP status in error messages:** Separate status code from error content

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Request ID generation | Custom UUID middleware | gin-contrib/requestid | Battle-tested, handles header propagation |
| Prometheus exposition | Custom text format | promhttp.Handler | Official implementation, handles content negotiation |
| Metrics collection | Custom counters/histograms | prometheus client_golang | Handles concurrency, exposition, race conditions |

**Key insight:** Observability infrastructure has many edge cases (concurrent access, memory efficiency, format compatibility). Use established libraries.

## Common Pitfalls

### Pitfall 1: Request ID Not Propagated to Services
**What goes wrong:** Request ID only in middleware logs, service layer logs are uncorrelated
**Why it happens:** Developer forgets to pass context with request ID to service calls
**How to avoid:** Store request ID in gin.Context, create helper to extract into standard context
**Warning signs:** Service logs show no request_id field

### Pitfall 2: High Cardinality Metrics Labels
**What goes wrong:** Using request path with IDs as metric label (e.g., `/users/123/profile`)
**Why it happens:** Using c.Request.URL.Path instead of c.FullPath()
**How to avoid:** Always use c.FullPath() for metric labels, which returns the route pattern
**Warning signs:** Prometheus memory grows rapidly, metrics endpoint slow

### Pitfall 3: Histogram Buckets Misconfigured
**What goes wrong:** Default buckets don't match API latency profile
**Why it happens:** Using Prometheus defaults designed for general HTTP servers
**How to avoid:** Define buckets matching AI API latency: .01, .05, .1, .25, .5, 1, 2.5, 5, 10 seconds
**Warning signs:** All requests fall into one or two buckets

### Pitfall 4: Error Types Not Wrapped Properly
**What goes wrong:** AppError.Cause not preserved when wrapping errors
**Why it happens:** Using fmt.Errorf instead of error wrapping with %w
**How to avoid:** Always use %w verb for wrapping: `fmt.Errorf("operation failed: %w", err)`
**Warning signs:** Stack traces incomplete, root cause lost

## Code Examples

### Request ID Middleware Integration
```go
// internal/api/middleware/requestid.go
package middleware

import (
    "github.com/gin-contrib/requestid"
    "github.com/gin-gonic/gin"
)

// RequestID wraps gin-contrib/requestid for consistent configuration
func RequestID() gin.HandlerFunc {
    return requestid.New(
        requestid.WithGenerator(func() string {
            return uuid.New().String()
        }),
        requestid.WithCustomHeaderStrKey("X-Request-ID"),
    )
}
```

### Complete Routes Setup with Observability
```go
// internal/api/routes.go (updated)
func SetupRoutes(sm *services.ServiceManager, logger *logrus.Logger, cfg *config.Config) *gin.Engine {
    router := gin.New()

    // 1. Recovery first (catches panics)
    router.Use(gin.Recovery())

    // 2. Request ID (must be early for other middleware)
    router.Use(middleware.RequestID())

    // 3. Prometheus metrics (before logging to count all requests)
    router.Use(observability.PrometheusMiddleware())

    // 4. Structured logging with request ID
    router.Use(middleware.LoggerWithRequestID(logger))

    // 5. CORS
    router.Use(middleware.CORS())

    // 6. Error handler
    router.Use(observability.ErrorHandler())

    // Health endpoints
    healthHandler := handlers.NewHealthHandler(sm.DB, sm.Redis)
    router.GET("/health", healthHandler.Health)
    router.GET("/ready", healthHandler.Ready)

    // Metrics endpoint (no auth required)
    router.GET("/metrics", observability.Handler())

    // ... rest of routes
}
```

### Service Layer Logging with Context
```go
// internal/services/auth_service.go (updated pattern)
func (s *AuthService) ValidateAPIKey(ctx context.Context, keyID string) (*models.APIKey, error) {
    log := observability.FromContext(s.logger, ctx)

    log.WithField("key_prefix", keyID[:8]).Debug("validating API key")

    apiKey, err := s.getAPIKeyFromDB(keyID)
    if err != nil {
        log.WithError(err).Warn("API key validation failed")
        return nil, observability.ErrUnauthorized
    }

    return apiKey, nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Text logging | Structured JSON logging | Industry standard | Machine-parseable logs |
| Request ID in logs only | Request ID in context + logs | 2020+ | Correlation across services |
| Custom metrics | Prometheus exposition | 2016+ | Ecosystem integration |
| HTTP status in error strings | Error type with status field | 2018+ | Type-safe error handling |

**Deprecated/outdated:**
- logrus.TextFormatter: Use JSONFormatter for production
- Manual error string parsing: Use typed errors with codes
- Custom metrics exposition: Use Prometheus client

## Open Questions

1. **Should metrics endpoint require authentication?**
   - What we know: /health and /ready have no auth; security requirements are high
   - What's unclear: If metrics should be exposed to monitoring systems without auth
   - Recommendation: No auth on /metrics but restrict via network policy (not exposed publicly)

2. **Should we add request ID to existing database logs?**
   - What we know: Database operations use logger without context
   - What's unclear: Scope of logging enhancement in this phase
   - Recommendation: Add request ID to service-layer DB operations, not GORM internals

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | testify (stretchr/testify v1.11.1) |
| Config file | None - uses Go conventions |
| Quick run command | `go test -race -cover ./internal/observability/...` |
| Full suite command | `go test -race -cover ./...` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| OBS-01 | Request ID generated and propagated | unit | `go test -race ./internal/api/middleware/... -run TestRequestID` | No - Wave 0 |
| OBS-02 | Structured JSON logging with request ID | unit | `go test -race ./internal/observability/... -run TestLogger` | No - Wave 0 |
| OBS-03 | /metrics endpoint exposes Prometheus metrics | unit | `go test -race ./internal/api/handlers/... -run TestMetrics` | No - Wave 0 |
| OBS-04 | Errors map to consistent HTTP status | unit | `go test -race ./internal/observability/... -run TestAppError` | No - Wave 0 |

### Sampling Rate
- **Per task commit:** `go test -race -cover ./internal/observability/... ./internal/api/middleware/...`
- **Per wave merge:** `go test -race -cover ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/observability/logger_test.go` - tests for context-aware logger
- [ ] `internal/observability/metrics_test.go` - tests for Prometheus metrics
- [ ] `internal/observability/errors_test.go` - tests for AppError types
- [ ] `internal/api/middleware/requestid_test.go` - tests for request ID middleware
- [ ] `internal/api/middleware/logging_test.go` - tests for structured logging middleware
- [ ] Framework install: `go get github.com/gin-contrib/requestid github.com/prometheus/client_golang` - new dependencies

## Sources

### Primary (HIGH confidence)
- Project codebase analysis (go.mod, existing middleware patterns, handlers)
- gin-contrib/requestid - official Gin contributor middleware
- prometheus/client_golang - official Prometheus Go client

### Secondary (MEDIUM confidence)
- Standard Go error handling patterns (errors.As, error wrapping)
- Standard Gin middleware patterns (existing auth.go, cors.go implementations)

### Tertiary (LOW confidence)
- None - all recommendations based on project codebase and standard library patterns

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Libraries are battle-tested, versions confirmed in go.mod compatibility
- Architecture: HIGH - Patterns follow existing project structure and Go conventions
- Pitfalls: HIGH - Common issues well-documented in Prometheus and Gin communities

**Research date:** 2026-04-05
**Valid until:** 30 days (stable ecosystem)

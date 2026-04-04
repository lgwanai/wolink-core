# Domain Pitfalls - Go Production Readiness

**Domain:** Go HTTP services / AI Gateway
**Researched:** 2026-04-04
**Confidence:** HIGH (based on established Go best practices)

---

## Critical Pitfalls

Mistakes that cause security vulnerabilities, data loss, or major production incidents.

### Pitfall 1: Ignoring Graceful Shutdown

**What goes wrong:**
Server stops accepting new connections immediately upon SIGTERM, terminating in-flight requests mid-processing. This causes:
- Partial data corruption (database writes cut off mid-transaction)
- Failed API responses to clients during deployments
- Connection pool resources not released properly

**Why it happens:**
Default `http.Server.ListenAndServe()` does not handle OS signals. Many developers assume the orchestration layer (Kubernetes, Docker) handles this, but it doesn't - it just sends SIGTERM and waits.

**Consequences:**
- Zero-downtime deployments impossible
- User-facing errors during every deployment
- Database connection leaks until timeouts expire
- Cloud provider health checks fail during rollout

**Prevention:**
```go
srv := &http.Server{Addr: ":8080"}

// Signal handling
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

go func() {
    <-quit
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    srv.Shutdown(ctx) // Waits for active requests
}()

srv.ListenAndServe()
```

**Detection:**
- No signal handling code in `main.go`
- No `http.Server.Shutdown()` call anywhere
- Deployment documentation mentions "connection refused" errors

**Phase to address:** Infrastructure / Production Hardening

---

### Pitfall 2: Goroutine Leaks from Unbounded Spawning

**What goes wrong:**
Goroutines are spawned for each request or task but never terminate properly, causing memory to grow unbounded until OOM kill.

**Why it happens:**
- Goroutines are cheap, so developers spawn them freely
- Missing context cancellation propagation
- Blocked channel operations without select/timeout
- Infinite loops in goroutines

**Consequences:**
- Memory grows linearly with traffic
- Server becomes unresponsive under load
- Eventually OOM killed by orchestrator
- Restart loop in production

**Prevention:**
```go
// WRONG: Unbounded spawn
for _, item := range items {
    go processItem(item) // May never finish
}

// CORRECT: Context propagation
ctx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
defer cancel()

for _, item := range items {
    go processItem(ctx, item) // Context ensures termination
}

// In processItem:
select {
case <-ctx.Done():
    return ctx.Err()
case result := <-processing:
    return result
}
```

**Detection:**
- Use `runtime.NumGoroutine()` in health checks
- Monitor goroutine count in metrics
- Profile with `net/http/pprof` under load
- Look for goroutines without context parameters

**Phase to address:** Performance Optimization

---

### Pitfall 3: Database Connection Pool Exhaustion

**What goes wrong:**
Connection pool reaches max connections, all subsequent requests hang waiting for connections. System appears "frozen" but is actually deadlocked.

**Why it happens:**
- Default `SetMaxOpenConns` is unlimited (0) in Go 1.21+
- Connections not returned due to unclosed rows/statements
- Long-running queries holding connections
- Missing context timeouts on queries

**Consequences:**
- All API requests hang indefinitely
- Health checks fail (if they need DB)
- Cascade failures across dependent services
- Requires restart to recover

**Prevention:**
```go
db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
sqlDB, _ := db.DB()

// Set pool limits based on load testing
sqlDB.SetMaxOpenConns(25)        // Match your DB's max_connections
sqlDB.SetMaxIdleConns(10)        // Keep some warm
sqlDB.SetConnMaxLifetime(5 * time.Minute) // Prevent stale connections
sqlDB.SetConnMaxIdleTime(10 * time.Minute)

// Always use context with timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
db.WithContext(ctx).Find(&users)
```

**Detection:**
- No connection pool configuration found
- Queries without `WithContext()`
- `rows.Close()` missing after `db.Raw().Rows()`
- Database metrics show connection count = max_open_conns

**Phase to address:** Performance Optimization

---

### Pitfall 4: Context Cancellation Not Propagated

**What goes wrong:**
Request is cancelled (client disconnects, timeout), but backend operations continue wasting resources on dead requests.

**Why it happens:**
- Using `context.Background()` instead of request context
- Not passing context to database calls
- Goroutines spawned without context
- External API calls without context

**Consequences:**
- Wasted CPU/memory on abandoned requests
- Backend services processing dead requests
- Billing for API calls after client gave up
- Delayed graceful shutdown (waiting for zombie work)

**Prevention:**
```go
// WRONG: Ignoring request context
func (s *Service) ProcessData(w http.ResponseWriter, r *http.Request) {
    data := s.fetchFromDB() // No context, can't be cancelled
}

// CORRECT: Propagate context everywhere
func (s *Service) ProcessData(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    data, err := s.fetchFromDB(ctx) // Cancellable
}

func (s *Service) fetchFromDB(ctx context.Context) (Data, error) {
    return db.WithContext(ctx).Find(&data)
}
```

**Detection:**
- Database calls without `WithContext()`
- HTTP client calls without context
- Goroutines spawned from handlers without context
- Functions taking `context.Context` but ignoring it

**Phase to address:** Code Structure / Performance Optimization

---

### Pitfall 5: Secret Management Anti-Patterns

**What goes wrong:**
Sensitive credentials (JWT secrets, API keys, database passwords) stored in code or use weak defaults that make it to production.

**Why it happens:**
- Developer convenience during development
- "I'll change it before production" mentality
- Configuration files committed to git
- Missing validation at startup

**Consequences:**
- JWT tokens can be forged (admin access compromised)
- Database accessible if credentials leak
- API keys stolen from code repository
- Compliance violations (SOC2, PCI-DSS)

**Prevention:**
```go
// WRONG: Default secrets
viper.SetDefault("security.jwt_secret", "your-secret-key")

// CORRECT: Fail without configuration
func MustGetSecret(key string) string {
    val := os.Getenv(key)
    if val == "" {
        log.Fatalf("required secret %s not configured", key)
    }
    if len(val) < 32 {
        log.Fatalf("secret %s too short (min 32 chars)", key)
    }
    return val
}

// In startup
jwtSecret := MustGetSecret("JWT_SECRET")
```

**Detection:**
- `SetDefault()` calls for secrets
- Secrets hardcoded in config files
- No validation of secret length/complexity
- `.env` files in git history

**Phase to address:** Security Hardening (immediate priority)

---

## Moderate Pitfalls

### Pitfall 6: Missing Request Timeout Configuration

**What goes wrong:**
HTTP clients and servers have no timeout or extremely long timeouts. Slow/failing backends cause thread exhaustion and cascade failures.

**Why it happens:**
- Default `http.Client` has no timeout
- Developers fear "false positives" from short timeouts
- Timeout not tested under real network conditions

**Consequences:**
- Slow backend takes down entire system
- Connection pool exhaustion
- Poor user experience (hanging requests)
- Difficult to debug (no clear error)

**Prevention:**
```go
// Server timeout
srv := &http.Server{
    Addr:              ":8080",
    ReadTimeout:       10 * time.Second,
    WriteTimeout:      30 * time.Second, // Allow for streaming
    IdleTimeout:       120 * time.Second,
    ReadHeaderTimeout: 5 * time.Second,
}

// Client timeout
client := &http.Client{
    Timeout: 30 * time.Second,
    Transport: &http.Transport{
        ResponseHeaderTimeout: 10 * time.Second,
    },
}
```

**Detection:**
- No `ReadTimeout`/`WriteTimeout` on http.Server
- Default `http.Get()` used (no timeout)
- API calls to external services without timeout context

**Phase to address:** Performance Optimization

---

### Pitfall 7: Improper Error Handling and Logging

**What goes wrong:**
Errors are silently swallowed, or logged without context. Debugging production issues becomes impossible.

**Why it happens:**
- `if err != nil { log.Println(err); return }` pattern
- No structured logging
- Missing request context in logs
- Error messages with sensitive data

**Consequences:**
- Production debugging requires code changes
- Cannot trace request flow
- Security incidents not detectable
- Compliance failures (audit trails missing)

**Prevention:**
```go
// Use structured logging (slog in Go 1.21+)
import "log/slog"

// Add request context
func (h *Handler) Process(w http.ResponseWriter, r *http.Request) {
    logger := slog.With(
        "request_id", r.Header.Get("X-Request-ID"),
        "user_id", getUserID(r),
    )

    data, err := h.service.Process(r.Context(), input)
    if err != nil {
        logger.Error("process failed",
            "error", err,
            "input", sanitizedInput,
        )
        // Return generic error to client
        respondError(w, "processing failed")
        return
    }
}

// Wrap errors with context
return fmt.Errorf("fetching user %d: %w", userID, err)
```

**Detection:**
- `fmt.Println` or `log.Println` scattered in code
- No structured logging library used
- Errors returned without context (`return err` instead of `return fmt.Errorf("context: %w", err)`)
- Request ID not propagated to logs

**Phase to address:** Code Structure / Production Hardening

---

### Pitfall 8: CORS Misconfiguration

**What goes wrong:**
CORS set to `*` (allow all origins) in production, enabling any website to call your API from user browsers.

**Why it happens:**
- Quick fix during development
- "We'll lock it down later" mentality
- Misunderstanding CORS security implications

**Consequences:**
- CSRF attacks possible
- Data exfiltration via malicious sites
- Session hijacking if credentials included
- Compliance failures

**Prevention:**
```go
// Configurable CORS
func CORSMiddleware(allowedOrigins []string) gin.HandlerFunc {
    return func(c *gin.Context) {
        origin := c.GetHeader("Origin")

        // Check against allowlist
        for _, allowed := range allowedOrigins {
            if origin == allowed {
                c.Header("Access-Control-Allow-Origin", origin)
                break
            }
        }

        c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")
        c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
        c.Header("Access-Control-Allow-Credentials", "true")

        if c.Request.Method == "OPTIONS" {
            c.AbortWithStatus(204)
            return
        }
        c.Next()
    }
}

// Load from config
allowedOrigins := viper.GetStringSlice("cors.allowed_origins")
// Fail if not configured in production
if len(allowedOrigins) == 0 && isProduction() {
    log.Fatal("CORS origins must be configured in production")
}
```

**Detection:**
- `Access-Control-Allow-Origin: *` in production code
- No origin allowlist validation
- CORS middleware doesn't check environment

**Phase to address:** Security Hardening (immediate priority)

---

### Pitfall 9: Missing Health Check Endpoints

**What goes wrong:**
Load balancers and orchestrators cannot detect unhealthy instances, routing traffic to broken servers.

**Why it happens:**
- Assumption that "server is up = healthy"
- Missing dependency checks
- Health endpoint not included in initial design

**Consequences:**
- Failed deployments not detected
- Traffic routed to dead instances
- Cascade failures in dependent services
- Poor Kubernetes/Docker orchestration

**Prevention:**
```go
// Liveness: Is the server running?
router.GET("/health", func(c *gin.Context) {
    c.JSON(200, gin.H{"status": "ok"})
})

// Readiness: Can the server handle requests?
router.GET("/ready", func(c *gin.Context) {
    checks := map[string]bool{}

    // Check database
    if err := db.Ping(); err != nil {
        checks["database"] = false
    } else {
        checks["database"] = true
    }

    // Check Redis
    if _, err := redis.Ping().Result(); err != nil {
        checks["redis"] = false
    } else {
        checks["redis"] = true
    }

    // All must pass
    allHealthy := true
    for _, v := range checks {
        if !v {
            allHealthy = false
        }
    }

    if allHealthy {
        c.JSON(200, checks)
    } else {
        c.JSON(503, checks)
    }
})
```

**Detection:**
- No `/health` or `/ready` endpoints
- Health checks don't verify dependencies
- Load balancer config points to application root

**Phase to address:** Production Hardening

---

### Pitfall 10: Inadequate Input Validation

**What goes wrong:**
User input trusted without validation, leading to injection attacks, buffer overflows, or business logic bypass.

**Why it happens:**
- Assumption "GORM handles SQL injection"
- Naive pattern matching (e.g., checking for "DROP TABLE")
- Validation only on frontend
- Trust of internal API consumers

**Consequences:**
- SQL injection despite ORM usage
- XSS via stored content
- DoS via unbounded inputs
- Business logic bypass

**Prevention:**
```go
// Use structured validation (go-playground/validator)
type CreateAPIKeyRequest struct {
    Name        string `json:"name" binding:"required,min=1,max=100"`
    Department  string `json:"department" binding:"required,oneof=eng sales marketing"`
    ExpiresDays int    `json:"expires_days" binding:"omitempty,min=1,max=365"`
}

// Sanitize free-form input
func sanitizeInput(input string) string {
    // Remove control characters
    input = strings.Map(func(r rune) rune {
        if unicode.IsControl(r) && r != '\n' && r != '\t' {
            return -1
        }
        return r
    }, input)
    return input
}

// Validate against business rules
func (s *Service) ValidateAPIKey(req *CreateAPIKeyRequest) error {
    if !s.validDepartments[req.Department] {
        return fmt.Errorf("invalid department: %s", req.Department)
    }
    return nil
}
```

**Detection:**
- String input directly used in queries (even with GORM)
- No validation library in dependencies
- Request structs without validation tags
- "DROP TABLE" string matching (anti-pattern)

**Phase to address:** Security Hardening

---

## Minor Pitfalls

### Pitfall 11: No Request Size Limits

**What goes wrong:**
Clients can send arbitrarily large request bodies, causing memory exhaustion or DoS.

**Prevention:**
```go
// In Gin
router.Use(func(c *gin.Context) {
    c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 10*1024*1024) // 10MB
    c.Next()
})
```

**Detection:** No `MaxBytesReader` or body size limit configured.

**Phase to address:** Security Hardening

---

### Pitfall 12: Missing Metrics and Observability

**What goes wrong:**
Production issues cannot be diagnosed. "It's slow" cannot be investigated.

**Prevention:**
```go
// Add Prometheus metrics
import "github.com/prometheus/client_golang/prometheus/promhttp"

router.GET("/metrics", gin.WrapH(promhttp.Handler()))

// Add request duration metrics
var httpDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
    Name:    "http_request_duration_seconds",
    Help:    "HTTP request duration",
    Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
}, []string{"method", "path", "status"})
```

**Detection:** No `/metrics` endpoint, no monitoring integration.

**Phase to address:** Production Hardening

---

### Pitfall 13: Incorrect JSON Handling

**What goes wrong:**
Using `json:` tags incorrectly, leading to unexpected behavior with omitempty, pointer vs value types, and time.Time formatting.

**Prevention:**
- Use `*time.Time` for optional timestamps
- Use `omitempty` carefully with required fields
- Use `json.RawMessage` for dynamic JSON

**Detection:** JSON serialization issues in tests, time.Time formatting inconsistencies.

**Phase to address:** Code Structure

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| Security Hardening | Secret defaults, CORS misconfig | Fail fast at startup if secrets not configured |
| Performance Optimization | Connection pool exhaustion, goroutine leaks | Set pool limits, use context everywhere |
| Graceful Shutdown | In-flight request termination | Implement signal handling with timeout |
| Health Checks | False positives/negatives | Check all dependencies, document health semantics |
| Request Tracing | Context not propagated | Add request ID middleware, use slog with context |
| Testing | Missing integration tests | Test against real dependencies (testcontainers) |
| Configuration | Missing validation at startup | Validate all required config before server starts |

---

## Common Anti-Patterns in Go HTTP Services

### 1. Global Variables for State
```go
// WRONG
var db *gorm.DB

func init() {
    db, _ = gorm.Open(...)
}

// CORRECT: Dependency injection
type Server struct {
    db *gorm.DB
    redis *redis.Client
}

func NewServer(db *gorm.DB, redis *redis.Client) *Server {
    return &Server{db: db, redis: redis}
}
```

### 2. Ignoring `defer rows.Close()`
```go
// WRONG
rows, _ := db.Raw("SELECT ...").Rows()
for rows.Next() {
    // ... connection leaked if error or early return
}

// CORRECT
rows, err := db.Raw("SELECT ...").Rows()
if err != nil {
    return err
}
defer rows.Close()
```

### 3. Using `context.Background()` in Request Path
```go
// WRONG
func (s *Service) Process(w http.ResponseWriter, r *http.Request) {
    data, _ := s.db.WithContext(context.Background()).Find(...)
}

// CORRECT
func (s *Service) Process(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    data, _ := s.db.WithContext(ctx).Find(...)
}
```

---

## Detection Checklist for This Project

Based on CONCERNS.md analysis:

- [x] JWT secret default detected → Pitfall 5
- [x] CORS `*` detected → Pitfall 8
- [x] No graceful shutdown → Pitfall 1
- [x] No health endpoints → Pitfall 9
- [x] Naive SQL injection check → Pitfall 10
- [ ] Connection pool config → Check for Pitfall 3
- [ ] Context propagation → Check for Pitfall 4
- [ ] Request timeouts → Check for Pitfall 6

---

## Sources

- Go Blog: "HTTP Server Graceful Shutdown in Go 1.8+" (HIGH confidence)
- Go Blog: "Context" package documentation (HIGH confidence)
- Database/sql documentation: Connection pool management (HIGH confidence)
- OWASP: Input Validation Cheat Sheet (HIGH confidence)
- Kubernetes documentation: Liveness and Readiness Probes (HIGH confidence)
- Project CONCERNS.md analysis (HIGH confidence - direct codebase inspection)

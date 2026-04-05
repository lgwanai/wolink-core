# Phase 4: Testing & Validation - Research

**Researched:** 2026-04-05
**Domain:** Go testing, benchmarking, integration testing, graceful shutdown verification
**Confidence:** HIGH

## Summary

This phase focuses on establishing comprehensive test coverage for the wolink-core AI Gateway project. The codebase already has a solid testing foundation with testify assertions, table-driven tests, and mock implementations. Current coverage varies from 0% (services, plugins) to 97.4% (observability). The project uses Go 1.25, Gin web framework, GORM, and go-redis with testify for assertions and sqlmock for database mocking.

**Primary recommendation:** Extend existing test patterns (table-driven tests, httptest recorder, mock interfaces) to services and handlers. Add performance benchmarks using Go's built-in `testing.B`. Use build tags to separate integration tests requiring external services from unit tests.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| TEST-01 | Unit tests for all services with minimum 80% coverage | Table-driven tests with testify; mock interfaces for DB/Redis; sqlmock for database tests |
| TEST-02 | Integration tests for all API endpoints | httptest.ResponseRecorder; Gin test mode; mock middleware for auth bypass |
| TEST-03 | Performance benchmarks for critical paths (chat completion, streaming) | Go `testing.B` benchmarks; measure allocations with `-benchmem` |
| TEST-04 | Graceful shutdown tests verify in-flight request completion | Existing tests in cmd/main_test.go provide pattern; extend with concurrent request scenarios |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/stretchr/testify | v1.11.1 | Assertions and mocking | Project standard; table-driven tests with assert/require |
| github.com/DATA-DOG/go-sqlmock | v1.5.2 | Database mocking | Already in use; GORM-compatible |
| net/http/httptest | stdlib | HTTP handler testing | Standard for Gin endpoint testing |
| testing | stdlib | Test framework and benchmarks | Go standard; supports `-race`, `-cover` |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/gin-gonic/gin | v1.12.0 | Web framework | Set `gin.SetMode(gin.TestMode)` in tests |
| gorm.io/gorm | v1.25.4 | ORM | Use sqlmock with GORM driver wrapper |
| github.com/go-redis/redis/v8 | v8.11.5 | Redis client | Use miniredis for in-memory Redis mocking |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| testify | gocheck, ginkgo | testify simpler, already adopted |
| sqlmock | dockertest | sqlmock faster for unit tests; dockertest better for integration |
| miniredis | real Redis in Docker | miniredis no external dependency; use Redis for integration tests |

**Installation:**
```bash
# Already installed - verify versions
go list -m github.com/stretchr/testify
go list -m github.com/DATA-DOG/go-sqlmock
```

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── services/
│   ├── auth_service.go
│   └── auth_service_test.go      # Unit tests alongside source
├── api/
│   ├── handlers/
│   │   ├── chat_handler.go
│   │   └── chat_handler_test.go  # Handler unit tests
│   └── routes_test.go            # Route integration tests
├── plugins/
│   ├── plugin_openai.go
│   └── plugin_openai_test.go     # Plugin tests with HTTP mocking
└── testutil/
    ├── mocks/                    # Shared mock implementations
    └── fixtures/                 # Test data
```

### Pattern 1: Table-Driven Tests
**What:** Single test function with multiple cases defined as struct slice
**When to use:** All unit tests with multiple input/output combinations
**Example:**
```go
// Source: existing internal/config/validator_test.go
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
                Security: SecurityConfig{JWTSecret: ""},
            },
            wantErr:     true,
            errContains: "JWT secret is required",
        },
        // ... more cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.config.Validate()
            if tt.wantErr {
                assert.Error(t, err)
                assert.Contains(t, err.Error(), tt.errContains)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### Pattern 2: Handler Testing with httptest
**What:** Use httptest.ResponseRecorder with Gin router for endpoint testing
**When to use:** HTTP handler unit and integration tests
**Example:**
```go
// Source: existing internal/api/handlers/health_handler_test.go
func TestHealth_Returns200OK(t *testing.T) {
    gin.SetMode(gin.TestMode)
    _, router := setupMockHealthHandler(nil, nil)

    req, _ := http.NewRequest(http.MethodGet, "/health", nil)
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)

    assert.Equal(t, http.StatusOK, w.Code)
    assert.JSONEq(t, `{"status": "ok"}`, w.Body.String())
}
```

### Pattern 3: Mock Interface Implementation
**What:** Create mock structs implementing service interfaces
**When to use:** Testing handlers and services in isolation
**Example:**
```go
// Source: existing internal/api/handlers/health_handler_test.go
type mockDBChecker struct {
    err error
}

func (m *mockDBChecker) PingContext(ctx context.Context) error {
    return m.err
}
```

### Pattern 4: Benchmark Functions
**What:** Functions named `BenchmarkXxx(*testing.B)` for performance testing
**When to use:** Critical paths: chat completion, streaming
**Example:**
```go
// Standard Go benchmark pattern
func BenchmarkChatCompletion(b *testing.B) {
    gin.SetMode(gin.TestMode)
    handler, router := setupBenchmarkHandler()
    body := bytes.NewBufferString(`{"model":"gpt-4","messages":[{"role":"user","content":"test"}]}`)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", body)
        w := httptest.NewRecorder()
        router.ServeHTTP(w, req)
    }
}
```

### Anti-Patterns to Avoid
- **Testing private functions directly:** Test through public API; private function bugs surface in public tests
- **Shared state between tests:** Use t.Parallel() only with isolated test data
- **Ignoring race conditions:** Always run `go test -race ./...`
- **Skipping error path tests:** Error paths are critical for production reliability
- **Mock overuse:** Prefer real implementations for simple utilities; mock external dependencies

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Mock HTTP server | Custom httptest.Server with handlers | httptest.Server with specific handlers | Standard library; handles TLS, cookies |
| Mock Redis client | Interface wrapper around go-redis | github.com/alicebob/miniredis | Full Redis protocol emulation |
| Mock time | Custom time provider interface | Go 1.25+ time mocking or interface | Clock skew tests need precision |
| Test fixtures | Manual JSON construction | encoding/json with structs | Type safety, IDE support |

**Key insight:** Go's testing ecosystem is mature. Prefer standard library and widely-adopted libraries over custom solutions.

## Common Pitfalls

### Pitfall 1: Gin Context Race Conditions
**What goes wrong:** Tests pass individually but fail with `-race` due to shared Gin context
**Why it happens:** Gin reuses context objects; tests modify context concurrently
**How to avoid:** Create fresh router and context for each test; use t.Parallel() carefully
**Warning signs:** Flaky tests, race detector warnings

### Pitfall 2: Database Mock Synchronization
**What goes wrong:** sqlmock expectations don't match actual queries
**Why it happens:** Query order or whitespace differences between mock and code
**How to avoid:** Use `sqlmock.AnyArg()` for dynamic values; match query patterns, not exact strings
**Warning signs:** "expected: X, actual: Y" errors from sqlmock

### Pitfall 3: Benchmark Measuring Wrong Thing
**What goes wrong:** Benchmark includes setup time, misleading results
**Why it happens:** Setup code before b.ResetTimer()
**How to avoid:** Put all setup before b.ResetTimer(); use b.RunParallel for concurrent benchmarks
**Warning signs:** Benchmark times vary wildly between runs

### Pitfall 4: Graceful Shutdown Test Timing
**What goes wrong:** Shutdown tests flaky due to timing assumptions
**Why it happens:** Hard-coded sleep values don't account for CI load
**How to avoid:** Use channels to signal completion; timeout assertions with generous margins
**Warning signs:** "context deadline exceeded" in tests

### Pitfall 5: Coverage False Positives
**What goes wrong:** 80% coverage but critical paths untested
**Why it happens:** Tests cover happy paths only; error handling skipped
**How to avoid:** Review coverage HTML output; ensure error branches covered
**Warning signs:** High coverage but production bugs in error paths

## Code Examples

### Unit Test with Mocks (Services)
```go
// Source: pattern from internal/api/handlers/health_handler_test.go
func TestAuthService_ValidateAPIKey_CacheHit(t *testing.T) {
    // Setup mock Redis
    mockRedis := miniredis.RunT(t)
    client := redis.NewClient(&redis.Options{
        Addr: mockRedis.Addr(),
    })

    // Pre-populate cache
    mockRedis.HSet("apikey:test-key", map[string]string{
        "id":       "1",
        "key_id":   "test-key",
        "status":   "active",
    })

    // Test cache hit path
    svc := NewAuthService(nil, client, logrus.New(), nil)
    apiKey, err := svc.ValidateAPIKey("test-key")

    assert.NoError(t, err)
    assert.Equal(t, "test-key", apiKey.KeyID)
}
```

### Integration Test (API Endpoints)
```go
// Source: pattern from internal/api/routes_test.go
func TestChatCompletion_Unauthorized(t *testing.T) {
    gin.SetMode(gin.TestMode)

    router := gin.New()
    router.POST("/v1/chat/completions", func(c *gin.Context) {
        // Handler expects api_key in context
        if _, exists := c.Get("api_key"); !exists {
            c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
            return
        }
    })

    body := `{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`
    req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()

    router.ServeHTTP(w, req)

    assert.Equal(t, http.StatusUnauthorized, w.Code)
}
```

### Performance Benchmark (Chat Completion)
```go
// Source: standard Go benchmark pattern
func BenchmarkChatCompletion_NonStream(b *testing.B) {
    gin.SetMode(gin.TestMode)

    // Setup handler with mock plugin
    handler := setupBenchmarkChatHandler()
    router := gin.New()
    router.POST("/v1/chat/completions", handler.ChatCompletions)

    body := `{"model":"gpt-4","messages":[{"role":"user","content":"benchmark test"}]}`
    req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer test-key")

    b.ResetTimer()
    b.ReportAllocs()

    for i := 0; i < b.N; i++ {
        w := httptest.NewRecorder()
        router.ServeHTTP(w, req)
    }
}
```

### Graceful Shutdown Test
```go
// Source: existing cmd/main_test.go pattern
func TestGracefulShutdown_WaitsForInFlightRequests(t *testing.T) {
    // Start server
    srv := &http.Server{
        Addr:    ":0",
        Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            time.Sleep(100 * time.Millisecond) // Simulate slow request
            w.WriteHeader(http.StatusOK)
        }),
    }

    go srv.ListenAndServe()
    time.Sleep(10 * time.Millisecond) // Let server start

    // Start request
    done := make(chan bool)
    go func() {
        resp, err := http.Get("http://" + srv.Addr)
        assert.NoError(t, err)
        assert.Equal(t, http.StatusOK, resp.StatusCode)
        done <- true
    }()

    // Initiate shutdown
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    err := srv.Shutdown(ctx)
    assert.NoError(t, err)

    // Verify request completed
    select {
    case <-done:
        // Success
    case <-time.After(2 * time.Second):
        t.Fatal("Request did not complete before shutdown")
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Testify assert only | testify/assert + require | Project start | require fails fast, cleaner test output |
| Real DB for tests | sqlmock + miniredis | Phase 2 | Faster, no external dependencies |
| Manual mock implementations | testify/mock | Consider for Phase 4 | Less boilerplate, automatic expectations |

**Deprecated/outdated:**
- `github.com/go-check/check`: Use testify instead
- `gopkg.in/check.v1`: Use testify instead

## Open Questions

1. **Integration test strategy for external APIs (OpenAI, Claude, DeepSeek)**
   - What we know: Plugins call external HTTP APIs; tests shouldn't make real calls
   - What's unclear: Should we use httptest.Server mocks or recorded responses (VCR-style)
   - Recommendation: Use httptest.Server with pre-recorded responses; faster and deterministic

2. **Coverage enforcement in CI**
   - What we know: 80% minimum required per requirements
   - What's unclear: Should this be enforced per-package or overall
   - Recommendation: Per-package enforcement prevents "coverage averaging" across packages

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.11.1 |
| Config file | None - use `//go:build` tags for integration tests |
| Quick run command | `go test -race ./...` |
| Full suite command | `go test -race -cover ./...` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| TEST-01 | Unit tests for services with 80% coverage | unit | `go test -race -cover ./internal/services/...` | Partial - placeholder tests |
| TEST-02 | Integration tests for API endpoints | integration | `go test -race ./internal/api/...` | Yes - routes_test.go, handlers tests |
| TEST-03 | Performance benchmarks for chat/streaming | benchmark | `go test -bench=. -benchmem ./internal/...` | No - Wave 0 |
| TEST-04 | Graceful shutdown verification | integration | `go test -race ./cmd/...` | Yes - main_test.go |

### Sampling Rate
- **Per task commit:** `go test -race ./internal/<modified-package>/...`
- **Per wave merge:** `go test -race -cover ./...`
- **Phase gate:** Full suite green with 80%+ coverage before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/services/auth_service_test.go` - unit tests for AuthService
- [ ] `internal/services/plugin_service_test.go` - unit tests for PluginService
- [ ] `internal/plugins/plugin_openai_test.go` - unit tests with HTTP mocking
- [ ] `internal/api/handlers/chat_handler_test.go` - handler tests for chat completion
- [ ] `internal/api/handlers/admin_handler_test.go` - handler tests for admin endpoints
- [ ] `internal/testutil/mocks/` - shared mock implementations
- [ ] `*_bench_test.go` files for performance benchmarks
- [ ] Integration test build tags: `//go:build integration`

*(If no gaps: "None - existing test infrastructure covers all phase requirements")*

## Sources

### Primary (HIGH confidence)
- Go testing documentation - https://pkg.go.dev/testing
- testify documentation - https://pkg.go.dev/github.com/stretchr/testify
- sqlmock documentation - https://pkg.go.dev/github.com/DATA-DOG/go-sqlmock
- Project existing tests: internal/api/handlers/*_test.go, internal/config/*_test.go

### Secondary (MEDIUM confidence)
- miniredis for Redis mocking - https://github.com/alicebob/miniredis
- Gin testing patterns - https://gin-gonic.com/docs/testing/

### Tertiary (LOW confidence)
- None - using primary and secondary sources only

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - testify, sqlmock already in use; patterns established
- Architecture: HIGH - existing tests demonstrate best practices
- Pitfalls: HIGH - based on standard Go testing anti-patterns

**Research date:** 2026-04-05
**Valid until:** 30 days - Go testing ecosystem stable

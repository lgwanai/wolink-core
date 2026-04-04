# Stack Research

**Domain:** Go Production Readiness & Performance Optimization
**Researched:** 2026-04-04
**Confidence:** HIGH

## Recommended Stack

### Core Technologies (Existing - Keep)

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go | 1.21+ | Runtime | Excellent for high-concurrency services, fast compilation, low memory footprint |
| Gin | 1.9+ | HTTP Framework | High performance, minimal overhead, widely adopted |
| GORM | 1.25+ | ORM | Mature, supports MySQL/PostgreSQL, good for rapid development |
| Redis | 6+ | Cache/Queue | In-memory speed, pub/sub, rate limiting, session storage |
| JWT (golang-jwt) | 5.3+ | Authentication | Standard token auth, well-maintained fork |

### Additions for Production Readiness

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| testify | 1.8+ | Testing Framework | Rich assertions, mocking, test suites - industry standard for Go |
| go-cmp | 0.6+ | Deep Comparisons | Better diff output for test failures |
| mockery | 2.42+ | Mock Generation | Auto-generate mocks from interfaces |
| prometheus/client_golang | 1.18+ | Metrics | Industry standard for observability |
| opentelemetry-go | 1.24+ | Tracing | Vendor-neutral distributed tracing |
| zerolog | 1.33+ | Structured Logging | Zero-allocation JSON logger, high performance |
| air | 1.52+ | Hot Reload | Development tool for fast iteration |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| go-playground/validator | 10.14+ | Request Validation | Already included with Gin, ensure proper usage |
| spf13/viper | 1.18+ | Configuration | Already in use, add validation |
| uber-go/zap | 1.27+ | High-performance Logging | Alternative to zerolog if structured logging needed |
| rs/cors | 1.10+ | CORS Handling | More configurable than custom middleware |
| uber-go/ratelimit | 0.3+ | Rate Limiting | Token bucket implementation |
| cenkalti/backoff | 4.3+ | Retry Logic | Exponential backoff for external calls |
| stretchr/testify | 1.8+ | Testing | Assertions, mocking, test suites |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| golangci-lint | Comprehensive Linting | Includes staticcheck, gosec, go vet, many others |
| gosec | Security Scanner | Detects common vulnerabilities |
| staticcheck | Static Analysis | Catches bugs before runtime |
| go test -race | Race Detection | Run in CI pipeline |
| go test -cover | Coverage Reports | Target 80%+ |
| benchstat | Benchmark Comparison | Compare performance improvements |

## Installation

```bash
# Testing dependencies
go get github.com/stretchr/testify@latest
go get github.com/google/go-cmp@latest
go install github.com/vektra/mockery/v2@latest

# Observability
go get github.com/prometheus/client_golang@latest
go get go.opentelemetry.io/otel@latest

# Logging
go get github.com/rs/zerolog@latest

# Development tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/air-verse/air@latest
```

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| testify | gocheck | Use gocheck for more xUnit-style testing |
| zerolog | zap | Use zap if already using Uber ecosystem |
| golangci-lint | megacheck | megacheck is deprecated; golangci-lint includes it |
| mockery | gomock | Use gomock if preferring Google's mock framework |
| Prometheus | OpenTelemetry metrics | Use OTel metrics for vendor-neutral approach |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| logrus (for new code) | Slower than zerolog/zap | zerolog or zap |
| standard log package | No structured logging | zerolog with JSON output |
| ioutil | Deprecated since Go 1.16 | io or os packages |
| context.Background() in tests | No timeout/cancel control | context.WithTimeout |
| global variables for config | Makes testing hard | Dependency injection |
| sql.DB directly | Manual connection management | GORM or sqlx with connection pooling |

## Stack Patterns by Variant

**If high-throughput is critical:**
- Use zerolog (zero-allocation logging)
- Enable connection pooling with explicit limits
- Consider pgxpool for PostgreSQL connection pooling

**If observability is critical:**
- Add Prometheus metrics endpoint
- Integrate OpenTelemetry tracing
- Use structured JSON logging

**If security is critical:**
- Enable all golangci-lint security checks
- Use gosec in CI pipeline
- Implement request rate limiting

## Version Compatibility

| Package A | Compatible With | Notes |
|-----------|-----------------|-------|
| Go 1.21 | Gin 1.9+ | Requires Go 1.20+ |
| GORM 1.25 | PostgreSQL 12+, MySQL 8+ | Both drivers supported |
| testify 1.8+ | Go 1.19+ | Full compatibility |
| zerolog 1.33+ | Go 1.17+ | No CGO dependencies |

## Key Recommendations

### Testing Strategy
1. Use `testify/suite` for test organization
2. Use `testify/mock` or `mockery` for interface mocking
3. Use table-driven tests for multiple scenarios
4. Run with `-race` flag always
5. Target 80%+ coverage minimum

### Logging Strategy
1. Switch from logrus to zerolog for performance
2. Use structured JSON logging in production
3. Include request IDs in all logs
4. Log at appropriate levels (debug for dev, info+ for prod)

### Metrics Strategy
1. Add `/metrics` endpoint for Prometheus
2. Track: request latency, error rates, active connections
3. Track: cache hit rates, database connection pool stats
4. Use histograms for latency, counters for errors

### Configuration Strategy
1. Add validation at startup
2. Fail fast on missing required config
3. Use environment variables for secrets
4. Support config file for non-sensitive values

## Sources

- Go 1.21 Release Notes — verified performance improvements
- Gin documentation — confirmed best practices
- testify documentation — standard testing patterns
- Prometheus Go client documentation — metrics integration
- OpenTelemetry Go documentation — tracing setup
- zerolog documentation — performance benchmarks

---
*Stack research for: Go Production Readiness*
*Researched: 2026-04-04*

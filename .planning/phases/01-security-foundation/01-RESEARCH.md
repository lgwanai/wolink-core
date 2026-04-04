# Phase 1: Security Foundation - Research

**Researched:** 2026-04-04
**Domain:** Go configuration validation, JWT security, CORS hardening, input validation
**Confidence:** HIGH (based on codebase analysis and established Go best practices)

## Summary

Phase 1 focuses on establishing fail-fast security patterns for the wolink-core AI Gateway. The codebase currently has several security gaps that could lead to production vulnerabilities: weak JWT secret defaults, permissive CORS configuration, and naive input validation. This research identifies the standard stack, patterns, and pitfalls for implementing SEC-01 through SEC-04.

**Primary recommendation:** Implement a unified validation layer at startup that checks all security-critical configuration, removing defaults for secrets and enforcing production hardening before the server accepts any traffic.

<phase_requirements>

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| SEC-01 | System validates configuration at startup and fails fast on missing/invalid values | Viper with custom validation; `MustGetSecret` pattern; startup validation before server starts |
| SEC-02 | JWT secret must be at least 32 characters with no production defaults | OWASP guidelines; Go `len()` check; fail-fast with clear error message |
| SEC-03 | CORS allowed origins loaded from configuration with fail-fast in production mode | Environment-aware CORS middleware; origin allowlist validation |
| SEC-04 | Input validation uses structured validation library with explicit rules | go-playground/validator v10 (already indirect dependency via Gin); binding tags |

</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| viper | v1.16.0 | Configuration management | Already in use; supports env vars, YAML, defaults |
| go-playground/validator | v10.14.0 | Struct validation | Already indirect dependency via Gin; tag-based validation |
| golang-jwt/jwt | v5.3.0 | JWT handling | Already in use; industry standard |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| testify | v1.8.4+ | Assertions and testing | Unit tests for validation logic |
| gosec | latest | Static security analysis | CI/CD pipeline for security scanning |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| go-playground/validator | govalidator | govalidator is less feature-rich; validator integrates with Gin binding |
| viper validation | cleanenv | cleanenv has built-in validation but would require config migration |
| custom CORS middleware | rs/cors | rs/cors is more feature-rich but adds dependency for simple allowlist check |

**Installation:**
```bash
# No new dependencies required - validator v10 is already an indirect dependency
# To make it direct for explicit validation use:
go get github.com/go-playground/validator/v10
```

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── config/
│   ├── config.go           # Existing - add Validate() method
│   └── validator.go        # NEW - custom validation logic
├── api/middleware/
│   ├── auth.go             # Existing - enhance CORS
│   └── cors.go             # NEW - environment-aware CORS middleware
└── services/
    └── security_service.go # Existing - enhance validation

cmd/
└── main.go                 # Add startup validation call
```

### Pattern 1: Startup Configuration Validation

**What:** Validate all configuration at startup before any server initialization. Fail immediately with clear error messages for missing/invalid config.

**When to use:** Always - this is a production readiness requirement.

**Example:**
```go
// Source: Established Go pattern for fail-fast security
// internal/config/validator.go

package config

import (
    "fmt"
    "os"
    "strings"
)

// Validate performs security-critical validation on configuration.
// Returns error if configuration is invalid for the current environment.
func (c *Config) Validate() error {
    // SEC-01: Fail fast on missing/invalid configuration
    if c.Server.Mode == "release" {
        // Production-specific validations
        if err := c.validateProduction(); err != nil {
            return err
        }
    }
    
    // SEC-02: JWT secret validation
    if err := c.validateJWTSecret(); err != nil {
        return err
    }
    
    // SEC-03: CORS validation
    if err := c.validateCORS(); err != nil {
        return err
    }
    
    return nil
}

func (c *Config) validateProduction() error {
    // In production, certain configurations are mandatory
    if c.Database.Password == "" {
        return fmt.Errorf("database password is required in production mode")
    }
    if c.Redis.Password == "" {
        return fmt.Errorf("redis password is required in production mode")
    }
    return nil
}

func (c *Config) validateJWTSecret() error {
    // SEC-02: Minimum 32 characters
    secret := c.Security.JWTSecret
    
    // Check for default/placeholder values
    defaultSecrets := []string{
        "your-secret-key",
        "your-super-secret-jwt-key",
        "secret",
        "jwt-secret",
        "changeme",
    }
    for _, def := range defaultSecrets {
        if strings.ToLower(secret) == def {
            return fmt.Errorf("JWT secret cannot be a default value: %q", secret)
        }
    }
    
    // Minimum length check
    if len(secret) < 32 {
        return fmt.Errorf("JWT secret must be at least 32 characters, got %d", len(secret))
    }
    
    return nil
}

func (c *Config) validateCORS() error {
    // SEC-03: In production mode, CORS must be explicitly configured
    if c.Server.Mode == "release" {
        // Check if CORS is using wildcard (not allowed in production)
        // This validation happens at middleware level
    }
    return nil
}

// MustValidate calls Validate and exits on error.
// Use in main() for fail-fast behavior.
func (c *Config) MustValidate() {
    if err := c.Validate(); err != nil {
        fmt.Fprintf(os.Stderr, "Configuration validation failed: %v\n", err)
        os.Exit(1)
    }
}
```

**Integration in main.go:**
```go
func main() {
    cfg, err := config.Load()
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }
    
    // SEC-01: Validate configuration before any initialization
    cfg.MustValidate()
    
    // ... rest of initialization
}
```

### Pattern 2: Environment-Aware CORS Middleware

**What:** CORS middleware that validates origins against a configurable allowlist and fails fast in production if not configured.

**When to use:** Any API that may be called from browsers.

**Example:**
```go
// Source: Go CORS best practices
// internal/api/middleware/cors.go

package middleware

import (
    "fmt"
    "net/http"
    "os"
    "strings"
    
    "github.com/gin-gonic/gin"
)

type CORSConfig struct {
    AllowedOrigins   []string
    AllowedMethods   []string
    AllowedHeaders   []string
    AllowCredentials bool
    IsProduction     bool
}

// NewCORS creates a CORS middleware with environment-aware validation.
func NewCORS(cfg CORSConfig) gin.HandlerFunc {
    // SEC-03: Fail fast in production if CORS not properly configured
    if cfg.IsProduction && len(cfg.AllowedOrigins) == 0 {
        fmt.Fprintln(os.Stderr, "ERROR: CORS allowed origins must be configured in production")
        os.Exit(1)
    }
    
    // Default methods and headers if not specified
    if len(cfg.AllowedMethods) == 0 {
        cfg.AllowedMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
    }
    if len(cfg.AllowedHeaders) == 0 {
        cfg.AllowedHeaders = []string{"Origin", "Content-Type", "Authorization"}
    }
    
    return func(c *gin.Context) {
        origin := c.GetHeader("Origin")
        
        // Check against allowlist
        allowed := false
        for _, allowedOrigin := range cfg.AllowedOrigins {
            if origin == allowedOrigin {
                allowed = true
                break
            }
        }
        
        // In development, be permissive
        if !cfg.IsProduction && origin == "" {
            allowed = true
        }
        
        if allowed && origin != "" {
            c.Header("Access-Control-Allow-Origin", origin)
            c.Header("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
            c.Header("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))
            if cfg.AllowCredentials {
                c.Header("Access-Control-Allow-Credentials", "true")
            }
        }
        
        if c.Request.Method == "OPTIONS" {
            c.AbortWithStatus(http.StatusNoContent)
            return
        }
        
        c.Next()
    }
}
```

**Configuration addition:**
```yaml
# config.yaml
cors:
  allowed_origins:
    - "https://your-domain.com"
    - "https://admin.your-domain.com"
  allow_credentials: true
```

### Pattern 3: Structured Input Validation

**What:** Use go-playground/validator with struct tags for declarative input validation, replacing naive string matching.

**When to use:** All API request validation.

**Example:**
```go
// Source: go-playground/validator documentation
// internal/api/handlers/requests.go

package handlers

// CreateAPIKeyRequest with validation tags
type CreateAPIKeyRequest struct {
    Name        string `json:"name" binding:"required,min=1,max=100"`
    Department  string `json:"department" binding:"required,oneof=eng sales marketing"`
    ExpiresDays int    `json:"expires_days" binding:"omitempty,min=1,max=365"`
}

// ChatCompletionRequest with validation
type ChatCompletionRequest struct {
    Model       string  `json:"model" binding:"required"`
    Messages    []Message `json:"messages" binding:"required,min=1,dive"`
    MaxTokens   int     `json:"max_tokens" binding:"omitempty,min=1,max=128000"`
    Temperature float64 `json:"temperature" binding:"omitempty,min=0,max=2"`
}

type Message struct {
    Role    string `json:"role" binding:"required,oneof=system user assistant"`
    Content string `json:"content" binding:"required,min=1"`
}
```

**Handler integration:**
```go
func (h *Handler) CreateAPIKey(c *gin.Context) {
    var req CreateAPIKeyRequest
    // SEC-04: Structured validation with binding tags
    if err := c.ShouldBindJSON(&req); err != nil {
        // Validator returns structured errors
        c.JSON(http.StatusBadRequest, gin.H{
            "error": formatValidationError(err),
        })
        return
    }
    // ... process validated request
}

func formatValidationError(err error) string {
    if validationErrors, ok := err.(validator.ValidationErrors); ok {
        var messages []string
        for _, fieldErr := range validationErrors {
            messages = append(messages, fmt.Sprintf(
                "field '%s' failed validation: %s",
                fieldErr.Field(), fieldErr.Tag(),
            ))
        }
        return strings.Join(messages, "; ")
    }
    return err.Error()
}
```

### Anti-Patterns to Avoid

- **Naive SQL injection detection:** The current `strings.Contains(strings.ToLower(content), "drop table")` is easily bypassed. GORM already uses parameterized queries, so this check is both insufficient and unnecessary. Replace with structured input validation.
- **Hardcoded secrets in defaults:** The current `viper.SetDefault("security.jwt_secret", "your-secret-key")` is a security vulnerability. Remove defaults for security-critical values.
- **Wild-card CORS:** `c.Header("Access-Control-Allow-Origin", "*")` in production allows any website to call the API. Must be configurable and environment-aware.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Input validation | Custom string matching, regex patterns | go-playground/validator | Handles edge cases, localization, struct tags |
| CORS middleware | Manual header setting | Configurable middleware with allowlist | Origin validation, credential handling, preflight |
| JWT secret validation | ad-hoc length checks | Config.Validate() method | Centralized, testable, consistent errors |
| Configuration validation | Scattered checks in main() | Validation method on Config struct | Single source of truth, easier testing |

**Key insight:** The codebase already has go-playground/validator as an indirect dependency. The cost to use it is minimal - just add struct tags and enable it for direct use.

## Common Pitfalls

### Pitfall 1: Default Secrets in Production

**What goes wrong:** JWT secret defaults to "your-secret-key" which is trivially guessable. If deployed with defaults, attackers can forge admin tokens.

**Why it happens:** Developer convenience during development. "I'll change it before production" mentality. Configuration files get committed with placeholder values.

**How to avoid:**
1. Remove all default values for `security.jwt_secret` from `setDefaults()`
2. Add startup validation that fails if secret is missing or too short
3. Use environment variable `AI_GATEWAY_SECURITY_JWT_SECRET` for production
4. Document secret generation: `openssl rand -base64 32`

**Warning signs:**
- `SetDefault("security.jwt_secret", ...)` in code
- Config file committed with `"your-secret-key"`
- No validation of secret length/complexity

### Pitfall 2: Permissive CORS in Production

**What goes wrong:** `Access-Control-Allow-Origin: *` allows any website to call the API from user browsers, enabling CSRF and data exfiltration attacks.

**Why it happens:** Quick fix during development. Developers misunderstand CORS as just a browser annoyance rather than a security feature.

**How to avoid:**
1. Load allowed origins from configuration
2. Fail fast in production mode if origins not configured
3. Validate each request's Origin against allowlist
4. Never use `*` in production

**Warning signs:**
- `Access-Control-Allow-Origin: *` hardcoded
- No `AllowedOrigins` in config struct
- No environment check in CORS middleware

### Pitfall 3: Naive Input Validation

**What goes wrong:** String matching like `strings.Contains(content, "drop table")` is trivially bypassed with SQL comments (`DROP/**/TABLE`), encoding, or alternative syntax.

**Why it happens:** Misunderstanding that ORM already handles SQL injection. Fear of unknown input. Attempting to "add security" without understanding the threat model.

**How to avoid:**
1. Trust GORM's parameterized queries for SQL injection protection
2. Use go-playground/validator for business rule validation
3. Define clear validation rules per field (min/max length, format, allowed values)
4. Remove naive string matching checks

**Warning signs:**
- `strings.Contains` for security checks
- Checking for "DROP", "DELETE", "INSERT" keywords
- No validation library in direct dependencies

### Pitfall 4: Validation After Initialization

**What goes wrong:** Configuration is validated only when used (lazy validation), causing runtime failures instead of startup failures.

**Why it happens:** Convenience of validating "just in time". Assumption that configuration is correct.

**How to avoid:**
1. Add `Validate()` method to Config struct
2. Call `cfg.MustValidate()` in main() before any initialization
3. Validate database connectivity, Redis connectivity during startup
4. Exit immediately with clear error message

**Warning signs:**
- No validation call in main()
- Configuration errors only discovered at runtime
- Server starts but fails on first request

## Code Examples

Verified patterns from codebase analysis:

### Current State (config.go:87-122)
```go
// Current defaults - SECURE defaults should NOT be set
func setDefaults() {
    viper.SetDefault("server.port", "8080")
    viper.SetDefault("server.mode", "debug")
    
    // PROBLEM: Default JWT secret is weak
    viper.SetDefault("security.jwt_secret", "your-secret-key")
    
    // Other defaults are fine for non-security config
    viper.SetDefault("database.host", "localhost")
    viper.SetDefault("redis.host", "localhost")
}
```

### Current State (middleware/auth.go:79-91)
```go
// Current CORS - too permissive
func CORS() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Header("Access-Control-Allow-Origin", "*") // PROBLEM
        c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
        c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
        
        if c.Request.Method == "OPTIONS" {
            c.AbortWithStatus(204)
            return
        }
        c.Next()
    }
}
```

### Current State (services/security_service.go:64-72)
```go
// Current validation - naive and ineffective
func (s *SecurityService) ValidateRequest(content string) error {
    // This is easily bypassed and GORM already handles SQL injection
    if strings.Contains(strings.ToLower(content), "drop table") {
        return fmt.Errorf("potentially malicious content detected")
    }
    return nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Default secrets for convenience | No defaults, fail-fast | Industry standard since 2015+ | Prevents production misconfigurations |
| Wildcard CORS | Configurable allowlist | OWASP guidelines | Prevents CSRF/data exfiltration |
| String matching for injection | ORM parameterized queries + struct validation | GORM 1.x+ | Proper security model |
| Lazy validation | Eager validation at startup | Go best practices | Fail-fast, easier debugging |

**Deprecated/outdated:**
- `viper.SetDefault()` for secrets: Security anti-pattern, should fail if not configured
- String matching for SQL injection detection: GORM handles this; remove or replace with business validation
- Wildcard CORS in any environment: Even development should use explicit origins

## Open Questions

1. **Should CORS configuration be required in development mode?**
   - What we know: Currently uses `*` which is convenient for development
   - What's unclear: Whether to enforce explicit origins even in debug mode
   - Recommendation: Allow `*` only when `server.mode` is "debug" AND no `cors.allowed_origins` configured. Document this clearly.

2. **What is the maximum reasonable JWT secret length?**
   - What we know: Minimum is 32 characters for security
   - What's unclear: Should there be an upper limit
   - Recommendation: No upper limit needed; longer is better. Document that 64+ characters is recommended for high-security environments.

3. **Should the existing `ValidateRequest` in security_service.go be removed or repurposed?**
   - What we know: Current implementation is ineffective for SQL injection
   - What's unclear: Whether there's value in content validation beyond SQL injection
   - Recommendation: Repurpose for business-rule content validation (e.g., detect and mask sensitive data, validate prompt structure). Remove SQL injection check since GORM handles this.

## Validation Architecture

> nyquist_validation is enabled in .planning/config.json

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go standard testing + testify (to be added) |
| Config file | None - use TestMain for setup |
| Quick run command | `go test -v ./internal/config/...` |
| Full suite command | `go test -race -cover ./...` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SEC-01 | Fail fast on missing config | unit | `go test -v ./internal/config/... -run TestConfig_Validate` | Wave 0 |
| SEC-01 | Fail fast on invalid config | unit | `go test -v ./internal/config/... -run TestConfig_Validate` | Wave 0 |
| SEC-02 | Reject JWT secret < 32 chars | unit | `go test -v ./internal/config/... -run TestConfig_validateJWTSecret` | Wave 0 |
| SEC-02 | Reject default JWT secrets | unit | `go test -v ./internal/config/... -run TestConfig_validateJWTSecret` | Wave 0 |
| SEC-03 | CORS rejects non-allowlist origins | unit | `go test -v ./internal/api/middleware/... -run TestCORS` | Wave 0 |
| SEC-03 | Fail if no CORS config in production | unit | `go test -v ./internal/api/middleware/... -run TestNewCORS` | Wave 0 |
| SEC-04 | Struct validation works | unit | `go test -v ./internal/api/handlers/... -run TestValidation` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test -v ./...`
- **Per wave merge:** `go test -race -cover ./...`
- **Phase gate:** Full suite green with 80%+ coverage for new code

### Wave 0 Gaps
- [ ] `internal/config/validator_test.go` - covers SEC-01, SEC-02
- [ ] `internal/api/middleware/cors_test.go` - covers SEC-03
- [ ] `internal/api/handlers/validation_test.go` - covers SEC-04
- [ ] Framework install: `go get github.com/stretchr/testify` - needed for assertions
- [ ] `go.mod` update: Make `github.com/go-playground/validator/v10` a direct dependency

*(No existing test infrastructure - all test files must be created in Wave 0)*

## Sources

### Primary (HIGH confidence)
- Codebase analysis: `internal/config/config.go`, `internal/api/middleware/auth.go`, `internal/services/security_service.go` - direct inspection of current implementation
- go-playground/validator v10.14.0 - already indirect dependency via Gin
- Go standard library: `os`, `fmt`, `strings` - for validation patterns

### Secondary (MEDIUM confidence)
- OWASP guidelines for JWT secret length (32+ characters minimum)
- Go best practices for fail-fast configuration validation
- CORS security best practices from MDN Web Docs

### Tertiary (LOW confidence)
- None - all recommendations based on codebase analysis and established practices

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - already using viper, Gin, GORM; validator is indirect dependency
- Architecture: HIGH - straightforward validation layer addition
- Pitfalls: HIGH - concrete examples found in codebase

**Research date:** 2026-04-04
**Valid until:** 30 days - stable Go patterns with no breaking changes expected

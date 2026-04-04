# Testing Patterns

**Analysis Date:** 2026-04-04

## Test Framework

**Runner:**
- Standard Go testing package (`testing`)
- No test files currently exist in the codebase

**Run Commands:**
```bash
go test ./...                    # Run all tests
go test -race ./...              # Run with race detection
go test -cover ./...             # Run with coverage
go test -v ./...                 # Verbose output
```

## Current State

**Test files:** None found

**Test coverage:** 0% - No tests implemented

## Recommended Test Structure

**Location:**
- Co-located with source files: `internal/services/auth_service_test.go`

**Naming:**
- Test files: `*_test.go`
- Test functions: `func TestXxx(t *testing.T)`

**Example structure:**
```
internal/
├── services/
│   ├── auth_service.go
│   ├── auth_service_test.go      # Unit tests for AuthService
│   ├── queue_service.go
│   └── queue_service_test.go     # Unit tests for QueueService
├── api/
│   └── handlers/
│       ├── chat_handler.go
│       └── chat_handler_test.go  # Unit tests for ChatHandler
```

## Table-Driven Tests Pattern

**Standard Go pattern for tests:**
```go
func TestValidateAPIKey(t *testing.T) {
    tests := []struct {
        name    string
        keyID   string
        want    *models.APIKey
        wantErr bool
    }{
        {
            name:    "valid key",
            keyID:   "ak-validkey123",
            want:    &models.APIKey{KeyID: "ak-validkey123", Status: "active"},
            wantErr: false,
        },
        {
            name:    "invalid key",
            keyID:   "ak-invalid",
            want:    nil,
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := authService.ValidateAPIKey(tt.keyID)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidateAPIKey() error = %v, wantErr %v", err, tt.wantErr)
            }
            // Additional assertions
        })
    }
}
```

## Mocking

**Recommended approach:** Use interfaces for mocking

**Interface-based mocking:**
```go
// Define interface for external dependencies
type Database interface {
    Find(dest interface{}, conds ...interface{}) *gorm.DB
    Create(value interface{}) *gorm.DB
}

// Use testify/mock or go/mock for mock implementations
```

**Mocking Redis:**
```go
// Use miniredis for Redis mocking in tests
import "github.com/alicebob/miniredis/v2"

func setupTestRedis(t *testing.T) *miniredis.Miniredis {
    s := miniredis.RunT(t)
    return s
}
```

**Mocking HTTP clients:**
```go
// Use httptest for HTTP mocking
import "net/http/httptest"

func TestCallModel(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(models.ChatCompletionResponse{})
    }))
    defer server.Close()
    
    // Use server.URL as BaseURL
}
```

## Test Helpers

**Recommended setup function pattern:**
```go
func setupTestService(t *testing.T) (*AuthService, func()) {
    // Setup test database
    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    if err != nil {
        t.Fatalf("Failed to connect to test database: %v", err)
    }
    
    // Run migrations
    db.AutoMigrate(&models.APIKey{}, &models.Department{})
    
    // Create service
    logger := logrus.New()
    logger.SetOutput(io.Discard) // Suppress logs in tests
    cfg := &config.Config{}
    
    service := NewAuthService(db, nil, logger, cfg)
    
    // Return cleanup function
    cleanup := func() {
        sqlDB, _ := db.DB()
        sqlDB.Close()
    }
    
    return service, cleanup
}
```

## Fixtures and Test Data

**Recommended approach:**
```go
// Define test fixtures in test files
func createTestAPIKey(t *testing.T, db *gorm.DB) *models.APIKey {
    apiKey := &models.APIKey{
        DepartmentID: 1,
        KeyID:        "ak-test123",
        KeySecret:    "secret123",
        Name:         "Test Key",
        Status:       "active",
        DailyLimit:   10000,
        MonthlyLimit: 300000,
    }
    if err := db.Create(apiKey).Error; err != nil {
        t.Fatalf("Failed to create test API key: %v", err)
    }
    return apiKey
}
```

## Coverage

**Target:** 80% coverage recommended

**View coverage:**
```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out    # View in browser
go tool cover -func=coverage.out    # Text output
```

## Test Types

**Unit Tests:**
- Test individual functions and methods
- Mock external dependencies (database, Redis, HTTP)
- Fast execution, no network calls

**Integration Tests:**
- Test service interactions
- Use test containers or local services
- Build with build tags: `//go:build integration`

**Example integration test:**
```go
//go:build integration

func TestAuthService_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    // Integration test code
}
```

## HTTP Handler Testing

**Gin handler testing pattern:**
```go
func TestChatCompletions(t *testing.T) {
    // Setup
    router := gin.New()
    handler := NewChatHandler(serviceManager, logger)
    router.POST("/v1/chat/completions", handler.ChatCompletions)
    
    // Create request
    body := `{"model": "gpt-4", "messages": [{"role": "user", "content": "hello"}]}`
    req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
    req.Header.Set("Authorization", "Bearer ak-test")
    req.Header.Set("Content-Type", "application/json")
    
    // Record response
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)
    
    // Assert
    if w.Code != http.StatusOK {
        t.Errorf("Expected status 200, got %d", w.Code)
    }
}
```

## Concurrency Testing

**Race detection:**
```bash
go test -race ./...
```

**Goroutine leak detection:**
```go
import "go.uber.org/goleak"

func TestMain(m *testing.M) {
    goleak.VerifyTestMain(m)
}
```

## Benchmarks

**Benchmark pattern:**
```go
func BenchmarkValidateAPIKey(b *testing.B) {
    // Setup
    authService := setupAuthService()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        authService.ValidateAPIKey("ak-test123")
    }
}
```

**Run benchmarks:**
```bash
go test -bench=. ./...
go test -bench=. -benchmem ./...  # Include memory allocation
```

## Test Configuration

**Environment variables for tests:**
```go
func TestMain(m *testing.M) {
    // Set test environment
    os.Setenv("AI_GATEWAY_DATABASE_TYPE", "sqlite")
    os.Setenv("AI_GATEWAY_DATABASE_DBNAME", ":memory:")
    
    os.Exit(m.Run())
}
```

## Recommended Test Dependencies

Add to `go.mod`:
```
github.com/stretchr/testify v1.8.4    // Assertions and mocking
github.com/alicebob/miniredis/v2 v2.30.0  // Redis mocking
go.uber.org/goleak v1.2.1              // Goroutine leak detection
```

---

*Testing analysis: 2026-04-04*

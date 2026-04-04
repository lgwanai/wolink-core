# Coding Conventions

**Analysis Date:** 2026-04-04

## Language

**Primary:** Go 1.21+

## Naming Patterns

**Packages:**
- Lowercase single-word names: `api`, `config`, `models`, `services`, `utils`, `plugins`
- Package names match directory names

**Files:**
- Snake_case: `auth_service.go`, `chat_handler.go`, `plugin_openai.go`
- Test files: `*_test.go` (no test files currently exist)
- Interface files: `interface.go` for defining package interfaces

**Types (structs):**
- PascalCase: `AuthService`, `ChatHandler`, `ModelPlugin`, `APIKey`
- Constructor pattern: `NewAuthService()`, `NewChatHandler()`

**Functions/Methods:**
- PascalCase for exported: `ValidateAPIKey`, `ChatCompletions`, `CheckRateLimit`
- camelCase for unexported: `cacheAPIKey`, `buildRequest`, `generateKeyID`
- Receiver names: single lowercase letter matching type initial (e.g., `s *AuthService`, `h *ChatHandler`, `p *OpenAIPlugin`)

**Variables:**
- camelCase for local: `apiKey`, `modelConfig`, `requestID`
- Short names acceptable in limited scope: `db`, `rdb`, `cfg`, `sm`

**Constants:**
- No explicit constants defined; use string literals for status values

## Code Style

**Formatting:**
- Use `gofmt` and `goimports` (standard Go formatting)
- No explicit formatting configuration files

**Linting:**
- No linting configuration files detected (`.golangci.yml`, etc.)
- Standard Go conventions apply

**Imports Organization:**
- Standard library first
- Third-party packages second
- Local packages third
- Example from `internal/api/routes.go`:
```go
import (
    // Standard library: none in this example

    // Local packages
    "wolink-core/internal/api/handlers"
    "wolink-core/internal/api/middleware"
    "wolink-core/internal/config"
    "wolink-core/internal/services"

    // Third-party
    "github.com/gin-gonic/gin"
    "github.com/sirupsen/logrus"
)
```

## Error Handling

**Pattern:**
```go
if err != nil {
    return fmt.Errorf("context message: %w", err)
}
```

**Error wrapping:**
- Always use `fmt.Errorf` with `%w` verb to wrap errors
- Include context about what operation failed
- Examples from `internal/services/auth_service.go`:
```go
return nil, fmt.Errorf("database error: %w", err)
return nil, fmt.Errorf("failed to create API key: %w", err)
```

**Error responses in handlers:**
```go
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch API keys"})
```

**Fatal errors on startup:**
```go
if err != nil {
    log.Fatalf("Failed to load config: %v", err)
}
```

## Logging

**Framework:** logrus

**Logger creation:**
```go
logger := utils.NewLogger(cfg.Log.Level)
```

**Log levels:** debug, info, warn, error

**Logging patterns:**
```go
logger.Infof("Server starting on port %s", cfg.Server.Port)
logger.Errorf("Failed to save conversation: %v", err)
logger.Debugf("OpenAI API request: %s", string(reqBody))
logger.Warnf("OpenAI API key is empty")
```

**Structured logging with fields:**
```go
logger.WithFields(logrus.Fields{
    "status_code": param.StatusCode,
    "latency":     param.Latency,
}).Info("API Request")
```

**JSON format:** Logger configured with JSON formatter

## Comments

**Chinese comments used throughout:**
```go
// 加载配置
// 初始化日志
// 初始化数据库
// 验证 API Key
// 检查速率限制
```

**Function documentation:**
- Public functions have brief Chinese comments
- Example: `// ValidateAPIKey 验证API密钥并返回相关信息`

**No JSDoc/TSDoc equivalent:** Go uses standard doc comments

## Function Design

**Size:** Functions typically 10-50 lines, some larger handlers up to 100 lines

**Parameters:**
- Struct pointers for complex data: `*models.ChatCompletionRequest`
- Context as first parameter for operations: `ctx context.Context`
- Dependencies injected via constructor

**Return values:**
- Result pointer and error: `(*models.APIKey, error)`
- Single error for operations: `error`

**Constructor pattern:**
```go
func NewAuthService(db *gorm.DB, redis *redis.Client, logger *logrus.Logger, cfg *config.Config) *AuthService {
    return &AuthService{
        db:     db,
        redis:  redis,
        logger: logger,
        config: cfg,
    }
}
```

## Module Design

**Exports:**
- Public types and functions use PascalCase
- Private helpers use camelCase

**No barrel files:** Go uses package-level exports naturally

**Interface definition pattern:**
- Define interfaces in separate `interface.go` file
```go
// internal/plugins/interface.go
type ModelPlugin interface {
    Name() string
    Protocol() string
    Call(ctx context.Context, config *models.ModelConfig, request *models.ChatCompletionRequest) (*models.ChatCompletionResponse, error)
    CallStream(ctx context.Context, config *models.ModelConfig, request *models.ChatCompletionRequest) (io.ReadCloser, error)
    HealthCheck(config *models.ModelConfig) bool
}
```

## Dependency Injection

**Service Manager pattern:**
```go
type ServiceManager struct {
    DB     *gorm.DB
    Redis  *redis.Client
    Logger *logrus.Logger
    Config *config.Config
    
    AuthService         *AuthService
    AdminAuthService    *AdminAuthService
    ModelService        *ModelService
    // ... other services
}
```

**Constructor injection:**
```go
func NewServiceManager(db *gorm.DB, rdb *redis.Client, logger *logrus.Logger, cfg *config.Config) *ServiceManager
```

## Handler Design

**Pattern:**
```go
type ChatHandler struct {
    serviceManager *services.ServiceManager
    logger         *logrus.Logger
}

func NewChatHandler(sm *services.ServiceManager, logger *logrus.Logger) *ChatHandler {
    return &ChatHandler{
        serviceManager: sm,
        logger:         logger,
    }
}

func (h *ChatHandler) ChatCompletions(c *gin.Context) {
    // Handler implementation
}
```

## Middleware Pattern

**Gin middleware:**
```go
func APIKeyAuth(authService *services.AuthService) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Middleware logic
        c.Next()
    }
}
```

## Configuration

**Viper for configuration:**
```go
viper.SetConfigName("config")
viper.SetConfigType("yaml")
viper.AddConfigPath("./configs")
viper.SetEnvPrefix("AI_GATEWAY")
viper.AutomaticEnv()
```

**Environment variables:** Prefix `AI_GATEWAY_` for env overrides

---

*Convention analysis: 2026-04-04*

# Architecture

**Analysis Date:** 2026-04-04

## Pattern Overview

**Overall:** Layered Architecture with Plugin-based Model Integration

**Key Characteristics:**
- Clean separation between API, service, and data layers
- Hot-pluggable model provider plugins
- Async processing via Redis queues for non-critical operations
- Dual authentication (API Key for clients, JWT for admin)
- Zero-copy streaming for real-time responses

## Layers

**API Layer (Handlers):**
- Purpose: HTTP request handling, input validation, response formatting
- Location: `internal/api/handlers/`
- Contains: Request handlers for chat, admin, and plugin operations
- Depends on: Services layer, middleware
- Used by: Gin router (`internal/api/routes.go`)

**Middleware Layer:**
- Purpose: Cross-cutting concerns (auth, logging, CORS)
- Location: `internal/api/middleware/`
- Contains: APIKeyAuth, AdminAuth, Logger, CORS handlers
- Depends on: Services layer (AuthService, AdminAuthService)
- Used by: API routes

**Service Layer:**
- Purpose: Core business logic orchestration
- Location: `internal/services/`
- Contains: AuthService, ModelService, PluginService, QueueService, SecurityService, etc.
- Depends on: Models, external plugins, database, Redis
- Used by: Handlers layer

**Plugin Layer:**
- Purpose: Model provider abstraction and integration
- Location: `internal/plugins/`
- Contains: ModelPlugin interface, provider implementations (OpenAI, DeepSeek, Claude)
- Depends on: HTTP clients, model configuration
- Used by: PluginService

**Data Layer:**
- Purpose: Data persistence and caching
- Location: `internal/models/`, `internal/utils/database.go`
- Contains: GORM models, database initialization, Redis setup
- Depends on: PostgreSQL/MySQL, Redis
- Used by: Services layer

**Configuration Layer:**
- Purpose: Application configuration management
- Location: `internal/config/`
- Contains: Configuration structs and Viper-based loading
- Depends on: YAML files, environment variables
- Used by: All layers

## Data Flow

**Chat Completion Request Flow:**

1. Client sends POST to `/v1/chat/completions` with API Key
2. `APIKeyAuth` middleware validates key via `AuthService`
3. `ChatHandler.ChatCompletions` parses request
4. `ModelConfigService` retrieves available models for API key
5. `SecurityService` detects and replaces sensitive information
6. `PluginService.CallModel` or `CallModelStream` invokes provider plugin
7. Plugin (OpenAI/DeepSeek/Claude) makes HTTP request to provider
8. Response streamed back to client (SSE for streaming)
9. Async: `QueueService` enqueues conversation and usage logs
10. Background workers persist to database

**Admin Authentication Flow:**

1. Admin sends POST to `/admin/auth/login`
2. `AdminAuthService.Login` validates credentials
3. JWT token generated and stored in `AdminSession`
4. Subsequent requests use `AdminAuth` middleware
5. Token validated against database session

**Async Processing Flow:**

1. Handler creates `ConversationTask` or `UsageLogTask`
2. `QueueService.EnqueueConversation` pushes to Redis list
3. Background goroutine pops from queue with `BRPop`
4. Worker processes task and persists to database
5. On failure, task re-queued with retry count (max 3 retries)

## Key Abstractions

**ModelPlugin Interface:**
- Purpose: Abstracts different LLM provider implementations
- Examples: `internal/plugins/plugin_openai.go`, `internal/plugins/plugin_deepseek.go`, `internal/plugins/plugin_claude.go`
- Pattern: Strategy pattern - each plugin implements Call, CallStream, HealthCheck

**ServiceManager:**
- Purpose: Central dependency container for all services
- Examples: `internal/services/service_manager.go`
- Pattern: Service Locator - holds references to all service instances

**StreamWrapper:**
- Purpose: Wraps HTTP response body for SSE streaming
- Examples: `internal/plugins/stream_wrapper.go`
- Pattern: Adapter pattern - adapts io.ReadCloser for line-by-line SSE reading

## Entry Points

**Main Application:**
- Location: `cmd/main.go`
- Triggers: Process start
- Responsibilities: Config loading, DB/Redis init, service manager creation, HTTP server startup, graceful shutdown

**API Routes:**
- Location: `internal/api/routes.go`
- Triggers: HTTP requests
- Responsibilities: Route registration, middleware chaining, handler instantiation

## Error Handling

**Strategy:** Layered error handling with context wrapping

**Patterns:**
- Services return errors with `fmt.Errorf("operation failed: %w", err)` wrapping
- Handlers convert service errors to appropriate HTTP status codes
- Queue tasks have retry logic (3 retries with exponential backoff)
- Logging at each layer for debugging

## Cross-Cutting Concerns

**Logging:** `logrus.Logger` instance passed through all layers, configured via `internal/utils/logger.go`

**Validation:** Request validation via Gin's `ShouldBindJSON` with struct tags

**Authentication:**
- API Key: Bearer token in Authorization header, validated against database/Redis
- Admin: JWT token with session management in database

**Rate Limiting:** Redis-based counters for daily, monthly, and concurrent request limits

**Caching:** Redis used for API key validation, model configurations, and rate limit counters

---

*Architecture analysis: 2026-04-04*

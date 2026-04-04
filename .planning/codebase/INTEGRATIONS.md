# External Integrations

**Analysis Date:** 2026-04-04

## APIs & External Services

**AI/LLM Providers:**
- **OpenAI** - GPT-4 and compatible models
  - SDK/Client: Custom plugin `internal/plugins/plugin_openai.go`
  - Auth: API key via `conn_config.api_key` in model YAML
  - Endpoint: `https://api.openai.com` (configurable)
  - Protocol: OpenAI-compatible REST API

- **Anthropic Claude** - Claude-3 Sonnet
  - SDK/Client: Custom plugin `internal/plugins/plugin_claude.go`
  - Auth: API key via `x-api-key` header
  - Endpoint: `https://api.anthropic.com`
  - Protocol: Anthropic Messages API (converted to OpenAI format internally)

- **DeepSeek** - DeepSeek-V3 model
  - SDK/Client: Custom plugin `internal/plugins/plugin_deepseek.go`
  - Auth: API key via `Authorization: Bearer` header
  - Endpoint: `https://api.deepseek.com`
  - Protocol: OpenAI-compatible REST API

## Data Storage

**Databases:**
- MySQL 8.0+ (primary recommendation)
  - Connection: Configured via `database.*` in `configs/config.yaml`
  - Client: GORM with `gorm.io/driver/mysql`
  - Tables: departments, api_keys, model_registries, api_key_model_mappings, conversations, usage_logs, admin_users, admin_sessions

- PostgreSQL 12+ (alternative)
  - Connection: Configured via `database.*` with `type: postgres`
  - Client: GORM with `gorm.io/driver/postgres`
  - Same table structure as MySQL

**File Storage:**
- Local filesystem only for configuration files
- Model icons stored via `icon_uri` or `icon_url` references

**Caching:**
- Redis 6+
  - Purpose: API key caching, rate limiting counters, message queues
  - Client: `github.com/go-redis/redis/v8`
  - Queue names: `ai_gateway:queue:conversations`, `ai_gateway:queue:usage_logs`

## Authentication & Identity

**Auth Provider:**
- Custom JWT-based authentication
  - Implementation: `internal/services/admin_auth_service.go`
  - Library: `github.com/golang-jwt/jwt/v5`
  - JWT secret: Configured via `security.jwt_secret` in config
  - Token storage: Redis for session management
  - Middleware: `internal/api/middleware/admin_auth.go`

**API Key Authentication:**
- Custom API key system for API consumers
  - Implementation: `internal/services/auth_service.go`
  - Key format: `ak-{key_id}:{key_secret}`
  - Key validation: Redis cache + database fallback
  - Rate limiting: Redis-based counters

## Monitoring & Observability

**Error Tracking:**
- None configured (standard Go error handling with logrus)

**Logs:**
- logrus structured logging
  - Configured in `internal/utils/logger.go`
  - Log level configurable via `log.level` in config
  - HTTP request logging via middleware

**Metrics:**
- Prometheus endpoint planned (see `configs/prometheus.yml`)
  - Endpoint: `/metrics` on application port
  - Scraped every 5 seconds

## CI/CD & Deployment

**Hosting:**
- Docker containers
- No specific cloud platform required (cloud-agnostic)

**CI Pipeline:**
- None configured (no CI config files detected)

**Deployment Scripts:**
- `scripts/deploy.sh` - Docker Compose deployment
- `scripts/start.sh` - Local development startup
- `scripts/init_db.sh` - Database initialization

## Environment Configuration

**Required env vars:**
```
AI_GATEWAY_DATABASE_TYPE       # mysql or postgres
AI_GATEWAY_DATABASE_HOST       # Database host
AI_GATEWAY_DATABASE_PORT       # Database port
AI_GATEWAY_DATABASE_USER       # Database user
AI_GATEWAY_DATABASE_PASSWORD   # Database password
AI_GATEWAY_DATABASE_DBNAME     # Database name
AI_GATEWAY_REDIS_HOST          # Redis host
AI_GATEWAY_REDIS_PORT          # Redis port
AI_GATEWAY_REDIS_PASSWORD      # Redis password
```

**Secrets location:**
- Configuration file: `configs/config.yaml` (contains passwords and JWT secret)
- Model configuration files: `configs/models/*.yaml` (contain API keys for LLM providers)
- Environment variables: Can override all config values with `AI_GATEWAY_` prefix

**Security Note:**
- API keys for LLM providers are stored in model configuration YAML files
- Database passwords and JWT secrets stored in main config file
- No secret management service integration detected

## Webhooks & Callbacks

**Incoming:**
- None detected

**Outgoing:**
- None detected (all LLM calls are synchronous HTTP requests)

## Plugin System

**Architecture:**
- Hot-pluggable plugin interface defined in `internal/plugins/interface.go`
- Plugin interface: `ModelPlugin` with methods: `Name()`, `Protocol()`, `Call()`, `CallStream()`, `HealthCheck()`
- Plugin manager: `internal/services/plugin_service.go`

**Supported Protocols:**
- `openai` - OpenAI-compatible API (plugin_openai.go)
- `claude` - Anthropic Claude API (plugin_claude.go)
- `deepseek` - DeepSeek API (plugin_deepseek.go)

**Adding New Providers:**
1. Create new plugin implementing `ModelPlugin` interface
2. Register plugin in `PluginService`
3. Add model configuration YAML in `configs/models/`

---

*Integration audit: 2026-04-04*

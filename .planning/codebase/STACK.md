# Technology Stack

**Analysis Date:** 2026-04-04

## Languages

**Primary:**
- Go 1.21 - Backend service implementation, all core logic in `internal/` and `cmd/`

**Secondary:**
- YAML - Configuration files in `configs/` for application and model settings
- SQL - Database migrations in `migrations/`
- Shell - Deployment and utility scripts in `scripts/`

## Runtime

**Environment:**
- Go 1.21+ (specified in `go.mod` line 3)
- Docker with alpine:latest for production (see `Dockerfile`)
- Docker Compose for local development and deployment

**Package Manager:**
- Go modules
- Lockfile: `go.sum` present (57594 bytes)

## Frameworks

**Core:**
- Gin v1.9.1 - HTTP web framework for API endpoints (`github.com/gin-gonic/gin`)
- GORM v1.25.4 - ORM for database operations (`gorm.io/gorm`)

**Database Drivers:**
- gorm.io/driver/mysql v1.5.2 - MySQL driver for GORM
- gorm.io/driver/postgres v1.5.2 - PostgreSQL driver for GORM

**Testing:**
- Standard Go testing package (no external test framework detected)
- No test files found in current codebase

**Build/Dev:**
- Docker multi-stage builds for production deployment
- Shell scripts for deployment automation (`scripts/deploy.sh`, `scripts/start.sh`)

## Key Dependencies

**Critical:**
- github.com/go-redis/redis/v8 v8.11.5 - Redis client for caching and message queues
- github.com/golang-jwt/jwt/v5 v5.3.0 - JWT authentication for admin users
- github.com/spf13/viper v1.16.0 - Configuration management with environment variable support
- github.com/google/uuid v1.3.0 - UUID generation for conversation IDs
- golang.org/x/crypto v0.9.0 - Password hashing (bcrypt)

**Infrastructure:**
- github.com/sirupsen/logrus v1.9.3 - Structured logging
- gopkg.in/yaml.v3 v3.0.1 - YAML parsing for configuration files

**Performance:**
- github.com/bytedance/sonic v1.9.1 - High-performance JSON encoder/decoder (Gin dependency)

## Configuration

**Environment:**
- Viper with environment variable support (prefix: `AI_GATEWAY_`)
- Configuration file: `configs/config.yaml`
- Model configurations: `configs/models/*.yaml`

**Build:**
- Multi-stage Dockerfile with builder pattern
- CGO_ENABLED=0 for static binary
- Configuration embedded via COPY in Docker image

**Key Configuration Files:**
- `configs/config.yaml` - Main application configuration
- `configs/models/gpt-4.yaml` - OpenAI GPT-4 model configuration
- `configs/models/claude-3.yaml` - Anthropic Claude model configuration
- `configs/models/deepseek-v3.yaml` - DeepSeek model configuration
- `configs/prometheus.yml` - Prometheus monitoring configuration

## Platform Requirements

**Development:**
- Go 1.21+
- MySQL 8.0+ OR PostgreSQL 12+
- Redis 6+

**Production:**
- Docker container runtime
- MySQL or PostgreSQL database server
- Redis server for caching and queues
- Prometheus + Grafana for monitoring (optional)

**Deployment Options:**
- Docker Compose with MySQL (`docker-compose.yml`)
- Docker Compose with PostgreSQL (`docker-compose.postgres.yml`)
- Standalone binary with configuration files

---

*Stack analysis: 2026-04-04*

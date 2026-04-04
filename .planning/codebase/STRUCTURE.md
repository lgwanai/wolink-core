# Codebase Structure

**Analysis Date:** 2026-04-04

## Directory Layout

```
wolink-core/
├── cmd/                          # Application entry point
│   └── main.go                   # Main program
├── internal/                     # Private application code
│   ├── api/                      # HTTP API layer
│   │   ├── handlers/             # Request handlers
│   │   ├── middleware/           # HTTP middleware
│   │   └── routes.go             # Route configuration
│   ├── config/                   # Configuration management
│   ├── models/                   # Data models and DTOs
│   ├── plugins/                  # Model provider plugins
│   ├── services/                 # Business logic services
│   └── utils/                    # Utilities (DB, Redis, Logger)
├── configs/                      # Configuration files
│   ├── config.yaml               # Main configuration
│   ├── config.mysql.yaml         # MySQL-specific config
│   ├── config.postgres.yaml      # PostgreSQL-specific config
│   ├── prometheus.yml            # Prometheus config
│   ├── models/                   # Model configuration files
│   └── templates/                # Configuration templates
├── migrations/                   # Database migrations
├── scripts/                      # Utility scripts
│   ├── deploy.sh                 # Deployment script
│   ├── init_db.sh                # Database initialization
│   ├── start.sh                  # Application start script
│   └── performance_test.sh       # Performance testing
├── .gitignore                    # Git ignore rules
├── Dockerfile                    # Docker build configuration
├── docker-compose.yml            # Docker Compose (main)
├── docker-compose.postgres.yml   # Docker Compose (PostgreSQL)
├── go.mod                        # Go module definition
├── go.sum                        # Go dependencies checksum
└── README.md                     # Project documentation
```

## Directory Purposes

**cmd/:**
- Purpose: Application entry points
- Contains: Main executable
- Key files: `main.go`

**internal/api/:**
- Purpose: HTTP layer with routing, handlers, and middleware
- Contains: Route definitions, request handlers, auth middleware
- Key files: `routes.go`, `handlers/chat_handler.go`, `middleware/auth.go`

**internal/config/:**
- Purpose: Configuration loading and management
- Contains: Config structs, Viper initialization
- Key files: `config.go`

**internal/models/:**
- Purpose: Data models for database and API DTOs
- Contains: GORM models, request/response structures
- Key files: `models.go`

**internal/plugins/:**
- Purpose: Model provider implementations
- Contains: Plugin interface, provider-specific implementations
- Key files: `interface.go`, `plugin_openai.go`, `plugin_deepseek.go`, `plugin_claude.go`

**internal/services/:**
- Purpose: Core business logic
- Contains: Service implementations for auth, models, conversations, queue
- Key files: `service_manager.go`, `auth_service.go`, `plugin_service.go`, `queue_service.go`

**internal/utils/:**
- Purpose: Shared utilities and initialization
- Contains: Database, Redis, Logger setup
- Key files: `database.go`, `logger.go`

**configs/:**
- Purpose: Runtime configuration files
- Contains: YAML configs for application and models
- Key files: `config.yaml`, `models/*.yaml`

**scripts/:**
- Purpose: Operational scripts
- Contains: Deployment, initialization, testing scripts
- Key files: `deploy.sh`, `start.sh`

## Key File Locations

**Entry Points:**
- `cmd/main.go`: Application bootstrap and server startup
- `internal/api/routes.go`: HTTP route registration

**Configuration:**
- `configs/config.yaml`: Main application configuration
- `configs/models/*.yaml`: Per-model provider configurations

**Core Logic:**
- `internal/services/service_manager.go`: Service dependency container
- `internal/services/plugin_service.go`: Plugin orchestration
- `internal/services/auth_service.go`: API key validation
- `internal/services/model_config_service.go`: Model configuration loading

**HTTP Handlers:**
- `internal/api/handlers/chat_handler.go`: Chat completion endpoint
- `internal/api/handlers/admin_handler.go`: Admin management endpoints
- `internal/api/handlers/admin_auth_handler.go`: Admin authentication

**Middleware:**
- `internal/api/middleware/auth.go`: API key authentication
- `internal/api/middleware/admin_auth.go`: JWT admin authentication

**Data Models:**
- `internal/models/models.go`: All database models and DTOs

**Plugin Implementations:**
- `internal/plugins/interface.go`: ModelPlugin interface definition
- `internal/plugins/plugin_openai.go`: OpenAI-compatible API plugin
- `internal/plugins/plugin_deepseek.go`: DeepSeek API plugin
- `internal/plugins/plugin_claude.go`: Claude API plugin

**Testing:**
- No test files present in codebase

## Naming Conventions

**Files:**
- Go files: `snake_case.go` (e.g., `auth_service.go`, `chat_handler.go`)
- Config files: `kebab-case.yaml` (e.g., `deepseek-v3.yaml`, `config.mysql.yaml`)
- Plugin files: `plugin_<provider>.go` (e.g., `plugin_openai.go`)

**Directories:**
- Lowercase, single words preferred (e.g., `api`, `services`, `models`)
- Nested packages use underscore or no separator (e.g., `handlers`, `middleware`)

**Go Identifiers:**
- Exported types/functions: PascalCase (e.g., `AuthService`, `NewServiceManager`)
- Unexported: camelCase (e.g., `cacheAPIKey`, `buildRequest`)
- Interfaces: noun or verb+noun (e.g., `ModelPlugin`, `io.ReadCloser`)

## Where to Add New Code

**New Feature (API endpoint):**
- Handler: `internal/api/handlers/<feature>_handler.go`
- Service: `internal/services/<feature>_service.go`
- Routes: Add to `internal/api/routes.go`
- Models: Add to `internal/models/models.go` (or create new file if large)

**New Model Provider:**
- Plugin implementation: `internal/plugins/plugin_<provider>.go`
- Register in: `internal/services/plugin_service.go` `registerBuiltinPlugins()`
- Config file: `configs/models/<model-name>.yaml`

**New Middleware:**
- Implementation: `internal/api/middleware/<middleware>.go`
- Registration: Add to `internal/api/routes.go` router setup

**New Configuration:**
- Struct: Add to `internal/config/config.go`
- Defaults: Add to `setDefaults()` in `internal/config/config.go`
- YAML: Add to `configs/config.yaml`

**New Database Model:**
- Model definition: `internal/models/models.go`
- Migration: Add to `internal/utils/database.go` `AutoMigrate()` call
- Service methods: Create or extend service in `internal/services/`

**New Background Worker:**
- Queue definition: `internal/services/queue_service.go`
- Task struct: Add to `queue_service.go`
- Processing logic: Add worker method following existing patterns

## Special Directories

**configs/models/:**
- Purpose: YAML configuration files for each LLM model/provider
- Generated: No
- Committed: Yes
- Format: Contains model metadata, connection config, capabilities

**migrations/:**
- Purpose: Database migration files
- Generated: No
- Committed: Yes
- Note: Currently minimal usage (using GORM AutoMigrate)

**scripts/:**
- Purpose: Operational and deployment scripts
- Generated: No
- Committed: Yes
- Executable: Yes (shell scripts have execute permission)

---

*Structure analysis: 2026-04-04*

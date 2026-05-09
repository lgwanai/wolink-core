---
phase: 06-admin
plan: 01
subsystem: infrastructure
tags: [stateless, gorm-removal, gateway, model-cleanup]

# Dependency graph
requires: []
provides:
  - "Removed all GORM models from internal/models/ (Conversation, UsageLog, ModelRegistry, APIKeyModelMapping)"
  - "Stripped GORM tags from APIKey and Node structs"
  - "Deleted ConversationService, UsageService, QueueService"
  - "Deleted admin_handler.go with all CRUD endpoints"
  - "Removed DB dependency from HealthHandler, ServiceManager, main.go"
  - "Model config loading made DB-safe (transitional for Plan 02)"
affects: [06-02, 06-03, plan-02-auth-refactor]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Stateless health checks — Ready() only checks Redis if configured, skips DB"
    - "Nil-db transitional pattern — services accept nil db parameter, skip DB ops when nil"
    - "Config-file-only model loading — GetModelsByAPIKey loads from directory when db is nil"

key-files:
  created: []
  modified:
    - "internal/models/models.go — Removed 4 GORM models, stripped APIKey tags"
    - "internal/models/node.go — Stripped GORM tags, removed TableName()"
    - "internal/services/node_service.go — Removed DB/Redis fields, 4 methods deleted"
    - "internal/services/service_manager.go — Removed DB field, 3 service fields, updated constructor"
    - "internal/services/model_config_service.go — DB-safe registerModelConfig and GetModelsByAPIKey"
    - "internal/api/handlers/health_handler.go — Removed DB dependency"
    - "internal/api/routes.go — Removed admin CRUD routes"
    - "internal/api/handlers/chat_handler.go — recordConversationAsync made no-op"
    - "cmd/main.go — DB init removed, updated ServiceManager call"
    - "internal/utils/database.go — Removed deleted models from AutoMigrate"
  deleted:
    - "internal/services/conversation_service.go"
    - "internal/services/usage_service.go"
    - "internal/services/queue_service.go"
    - "internal/api/handlers/admin_handler.go"
    - "internal/api/handlers/admin_handler_test.go"

key-decisions:
  - "Kept APIKey.ID field (without GORM tag) for AuthService/rate-limiting compatibility — Plan 02 will refactor to use KeyID strings"
  - "ModelConfigService uses nil-db transitional pattern: DB ops skipped when db=nil, loads models from config files directly"
  - "chat_handler.go recordConversationAsync made no-op — conversation recording belongs to admin service"

patterns-established:
  - "Nil-db transition: services pass nil db, conditionally skip DB operations at runtime"
  - "Config-first model loading: GetModelsByAPIKey reads all models from directory when DB unavailable"

requirements-completed:
  - GW-01
  - GW-02
  - GW-03

# Metrics
duration: 19min
completed: 2026-05-09
---

# Phase 06 Plan 01: Stateless Gateway — Database Removal Summary

**Stripped all GORM models, deleted 3 DB-dependent services, removed admin CRUD handler, and made gateway compile without database connection**

## Performance

- **Duration:** 19 min
- **Started:** 2026-05-09T15:15:35Z
- **Completed:** 2026-05-09T15:34:25Z
- **Tasks:** 3
- **Files modified:** 18 total (5 deleted, 13 modified)

## Accomplishments

- Removed 4 GORM model definitions (ModelRegistry, APIKeyModelMapping, Conversation, UsageLog) and stripped GORM tags from APIKey and Node
- Deleted 3 DB-dependent services: ConversationService, UsageService, QueueService
- Deleted admin_handler.go with all CRUD endpoints (API keys, models, usage, conversations)
- Removed DB dependency from HealthHandler, ServiceManager, main.go, and routes.go
- Build passes cleanly (`go build ./...` and `go vet ./internal/...`)
- ModelConfigService transitioned to load models from config files when DB is unavailable

## Task Commits

Each task was committed atomically:

1. **Task 1: Remove database-dependent GORM models** - `2a4e072` (feat)
2. **Task 2: Remove database-dependent services and refactor NodeService** - `4790dc7` (feat)
3. **Task 3: Remove admin_handler, update health handler, ServiceManager, routes, and main.go** - `aa63569` (feat)

## Files Created/Modified

- `internal/models/models.go` - Removed Conversation, UsageLog, ModelRegistry, APIKeyModelMapping; stripped GORM from APIKey
- `internal/models/node.go` - Stripped GORM tags from Node, removed TableName()
- `internal/services/node_service.go` - Removed db/redis fields, 4 DB-write methods deleted
- `internal/services/service_manager.go` - Removed DB field, 3 service fields, updated constructor (nil db)
- `internal/services/model_config_service.go` - DB-safe transitional refactor for config-file loading
- `internal/api/handlers/health_handler.go` - Removed gorm.DB dependency, stateless readiness checks
- `internal/api/routes.go` - Removed all admin CRUD routes
- `internal/api/handlers/chat_handler.go` - recordConversationAsync made no-op
- `cmd/main.go` - DB init removed, ServiceManager call updated
- `internal/utils/database.go` - Removed deleted models from AutoMigrate
- **Deleted:** conversation_service.go, usage_service.go, queue_service.go, admin_handler.go, admin_handler_test.go

## Decisions Made

1. **Kept APIKey.ID field (without GORM tag)** — Removing it would break AuthService's rate-limiting Redis keys (`concurrent:%d`, `daily:%d:%s`) and ModelConfigService's cache keys. Plan 02 will refactor to use KeyID strings.
2. **ModelConfigService nil-db transitional pattern** — When db is nil, `registerModelConfig` skips DB registration and `GetModelsByAPIKey` loads all models from config directory. Plan 02 will implement proper config-driven model resolution.
3. **No-op conversation recording** — `recordConversationAsync` in chat_handler.go was made a no-op instead of fully removed, keeping the call sites intact. Plan 02 can properly refactor.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] chat_handler.go referenced deleted QueueService types**
- **Found during:** Task 3 (ServiceManager/chat_handler integration)
- **Issue:** Deleting queue_service.go removed QueueService, ConversationTask, UsageLogTask types, but chat_handler.go still referenced them at lines 477, 494, and in recordConversationDirect
- **Fix:** Made recordConversationAsync a no-op (logs debug message), removed recordConversationDirect entirely
- **Files modified:** internal/api/handlers/chat_handler.go
- **Committed in:** `aa63569` (Task 3 commit)

**2. [Rule 3 - Blocking] model_config_service.go used deleted GORM model types**
- **Found during:** Task 3 (build failure)
- **Issue:** registerModelConfig used models.ModelRegistry and GetModelsByAPIKey used models.APIKeyModelMapping — both deleted in Task 1
- **Fix:** Refactored registerModelConfig to skip DB ops when db is nil. Refactored GetModelsByAPIKey to load models from config directory when DB unavailable
- **Files modified:** internal/services/model_config_service.go
- **Committed in:** `aa63569` (Task 3 commit)

**3. [Rule 3 - Blocking] database.go AutoMigrate referenced deleted model types**
- **Found during:** Task 3 (build failure)
- **Issue:** AutoMigrate call referenced models.ModelRegistry, models.APIKeyModelMapping, models.Conversation, models.UsageLog
- **Fix:** Removed deleted models from AutoMigrate, kept only APIKey
- **Files modified:** internal/utils/database.go
- **Committed in:** `aa63569` (Task 3 commit)

**4. [Rule 3 - Blocking] Test files referenced deleted models and changed APIs**
- **Found during:** Task 3 (vet failures)
- **Issue:** chat_handler_test.go, chat_handler_bench_test.go, health_handler_test.go, node_service_test.go, tts_test.go all referenced deleted models (ModelRegistry, APIKeyModelMapping) or outdated constructor signatures
- **Fix:** Updated all test files: removed DB-dependent setup, fixed constructor calls (NewNodeService, NewHealthHandler), rewrote tts_test.go without ModelRegistry
- **Files modified:** 5 test files across handlers/ and services/
- **Committed in:** `aa63569` (Task 3 commit)

---

**Total deviations:** 4 auto-fixed (all Rule 3 - Blocking)
**Impact on plan:** All auto-fixes were necessary to restore build and vet. The plan assumed removing models/services/handlers wouldn't cascade to other files, but the tight coupling required cross-file fixes. No scope creep.

## Issues Encountered

- Go binary not in default PATH — resolved by using full path `/opt/homebrew/bin/go`
- Plan's instruction to remove `apiKey.ID` contradicted requirement for `go build` to succeed — opted to keep the field (without GORM tag) as transitional measure

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Gateway compiles without database connection (`go build ./...` passes)
- Remaining GORM references in AuthService, ModelService, ModelConfigService — planned for refactor in Plan 02
- Gateway is ready for Plan 02: Config-driven auth (APIKeyValidator + AdminSyncService)

---
*Phase: 06-admin*
*Completed: 2026-05-09*

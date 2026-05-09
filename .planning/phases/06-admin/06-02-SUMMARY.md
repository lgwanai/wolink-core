---
phase: 06-admin
plan: 02
subsystem: infrastructure
tags: [stateless, auth, api-key-validator, admin-sync, config-driven]

# Dependency graph
requires:
  - phase: 06-01
    provides: "Removed GORM models and DB-dependent services"
provides:
  - "GatewayConfig struct with mode, API key whitelist, and admin master settings"
  - "APIKeyValidator — validates keys from config whitelist (single-node) or synced cache (multi-node)"
  - "AdminSyncService — periodic sync of API keys and model configs from admin master API"
  - "AuthService refactored to use APIKeyValidator instead of gorm.DB"
  - "ModelConfigService refactored to load from filesystem only (no DB)"
  - "ModelService reduced to no-op placeholder"
  - "ServiceManager wires APIKeyValidator and AdminSyncService for multi-node mode"
affects: [06-03, plan-03-wiring]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Config-driven auth: API key validation from config file or synced cache, no database"
    - "Thread-safe cache with sync.RWMutex for concurrent API key access"
    - "Periodic sync pattern for multi-node admin API communication"

key-files:
  created:
    - "internal/services/api_key_validator.go — Stateless API key validation service"
    - "internal/services/admin_sync_service.go — Periodic config sync from admin master"
  modified:
    - "internal/config/config.go — Added GatewayConfig, APIKeyEntry, AdminMasterConfig structs"
    - "internal/services/auth_service.go — Removed gorm.DB, uses APIKeyValidator"
    - "internal/services/model_config_service.go — Removed gorm.DB, filesystem-only loading"
    - "internal/services/model_service.go — Simplified to no-op placeholder"
    - "internal/services/service_manager.go — Wires APIKeyValidator, AdminSyncService"
    - "internal/services/auth_service_test.go — Rewritten for config-driven validation"
    - "internal/services/auth_service_bench_test.go — Updated for APIKeyValidator"
    - "internal/api/handlers/chat_handler_test.go — Updated constructor calls"
    - "internal/api/handlers/chat_handler_bench_test.go — Updated constructor calls"

key-decisions:
  - "APIKeyValidator owns all API key logic (validate, cache, list) — AuthService delegates to it via ValidateAPIKey"
  - "AdminSyncService uses X-Admin-Token header for admin API auth, matching existing gateway pattern"
  - "Redis caching removed from ModelConfigService.GetModelsByAPIKey — filesystem read is fast enough for infrequent model config changes"

patterns-established:
  - "Config-driven validation: ValidateAPIKey checks mode (single/multi) and routes to whitelist or synced cache"
  - "Thread-safe cache: sync.RWMutex for concurrent reads with atomic cache replacement"
  - "Periodic sync: AdminSyncService with configurable interval, immediate initial sync"

requirements-completed:
  - GW-04
  - GW-05
  - GW-06

# Metrics
duration: 6min
completed: 2026-05-09
---

# Phase 06 Plan 02: Config-Driven Auth Summary

**Config-driven API key validation via APIKeyValidator (config whitelist for single-node, admin API sync for multi-node) — AuthService, ModelConfigService, and ServiceManager fully refactored for stateless no-DB operation**

## Performance

- **Duration:** 6 min
- **Started:** 2026-05-09T15:43:16Z
- **Completed:** 2026-05-09T15:49:41Z
- **Tasks:** 3
- **Files modified:** 11 total (2 created, 9 modified)

## Accomplishments

- Added `GatewayConfig` with mode, APIKeyEntry whitelist, AdminMasterConfig, and SyncInterval fields to config.go
- Created `APIKeyValidator` — validates keys from config whitelist (single-node) or synced in-memory cache (multi-node), thread-safe with RWMutex
- Created `AdminSyncService` — periodic HTTP pull from admin master API (`/admin/api-keys`, `/admin/models`) with configurable interval, X-Admin-Token auth
- Refactored `AuthService` — removed all gorm.DB dependencies, delegates ValidateAPIKey to APIKeyValidator, removed GenerateAPIKey/cacheAPIKey/updateDatabaseUsage/private helpers
- Refactored `ModelConfigService` — removed gorm.DB and redis caching, loads models from filesystem only, removed registerModelConfig and findConfigFile
- Simplified `ModelService` to no-op placeholder
- Updated `ServiceManager` to create APIKeyValidator, wire it to AuthService, and start AdminSyncService in multi-node mode
- Fixed all test files to match new constructor signatures and config-driven behavior
- Full `go build ./internal/...` and `go vet ./internal/...` pass cleanly
- Zero gorm.DB references in production service code

## Task Commits

Each task was committed atomically:

1. **Task 1: Add GatewayConfig with mode, API key whitelist, and admin master settings** - `8b561ad` (feat)
2. **Task 2: Create APIKeyValidator and AdminSyncService** - `c5a0bbc` (feat)
3. **Task 3: Refactor AuthService, ModelConfigService, ModelService, and ServiceManager for no-DB operation** - `54b133c` (feat)

## Files Created/Modified

- `internal/config/config.go` — Added GatewayConfig, APIKeyEntry, AdminMasterConfig structs; Gateway field on Config; defaults (single mode, empty whitelist, 30s sync)
- `internal/services/api_key_validator.go` (new) — APIKeyValidator with whitelist (single) and cache (multi) validation, thread-safe
- `internal/services/admin_sync_service.go` (new) — AdminSyncService with StartSync/StopSync, periodic HTTP sync
- `internal/services/auth_service.go` — Removed gorm.DB, redis caching; ValidateAPIKey delegates to APIKeyValidator; removed DB-dependent methods
- `internal/services/model_config_service.go` — Removed gorm.DB, redis caching; LoadModelConfigs validates filesystem files only; removed registerModelConfig/findConfigFile
- `internal/services/model_service.go` — Simplified to no-op placeholder struct
- `internal/services/service_manager.go` — Creates APIKeyValidator, wires services, starts AdminSyncService in multi-node mode, stops on shutdown
- `internal/services/auth_service_test.go` — Rewritten for config-driven validation (valid, invalid, empty whitelist, multi-node cache tests)
- `internal/services/auth_service_bench_test.go` — Updated to use APIKeyValidator with config pre-populated
- `internal/api/handlers/chat_handler_test.go` — Updated constructor calls for NewAuthService, NewModelConfigService
- `internal/api/handlers/chat_handler_bench_test.go` — Updated constructor calls with APIKeyValidator

## Decisions Made

1. **APIKeyValidator as single source of truth** — AuthService delegates `ValidateAPIKey` entirely to APIKeyValidator. This keeps auth logic centralized and allows both single-node and multi-node validation without AuthService knowing the mode.
2. **AdminSyncService uses X-Admin-Token** — Matches the existing gateway admin auth pattern established in Phase 5, ensuring consistency across admin API communication.
3. **No Redis caching for model configs** — `GetModelsByAPIKey` previously cached results in Redis with 5-minute TTL. Removed because filesystem reads are fast enough for infrequent model config changes, simplifying the code.
4. **ModelService as no-op** — The service was previously a placeholder anyway. Plan 02 explicitly reduces it to a no-op since ModelConfigService handles all model configuration.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- Test files (`auth_service_test.go`, `chat_handler_test.go`, `chat_handler_bench_test.go`, `auth_service_bench_test.go`) had old constructor signatures and DB-dependent test logic — all fixed as part of Task 3. This was expected since the plan noted test files would need updating.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Gateway fully stateless — API key validation and model config loading work without database
- Ready for Plan 06-03: Wire stateless gateway, finalize entry point, update config
- `go build ./internal/...` and `go vet ./internal/...` pass cleanly
- All production service code has zero gorm.DB references

---

## Self-Check: PASSED

*Phase: 06-admin*
*Completed: 2026-05-09*

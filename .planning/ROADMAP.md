# Roadmap: wolink-core Production Readiness

## Overview

Transform wolink-core AI Gateway from early development to production-ready status through a focused 4-phase approach: security hardening, infrastructure hardening, observability implementation, and comprehensive testing. Each phase builds on the previous, establishing a stable foundation before adding capabilities.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Security Foundation** - Configuration validation, JWT enforcement, CORS hardening, input validation
- [x] **Phase 2: Infrastructure Hardening** - Graceful shutdown, health endpoints, connection pooling, timeouts
- [x] **Phase 3: Observability** - Request tracing, structured logging, Prometheus metrics, error standardization
- [x] **Phase 4: Testing & Validation** - Unit tests, integration tests, benchmarks, shutdown tests
- [x] **Phase 5: Admin Auth Refactor** - Remove admin auth, department management; add token-based admin API

## Phase Details

### Phase 1: Security Foundation
**Goal**: Application fails fast on invalid configuration and enforces security requirements at startup
**Depends on**: Nothing (first phase)
**Requirements**: SEC-01, SEC-02, SEC-03, SEC-04
**Success Criteria** (what must be TRUE):
  1. Application exits immediately with clear error when required configuration is missing or invalid
  2. JWT secret shorter than 32 characters causes startup failure in production mode
  3. CORS rejects requests from origins not in the configured allowlist
  4. All user inputs pass through structured validation before processing
**Plans**: 5 plans in 4 waves (Wave 0 + Waves 1-3)

Plans:
- [x] 01-00: Wave 0 - Test infrastructure setup (testify + test file scaffolds)
- [x] 01-01: Configuration validation and fail-fast (SEC-01, SEC-02) - Wave 1
- [x] 01-02: CORS configuration hardening (SEC-03) - Wave 1
- [x] 01-03: Input validation infrastructure (SEC-04) - Wave 2
- [x] 01-04: Authentication request validation (SEC-04) - Wave 3

### Phase 2: Infrastructure Hardening
**Goal**: Server handles production traffic with proper resource management and health visibility
**Depends on**: Phase 1
**Requirements**: INFRA-01, INFRA-02, INFRA-03, INFRA-04, INFRA-05, INFRA-06
**Success Criteria** (what must be TRUE):
  1. Server completes in-flight requests before shutting down when receiving SIGTERM
  2. `/health` endpoint returns 200 OK when server is running
  3. `/ready` endpoint returns 200 OK only when database and Redis are connected, 503 otherwise
  4. Database and Redis connections are pooled with configurable limits
  5. HTTP server enforces read/write/idle timeouts to prevent resource exhaustion
**Plans**: 5 plans in 2 waves

Plans:
- [x] 02-01: Infrastructure configuration structs (INFRA-01, INFRA-04, INFRA-05, INFRA-06) - Wave 1
- [x] 02-02: Health and readiness endpoints (INFRA-02, INFRA-03) - Wave 1
- [x] 02-03: Database connection pooling (INFRA-04) - Wave 2
- [x] 02-04: Redis connection pooling (INFRA-05) - Wave 2
- [x] 02-05: HTTP server timeouts and shutdown (INFRA-01, INFRA-06) - Wave 2

### Phase 3: Observability
**Goal**: All requests are traceable and system behavior is measurable through logs and metrics
**Depends on**: Phase 2
**Requirements**: OBS-01, OBS-02, OBS-03, OBS-04
**Success Criteria** (what must be TRUE):
  1. Every request has a unique ID visible in logs and response headers
  2. Log entries are structured JSON containing request ID, timestamp, and contextual fields
  3. `/metrics` endpoint exposes request rate, latency, and error counters in Prometheus format
  4. All API errors return consistent HTTP status codes matching error type
**Plans**: 6 plans in 3 waves (Wave 0 + Waves 1-2)

Plans:
- [x] 03-00: Wave 0 - Install dependencies and test scaffolds - Wave 0
- [x] 03-01: Request ID middleware (OBS-01) - Wave 1
- [x] 03-02: Structured JSON logging (OBS-02) - Wave 1
- [x] 03-03: Prometheus metrics endpoint (OBS-03) - Wave 1
- [x] 03-04: Error type standardization (OBS-04) - Wave 1
- [x] 03-05: Integration and middleware chain - Wave 2

### Phase 4: Testing & Validation
**Goal**: All critical paths have automated tests providing confidence for production deployment
**Depends on**: Phase 3
**Requirements**: TEST-01, TEST-02, TEST-03, TEST-04
**Success Criteria** (what must be TRUE):
  1. All services have unit tests with minimum 80% code coverage
  2. All API endpoints have integration tests verifying request/response behavior
  3. Critical paths (chat completion, streaming) have performance benchmarks
  4. Graceful shutdown is verified through automated tests
**Plans**: 4 plans in 2 waves

Plans:
- [x] 04-01: Unit tests for services (TEST-01) - Wave 1
- [x] 04-02: Integration tests for handlers and plugins (TEST-02) - Wave 1
- [x] 04-03: Performance benchmarks (TEST-03) - Wave 2
- [x] 04-04: Graceful shutdown tests (TEST-04) - Wave 2

## Progress

**Execution Order:**
Phases execute in numeric order: 1 -> 2 -> 3 -> 4 -> 5 -> 6 -> 7 -> 8 -> 9

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Security Foundation | 5/5 | Complete | 2026-04-05 |
| 2. Infrastructure Hardening | 5/5 | Complete | 2026-04-05 |
| 3. Observability | 6/6 | Complete | 2026-04-05 |
| 4. Testing & Validation | 4/4 | Complete | 2026-04-05 |
| 5. Admin Auth Refactor | 4/4 | Complete | 2026-04-25 |
| 6. Stateless Gateway | 3/3 | Complete | 2026-05-09 |
| 7. OCR Support | 3/4 | In Progress | — |
| 8. Passthrough/Parsed Mode | 0/0 | Not Started | — |
| 9. TUI Gateway Manager | 0/0 | Not Started | — |

### Phase 5: wolink-core 网关核心化重构

**Goal:** Transform wolink-core into a pure gateway by removing admin authentication, department management, and adding admin management API with token authentication for external admin system communication.
**Depends on:** Phase 4
**Requirements:** ADMIN-01, ADMIN-02, ADMIN-03, ADMIN-04, ADMIN-05
**Plans:** 4 plans in 4 waves

Plans:
- [x] 05-01: Configuration and Models - Add AdminConfig, remove Department/AdminUser/AdminSession models
- [x] 05-02: Services Layer - Create NodeService, delete AdminAuthService, update AuthService
- [x] 05-03: Handlers and Routes - Create AdminTokenAuth middleware, NodeHandler, update routes
- [x] 05-04: Migration and Tests - Database migration script, tests, documentation

### Phase 6: 去掉数据库连接相关的业务逻辑，网关不要直接访问数据库，完全使用admin来控制，提供接口

**Goal:** Transform gateway into fully stateless operation — no database connection, all config from files (single-node) or admin API sync (multi-node)
**Depends on:** Phase 5
**Requirements:** GW-01, GW-02, GW-03, GW-04, GW-05, GW-06
**Plans:** 3 plans in 3 waves

Plans:
- [x] 06-01: Remove database-dependent models, services, and handlers — Wave 1
- [x] 06-02: Config-driven auth (APIKeyValidator + AdminSyncService) — Wave 2
- [x] 06-03: Wire stateless gateway, finalize entry point, update config — Wave 3

### Phase 7: 增加OCR模型配置支持GLM-OCR和PaddleOCR连接及测试用例

**Goal:** Enable OCR model configuration for GLM-OCR and PaddleOCR with full plugin support, API endpoint, and comprehensive testing
**Depends on:** Phase 6
**Requirements:** OCR-01, OCR-02, OCR-03, OCR-04
**Plans:** 4 plans in 3 waves

Plans:
- [ ] 07-01: Create OCR model configuration files for GLM-OCR and PaddleOCR - Wave 1
- [ ] 07-02: Add OCRPlugin interface and OpenAI plugin implementation - Wave 1
- [ ] 07-03: Add OCR handler, route, and service method - Wave 2
- [ ] 07-04: Write comprehensive OCR tests and verify with test image - Wave 3

### Phase 8: 模型透传/解析双模式 - 模型颗粒度配置 passthrough/parsed 模式

**Goal:** [To be planned]
**Depends on:** Phase 7
**Plans:** 0 plans

Plans:
- [ ] TBD (run /gsd:plan-phase 8 to break down)

### Phase 9: TUI 网关管理器 - 独立的终端管理界面

**Goal:** Build a standalone TUI (Terminal UI) process that manages the wolink gateway independently. The TUI runs as a separate process from the gateway — TUI exit does not affect the running gateway service. Users can start/restart/stop the gateway service, configure new providers and models, view node status, modify configuration, enable/disable plugins, and manage all gateway operations through an interactive terminal interface.

**Depends on:** Phase 8
**Requirements:** TUI-01, TUI-02, TUI-03, TUI-04, TUI-05
**Plans:** 4 plans in 4 waves

Plans:
- [x] 09-01: Foundation — Install Charm v2 deps, gateway HTTP client, process lifecycle (os/exec), styles, keymaps — Wave 1
- [x] 09-02: Dashboard — Main Bubble Tea model, health polling, status panels, tab bar navigation — Wave 2
- [x] 09-03: Providers — Form validation, provider wizard, model form, provider list with add/edit/delete — Wave 3
- [x] 09-04: Plugins + Tests — Plugin list with reload/unload, unit tests for all components — Wave 4

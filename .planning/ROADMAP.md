# Roadmap: wolink-core Production Readiness

## Overview

Transform wolink-core AI Gateway from early development to production-ready status through a focused 4-phase approach: security hardening, infrastructure hardening, observability implementation, and comprehensive testing. Each phase builds on the previous, establishing a stable foundation before adding capabilities.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [ ] **Phase 1: Security Foundation** - Configuration validation, JWT enforcement, CORS hardening, input validation
- [ ] **Phase 2: Infrastructure Hardening** - Graceful shutdown, health endpoints, connection pooling, timeouts
- [ ] **Phase 3: Observability** - Request tracing, structured logging, Prometheus metrics, error standardization
- [ ] **Phase 4: Testing & Validation** - Unit tests, integration tests, benchmarks, shutdown tests

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
**Plans**: TBD

Plans:
- [ ] 01-01: Configuration validation and fail-fast
- [ ] 01-02: JWT secret enforcement
- [ ] 01-03: CORS configuration hardening
- [ ] 01-04: Input validation enhancement

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
**Plans**: TBD

Plans:
- [ ] 02-01: Graceful shutdown implementation
- [ ] 02-02: Health and readiness endpoints
- [ ] 02-03: Database connection pooling
- [ ] 02-04: Redis connection pooling
- [ ] 02-05: HTTP server timeout configuration

### Phase 3: Observability
**Goal**: All requests are traceable and system behavior is measurable through logs and metrics
**Depends on**: Phase 2
**Requirements**: OBS-01, OBS-02, OBS-03, OBS-04
**Success Criteria** (what must be TRUE):
  1. Every request has a unique ID visible in logs and response headers
  2. Log entries are structured JSON containing request ID, timestamp, and contextual fields
  3. `/metrics` endpoint exposes request rate, latency, and error counters in Prometheus format
  4. All API errors return consistent HTTP status codes matching error type
**Plans**: TBD

Plans:
- [ ] 03-01: Request ID middleware
- [ ] 03-02: Structured JSON logging
- [ ] 03-03: Prometheus metrics endpoint
- [ ] 03-04: Error type standardization

### Phase 4: Testing & Validation
**Goal**: All critical paths have automated tests providing confidence for production deployment
**Depends on**: Phase 3
**Requirements**: TEST-01, TEST-02, TEST-03, TEST-04
**Success Criteria** (what must be TRUE):
  1. All services have unit tests with minimum 80% code coverage
  2. All API endpoints have integration tests verifying request/response behavior
  3. Critical paths (chat completion, streaming) have performance benchmarks
  4. Graceful shutdown is verified through automated tests
**Plans**: TBD

Plans:
- [ ] 04-01: Unit test suite setup and service tests
- [ ] 04-02: Integration tests for API endpoints
- [ ] 04-03: Performance benchmarks
- [ ] 04-04: Graceful shutdown tests

## Progress

**Execution Order:**
Phases execute in numeric order: 1 -> 2 -> 3 -> 4

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Security Foundation | 0/4 | Not started | - |
| 2. Infrastructure Hardening | 0/5 | Not started | - |
| 3. Observability | 0/4 | Not started | - |
| 4. Testing & Validation | 0/4 | Not started | - |

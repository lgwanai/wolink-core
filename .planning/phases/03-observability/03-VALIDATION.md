---
phase: 03
slug: observability
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-05
---

# Phase 03 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test with testify |
| **Config file** | none — Wave 0 installs |
| **Quick run command** | `go test -race -cover ./internal/observability/... ./internal/api/middleware/...` |
| **Full suite command** | `go test -race -cover ./...` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -race -cover ./internal/observability/... ./internal/api/middleware/...`
- **After every plan wave:** Run `go test -race -cover ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 03-01-01 | 01 | 1 | OBS-01 | unit | `go test -race ./internal/api/middleware/... -run TestRequestID` | ❌ W0 | ⬜ pending |
| 03-02-01 | 02 | 1 | OBS-02 | unit | `go test -race ./internal/observability/... -run TestLogger` | ❌ W0 | ⬜ pending |
| 03-03-01 | 03 | 2 | OBS-03 | unit | `go test -race ./internal/api/handlers/... -run TestMetrics` | ❌ W0 | ⬜ pending |
| 03-04-01 | 04 | 2 | OBS-04 | unit | `go test -race ./internal/observability/... -run TestAppError` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/observability/logger_test.go` — tests for context-aware logger
- [ ] `internal/observability/metrics_test.go` — tests for Prometheus metrics
- [ ] `internal/observability/errors_test.go` — tests for AppError types
- [ ] `internal/api/middleware/requestid_test.go` — tests for request ID middleware
- [ ] `internal/api/middleware/logging_test.go` — tests for structured logging middleware
- [ ] Framework install: `go get github.com/gin-contrib/requestid github.com/prometheus/client_golang`

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Metrics integration with Prometheus | OBS-03 | Requires external Prometheus server | Start server, curl /metrics, verify Prometheus can scrape |
| Request ID in distributed tracing | OBS-01 | Requires multiple services | Verify X-Request-ID propagates across service boundaries |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

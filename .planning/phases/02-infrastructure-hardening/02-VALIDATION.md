---
phase: 2
slug: infrastructure-hardening
status: draft
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-05
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (standard library) + testify |
| **Config file** | none — Phase 1 installed testify |
| **Quick run command** | `go test -v ./internal/api/... ./internal/config/...` |
| **Full suite command** | `go test -v -race -cover ./...` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -v ./internal/api/...`
- **After every plan wave:** Run `go test -v -race -cover ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 02-01-01 | 01 | 1 | INFRA-01 | unit | `go test -v ./internal/api/... -run TestGraceful` | ✅ W0 | pending |
| 02-02-01 | 02 | 1 | INFRA-02 | unit | `go test -v ./internal/api/... -run TestHealth` | ✅ W0 | pending |
| 02-02-02 | 02 | 1 | INFRA-03 | unit | `go test -v ./internal/api/... -run TestReadiness` | ✅ W0 | pending |
| 02-03-01 | 03 | 2 | INFRA-04 | unit | `go test -v ./internal/config/... -run TestDatabase` | ✅ W0 | pending |
| 02-04-01 | 04 | 2 | INFRA-05 | unit | `go test -v ./internal/config/... -run TestRedis` | ✅ W0 | pending |
| 02-05-01 | 05 | 2 | INFRA-06 | unit | `go test -v ./internal/config/... -run TestTimeout` | ✅ W0 | pending |

*Status: pending / green / red / flaky*

---

## Wave 0 Requirements

- [x] `internal/api/handlers/health_test.go` — tests for health endpoints (Phase 1 scaffold)
- [x] `internal/config/config_test.go` — tests for config (Phase 1 scaffold)
- [x] `github.com/stretchr/testify` — assertion library (Phase 1 installed)

*Existing infrastructure covers all phase requirements.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Server completes in-flight requests on SIGTERM | INFRA-01 | Requires process execution | Start server, send request, send SIGTERM during request, verify response received |
| Connection pool limits enforced | INFRA-04, INFRA-05 | Requires load testing | Use connection-exhaustion test tool |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 15s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** complete

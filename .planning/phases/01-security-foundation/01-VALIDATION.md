---
phase: 1
slug: security-foundation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-04
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (standard library) |
| **Config file** | none — Wave 0 installs testify |
| **Quick run command** | `go test -v ./internal/config/... ./internal/api/middleware/... ./internal/services/security_service_test.go` |
| **Full suite command** | `go test -v -race -cover ./...` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -v ./internal/config/... ./internal/api/middleware/...`
- **After every plan wave:** Run `go test -v -race -cover ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 01-01-01 | 01 | 1 | SEC-01 | unit | `go test -v ./internal/config/...` | ❌ W0 | ⬜ pending |
| 01-01-02 | 01 | 1 | SEC-01 | unit | `go test -v ./internal/config/...` | ❌ W0 | ⬜ pending |
| 01-02-01 | 02 | 1 | SEC-02 | unit | `go test -v ./internal/config/...` | ❌ W0 | ⬜ pending |
| 01-03-01 | 03 | 1 | SEC-03 | unit | `go test -v ./internal/api/middleware/...` | ❌ W0 | ⬜ pending |
| 01-03-02 | 03 | 1 | SEC-03 | unit | `go test -v ./internal/api/middleware/...` | ❌ W0 | ⬜ pending |
| 01-04-01 | 04 | 1 | SEC-04 | unit | `go test -v ./internal/services/...` | ❌ W0 | ⬜ pending |
| 01-04-02 | 04 | 1 | SEC-04 | unit | `go test -v ./internal/api/handlers/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/config/config_test.go` — tests for config validation
- [ ] `internal/api/middleware/cors_test.go` — tests for CORS middleware
- [ ] `internal/services/security_service_test.go` — tests for input validation
- [ ] `go get github.com/stretchr/testify` — assertion library

*Existing test infrastructure: None detected in codebase*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Application exits on missing JWT secret | SEC-02 | Requires process execution | Run `JWT_SECRET="" go run cmd/main.go` and verify exit code != 0 |
| CORS rejects unauthorized origin in production | SEC-03 | Requires HTTP server running | Start server in production mode, curl with Origin header not in allowlist, verify rejection |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

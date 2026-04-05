---
phase: 04
slug: testing-validation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-05
---

# Phase 04 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + testify v1.11.1 |
| **Config file** | none — use `//go:build` tags for integration tests |
| **Quick run command** | `go test -race ./...` |
| **Full suite command** | `go test -race -cover ./...` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -race ./internal/<modified-package>/...`
- **After every plan wave:** Run `go test -race -cover ./...`
- **Before `/gsd:verify-work`:** Full suite must be green with 80%+ coverage
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 04-01-01 | 01 | 1 | TEST-01 | unit | `go test -race -cover ./internal/services/...` | Partial | ⬜ pending |
| 04-02-01 | 02 | 1 | TEST-02 | integration | `go test -race ./internal/api/...` | Yes | ⬜ pending |
| 04-03-01 | 03 | 2 | TEST-03 | benchmark | `go test -bench=. -benchmem ./internal/...` | ❌ W0 | ⬜ pending |
| 04-04-01 | 04 | 2 | TEST-04 | integration | `go test -race ./cmd/...` | Yes | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/services/auth_service_test.go` — unit tests for AuthService
- [ ] `internal/services/plugin_service_test.go` — unit tests for PluginService
- [ ] `internal/plugins/plugin_openai_test.go` — unit tests with HTTP mocking
- [ ] `internal/api/handlers/chat_handler_test.go` — handler tests for chat completion
- [ ] `internal/api/handlers/admin_handler_test.go` — handler tests for admin endpoints
- [ ] `internal/testutil/mocks/` — shared mock implementations
- [ ] `*_bench_test.go` files — performance benchmarks
- [ ] Integration test build tags: `//go:build integration`

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Coverage enforcement in CI | TEST-01 | Requires CI pipeline | Verify coverage gates work in CI |
| Performance regression detection | TEST-03 | Requires baseline comparison | Compare benchmark results to baseline |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

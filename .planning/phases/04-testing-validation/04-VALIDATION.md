---
phase: 04
slug: testing-validation
status: planned
nyquist_compliant: true
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
| 04-01-01 | 01 | 1 | TEST-01 | unit | `go test -race -cover ./internal/services/...` | Partial | ✅ planned |
| 04-01-02 | 01 | 1 | TEST-01 | unit | `go test -race -cover ./internal/services/...` | No | ✅ planned |
| 04-01-03 | 01 | 1 | TEST-01 | unit | `go test -race -cover ./internal/services/...` | No | ✅ planned |
| 04-02-01 | 02 | 1 | TEST-02 | integration | `go test -race -cover ./internal/plugins/...` | No | ✅ planned |
| 04-02-02 | 02 | 1 | TEST-02 | integration | `go test -race -cover ./internal/api/handlers/...` | No | ✅ planned |
| 04-02-03 | 02 | 1 | TEST-02 | integration | `go test -race -cover ./internal/api/handlers/...` | No | ✅ planned |
| 04-03-01 | 03 | 2 | TEST-03 | benchmark | `go test -bench=. -benchmem ./internal/services/...` | No | ✅ planned |
| 04-03-02 | 03 | 2 | TEST-03 | benchmark | `go test -bench=. -benchmem ./internal/api/handlers/...` | No | ✅ planned |
| 04-03-03 | 03 | 2 | TEST-03 | benchmark | `go test -bench=. -benchmem ./internal/plugins/...` | No | ✅ planned |
| 04-04-01 | 04 | 2 | TEST-04 | integration | `go test -race ./cmd/... -run TestGracefulShutdown` | Yes | ✅ planned |
| 04-04-02 | 04 | 2 | TEST-04 | unit | `go test -race ./internal/utils/...` | No | ✅ planned |
| 04-04-03 | 04 | 2 | TEST-04 | integration | `go test -race -tags=integration ./cmd/...` | No | ✅ planned |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

None required - all tasks have clear automated verification paths.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Coverage enforcement in CI | TEST-01 | Requires CI pipeline | Verify coverage gates work in CI |
| Performance regression detection | TEST-03 | Requires baseline comparison | Compare benchmark results to baseline |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (none needed)
- [x] No watch-mode flags
- [x] Feedback latency < 30s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

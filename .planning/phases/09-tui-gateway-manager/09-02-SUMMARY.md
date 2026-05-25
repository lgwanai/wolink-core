---
phase: 09-tui-gateway-manager
plan: 09-02
type: execute
status: complete
subsystem: tui
tags:
  - bubbletea
  - tui
  - dashboard
  - lifecycle
requires: []
provides:
  - model.go (Bubble Tea Model struct, Init, Update, View)
  - commands.go (tea.Cmd factories for health polling, status fetches, lifecycle actions)
  - dashboard.go (Dashboard tab with status panels and lifecycle keybindings)
  - dashboard_test.go (behavior tests)
affects:
  - main.go (entry point updated to startTUI)
tech-stack:
  added:
    - charm.land/bubbletea/v2 (TUI framework)
    - charm.land/bubbles/v2/spinner (lifecycle progress indicator)
  patterns:
    - Bubble Tea v2 Elm Architecture (Model-Update-View)
    - Periodic polling via tea.Tick
    - Lifecycle commands via tea.Batch and tea.Sequence
    - Tab-based navigation with keyboard dispatch
key-files:
  modified:
    - cmd/tui/main.go: updated entry point to call startTUI()
    - cmd/tui/model.go: added spinner support
  created:
    - cmd/tui/model.go: 347 lines — Bubble Tea v2 Model with Init/Update/View, tab dispatch, polling
    - cmd/tui/commands.go: 126 lines — tea.Cmd factories for health, status, lifecycle
    - cmd/tui/dashboard.go: 277 lines — Dashboard tab with 5 panels and lifecycle keybindings
    - cmd/tui/dashboard_test.go: 100 lines — 8 behavior tests
decisions:
  - dashboard.go placed in cmd/tui/ instead of cmd/tui/tabs/ (Go package system requires same package)
  - Spinner integrated via bubbles/v2/spinner with Dot animation style
  - Lifecycle blocking commands use tea.Sequence for step-by-step progress updates
  - Key matching uses msg.String() (v2 KeyPressMsg.String()) for consistency with plan pattern
metrics:
  duration: 12 min
  completed_date: "2026-05-25"
---

# Phase 9 Plan 2: Main Model + Dashboard Tab

**One-liner:** Bubble Tea v2 Model with health polling, lifecycle commands, and Dashboard tab with 5 status panels (Gateway Status, Node Metrics, Quick Actions, Lifecycle, Errors) and Ctrl+S/Ctrl+X/Ctrl+R keybindings.

## Task Summary

| # | Task | Type | Status | Commit |
|---|------|------|--------|--------|
| 1 | Create main Model struct and message types with Init/Update/View | auto | Complete | 66b65da |
| 2 | Create Dashboard tab with live status panels and lifecycle action keybindings | auto (tdd) | Complete | d488bb8 |

## Verifications

- `go build ./cmd/tui/...` — PASS
- `go vet ./cmd/tui/...` — PASS
- `go test ./cmd/tui/... -run TestRenderDashboard` — all 6 tests pass
- `go test ./cmd/tui/... -run TestHandleDashboard` — both tests pass

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Path] dashboard.go moved from cmd/tui/tabs/ to cmd/tui/**
- **Found during:** Task 1 build
- **Issue:** Go's package system requires all files in the same `cmd/tui/` directory to belong to the same package. Putting `dashboard.go` in a subdirectory `cmd/tui/tabs/` creates a separate package that cannot access the unexported `model` type.
- **Fix:** Placed `dashboard.go` at `cmd/tui/dashboard.go` instead of `cmd/tui/tabs/dashboard.go`. The tabs/ subdirectory approach is aspirational from the planning stage but incompatible with Go's package rules for unexported type access.
- **Files modified:** cmd/tui/dashboard.go (location), cmd/tui/dashboard_test.go (location)
- **Commit:** 66b65da

**2. [Rule 2 - Missing] startGatewayCmd signature change**
- **Found during:** Task 1 implementation
- **Issue:** The plan specified `startGatewayCmd(gwLifecycle, pollInterval)` but the function needs the gateway URL to poll /health for readiness. `pollInterval` is not needed for the health polling itself.
- **Fix:** Changed signature to `startGatewayCmd(gwLifecycle, gatewayURL string)` — uses the gateway URL for direct HTTP health checks during the 5s readiness wait.
- **Files modified:** cmd/tui/commands.go
- **Commit:** 66b65da

### v2 API Compatibility Notes

- `View()` returns `tea.View` not `string` in Bubble Tea v2 (plan was written with v1 conventions)
- `tea.RequestBackgroundColor` is a `Msg` not a `Cmd` in v2; used as `tea.RequestBackgroundColor` (function value) in `tea.Batch` because `Cmd` is defined as `func() Msg`
- `tea.KeyMsg` in v2 is an interface (`KeyPressMsg`/`KeyReleaseMsg`); constructed via `tea.KeyPressMsg(tea.Key{...})` for tests

## Threat Surface Scan

No new threat surface introduced. The dashboard renders only `NodeStatus` fields and health state. Lifecycle commands return error messages on failure per threat model T-09-10.

## Self-Check

- [x] cmd/tui/model.go exists (347 lines, includes Init/Update/View/lifecycleState/startGatewayMsg/restartCompleteMsg)
- [x] cmd/tui/commands.go exists (126 lines, includes startGatewayCmd/stopGatewayCmd/restartGatewayCmd)
- [x] cmd/tui/dashboard.go exists (277 lines, includes renderDashboard/handleDashboardKeyMsg)
- [x] cmd/tui/main.go updated (contains tea.NewProgram, calls startTUI)
- [x] go build ./cmd/tui/... exits 0
- [x] go vet ./cmd/tui/... passes
- [x] 8 dashboard tests pass
- [x] Commit 66b65da exists
- [x] Commit d488bb8 exists

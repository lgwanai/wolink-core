---
phase: 09-tui-gateway-manager
plan: 09-04
subsystem: ui
tags: [tui, bubbletea, plugins, testing, go]

requires:
  - phase: 09-tui-gateway-manager
    provides: TUI model, commands, gateway client, lifecycle, styles, providers tab, dashboard tab

provides:
  - Plugins tab with list rendering, reload/unload/refresh/health-check actions
  - Numeric tab switching with auto-load
  - Full test suite: lifecycle, client HTTP, validation, model

affects: []

tech-stack:
  added: []
  patterns:
    - Plugin tab following providers tab pattern (strings.Builder rendering, simple key handling)
    - Bubble Tea model testing via direct Update() calls with message types

key-files:
  created:
    - cmd/tui/plugins.go
    - cmd/tui/gateway/lifecycle_test.go
    - cmd/tui/gateway/client_test.go
    - cmd/tui/forms/validation_test.go
    - cmd/tui/model_test.go
  modified:
    - cmd/tui/model.go

key-decisions:
  - "Constant naming: prefixed plugins state constants (pluginsStateIdle, pluginsStateLoading, pluginsStateList, pluginsStateError) to avoid collision with providers tab's pList/pAdd/pEdit"
  - "Plugin tab file placed in cmd/tui/ (not cmd/tui/tabs/) to keep package main namespace and match existing dashboard.go/providers.go pattern"
  - "Text-based table rendering using strings.Builder (same as providers.go) instead of Bubbles table component for consistency and simplicity"

requirements-completed:
  - TUI-04  # Plugin enable/disable via admin API
  - TUI-01  # Gateway lifecycle management
  - TUI-05  # TUI runs as independent process

duration: 45min
completed: 2026-05-25
---

# Phase 9 Plan 4: Plugins Tab with Reload/Unload and Comprehensive Unit Tests

**Plugins tab with list, reload, unload, health-check actions and full test suite for lifecycle, client, validation, and model.**

## Performance

- **Duration:** ~45 min
- **Started:** 2026-05-25T06:47:00Z
- **Completed:** 2026-05-25T07:32:00Z
- **Tasks:** 2
- **Files modified:** 6 (1 new, 1 modified, 4 test files)

## Accomplishments

- Plugins tab with list table showing Name, Protocol, Version, Loaded, Healthy (color-coded yes/no)
- Reload, unload, refresh, health-check actions via admin API with auto-refresh after each action
- Numeric tab switching (1, 2, 3) with auto-loading of plugin list on first visit
- 26 lifecycle tests: constructor, PID, IsRunning, start/stop/restart (error paths and integration), SetBinaryPath, SetOutput, SysProcAttr verification, concurrent access race test
- 22 client HTTP tests: Health, NodeStatus, ListPlugins, ReloadPlugin, UnloadPlugin, HealthCheckPlugins, auth error, timeout, X-Admin-Token header, URL normalization, malformed JSON, APIError formatting
- 48 validation tests: ValidateProtocol, ValidateBaseURL, ValidatePort, ValidateRequired, ValidateYAML, ValidateModelID with table-driven subtests
- 34 model tests: Init, WindowSize, BackgroundColor, statusUpdateMsg, healthMsg, errMsg, lifecycle messages, tab navigation (tab/vim/numeric), quit/ctrl+c, plugin messages, renderContent all tabs, renderPluginsContent states, plugin key navigation, loading state ignore

## Task Commits

1. **Task 1: Create Plugins tab with list, reload, unload and wire into model** - `970c78f` (feat)
2. **Task 2: Write comprehensive unit tests for all TUI components** - `6279e58` (test)

## Files Created/Modified

- `cmd/tui/plugins.go` - Plugins tab with state management, list rendering, key handling, and tea.Cmd factories for API calls
- `cmd/tui/model.go` - Added plugin state fields, message handlers, numeric tab switching with auto-load
- `cmd/tui/gateway/lifecycle_test.go` - Unit tests for GatewayLifecycle methods
- `cmd/tui/gateway/client_test.go` - Unit tests for GatewayClient HTTP calls using httptest.NewServer
- `cmd/tui/forms/validation_test.go` - Unit tests for form validation rules
- `cmd/tui/model_test.go` - Unit tests for model Update message handling and tab navigation

## Decisions Made

- **Constant naming:** Prefixed plugin state constants to avoid collision with providers tab's `pList` and other declarations
- **File placement:** Plugin tab placed directly in `cmd/tui/` (not `cmd/tui/tabs/`) to match existing pattern and avoid package boundary issues
- **Rendering approach:** Text-based table via strings.Builder (same pattern as providers.go) instead of Bubbles table component for consistency and simplicity

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Renamed plugin state constants to avoid collision**
- **Found during:** Task 1 (Create Plugins tab)
- **Issue:** Plugin state constants `pList`, `pLoading`, etc. collided with providers tab constants in the same `package main` namespace
- **Fix:** Prefixed all plugin state constants with `pluginsState` (e.g., `pluginsStateIdle`, `pluginsStateList`)
- **Files modified:** cmd/tui/plugins.go, cmd/tui/model.go
- **Verification:** Build passes
- **Committed in:** 970c78f (Task 1 commit)

**2. [Rule 3 - Blocking] Fixed test type assertions for handlePluginsKeyMsg return type**
- **Found during:** Task 2 (Model tests)
- **Issue:** `handlePluginsKeyMsg` returns `(model, tea.Cmd)` (concrete struct), but tests used `result.(model)` type assertion (as if returning `(tea.Model, tea.Cmd)`)
- **Fix:** Changed to direct variable assignment `updated, _ := handlePluginsKeyMsg(...)`
- **Files modified:** cmd/tui/model_test.go
- **Verification:** Tests pass
- **Committed in:** 6279e58 (Task 2 commit)

**3. [Rule 3 - Blocking] Added numeric tab switching for '1', '2', '3' keys**
- **Found during:** Task 2 (Model tests)
- **Issue:** Tests expected numeric key switching but model.go only handled "tab", "l", "shift+tab", "h"
- **Fix:** Added "1" -> tabDashboard, "2" -> tabProviders, "3" -> tabPlugins case handlers with auto-loading
- **Files modified:** cmd/tui/model.go
- **Verification:** Tests pass
- **Committed in:** 6279e58 (Task 2 commit)

---

**Total deviations:** 3 auto-fixed (3 blocking)
**Impact on plan:** All auto-fixes necessary for correctness. No scope creep.

## Issues Encountered

- Pre-existing test failures in dashboard_test.go (TestHandleDashboardKeyMsg_CtrlS, TestHandleDashboardKeyMsg_CtrlR) - these test unimplemented dashboard key handlers and predate this plan
- Go template syntax `{{.foo}}` is not valid YAML (yaml.v3 treats it as an invalid map key)
- HTTP timeout errors use "context deadline exceeded" not "timeout" in Go's net/http client
- API response with JSON error body is returned as APIError struct, not raw "unexpected status" text

## Next Phase Readiness

- All three tabs (Dashboard, Providers, Plugins) are fully wired into the main model
- Comprehensive test coverage for gateway lifecycle, HTTP client, form validation, and model message handling
- Phase 09 is complete - TUI Gateway Manager is fully implemented and tested

---
*Phase: 09-tui-gateway-manager*
*Completed: 2026-05-25*

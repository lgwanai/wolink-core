---
phase: 09-tui-gateway-manager
plan: 09-01
subsystem: ui
tags: [bubbletea, bubbles, lipgloss, tui, charm, os-exec]

# Dependency graph
requires:
  - phase: 06-stateless-gateway
    provides: admin API endpoints (GET /admin/node/status, GET /admin/plugins, POST/DELETE plugin routes)
provides:
  - TUI config struct with flag/env parsing
  - Gateway HTTP client for all admin API calls
  - Gateway process lifecycle manager (build/start/stop/restart)
  - Lipgloss style definitions with light/dark adaptation
  - Vim-style keybinding definitions
  - Main entry point scaffold (diagnostic mode, no Bubble Tea yet)
affects: [09-tui-gateway-manager plans 09-02, 09-03, 09-04]

# Tech tracking
tech-stack:
  added:
    - charm.land/bubbletea/v2 v2.0.6
    - charm.land/bubbles/v2 v2.1.0
    - charm.land/lipgloss/v2 v2.0.3
  patterns:
    - Bubble Tea Elm Architecture (Model-Update-View) prepared in config+styles+keymaps
    - Gateway process lifecycle: exec.Command + Setsid + Setpgid + Process.Release
    - HTTP client wrapping admin API with X-Admin-Token auth
    - Connfig-driven style factory (newStyles(isDark bool)) using lipgloss.LightDark
    - Keybinding definitions implementing help.KeyMap interface

key-files:
  created:
    - cmd/tui/config.go
    - cmd/tui/gateway/client.go
    - cmd/tui/gateway/lifecycle.go
    - cmd/tui/styles.go
    - cmd/tui/keymap.go
    - cmd/tui/main.go
  modified:
    - go.mod
    - go.sum

key-decisions:
  - "Bubbles v2 key.Binding replaces researched help.KeyBinding — actual Bubbles v2 API uses key.NewBinding with key.WithKeys/key.WithHelp functional options"
  - "lipgloss.LightDark returns func(light, dark color.Color) color.Color; requires image/color import for type-safe wrapper"
  - "GatewayClient uses single http.Client with configurable timeout per operation type (5s health, 10s admin)"

patterns-established:
  - "Gateway HTTP calls: doRequest() helper with X-Admin-Token, JSON unmarshal typed structs, error wrapping via APIError"
  - "Process lifecycle: EnsureBuilt() builds from source, Start() detaches via Setsid+Setpgid+Release, Stop() sends SIGTERM with configurable timeout"
  - "Style creation: newStyles(isDark) factory with local ld() helper wrapping lipgloss.LightDark"
  - "Keybinding definition: NewKeymap() returns keymap struct with key.NewBinding + key.WithKeys + key.WithHelp"

requirements-completed:
  - TUI-01
  - TUI-05

# Metrics
duration: 12min
completed: 2026-05-25
---

# Phase 09 Plan 01: TUI Gateway Manager Foundation

**Charm Bubble Tea v2 ecosystem packages, TUI config struct, gateway HTTP client, process lifecycle manager, lipgloss styles, and keybinding definitions**

## Performance

- **Duration:** 12 min
- **Started:** 2026-05-25T06:00:00Z
- **Completed:** 2026-05-25T06:12:00Z
- **Tasks:** 3 (1 checkpoint + 2 auto)
- **Files modified:** 8 (6 created, 2 modified)

## Accomplishments

- Charm v2 packages installed and verified: charm.land/bubbletea/v2 (v2.0.6), charm.land/bubbles/v2 (v2.1.0), charm.land/lipgloss/v2 (v2.0.3)
- TUIConfig struct with flag/env parsing supporting GatewayURL, AdminToken, GatewayBinary, PollInterval, ModelsDir, PluginsDir, WorkingDir
- GatewayClient with Health(), NodeStatus(), ListPlugins(), ReloadPlugin(), UnloadPlugin(), HealthCheckPlugins() — all via X-Admin-Token auth
- GatewayLifecycle with Start() (Setsid+Setpgid+Release for detached child), Stop() (SIGTERM + timeout), EnsureBuilt(), IsRunning(), PID()
- lipgloss styles struct with 14 named styles and light/dark adaptation via lipgloss.LightDark; RenderStatusDot() status indicator
- Vim-style keybinding definitions using Bubbles v2 key.Binding implementing help.KeyMap interface
- main.go entry point with diagnostic mode (no Bubble Tea yet — Plan 02 adds interactive TUI)

## Task Commits

Each task was committed atomically:

1. **Task 0: Verify and install Charm Bubble Tea v2 ecosystem packages** - (checkpoint, approved by user)
2. **Task 1: Create TUIConfig, gateway HTTP client, and gateway process lifecycle** - `367e6c6` (feat)
3. **Task 2: Create lipgloss styles, keybinding definitions, and main.go entry point scaffold** - `ab1c82b` (feat)

**Plan metadata:** (final commit)

## Files Created/Modified

- `cmd/tui/config.go` — TUIConfig struct with flag/env parsing, env() helper, DefaultConfig()
- `cmd/tui/gateway/client.go` — GatewayClient with all admin API methods, APIError type, doRequest() helper
- `cmd/tui/gateway/lifecycle.go` — GatewayLifecycle with Start/Stop/Restart/EnsureBuilt/IsRunning/PID methods
- `cmd/tui/styles.go` — styles struct with 14 Lipgloss styles, newStyles(isDark) factory, RenderStatusDot()
- `cmd/tui/keymap.go` — keymap struct with key.Binding fields, NewKeymap(), ShortHelp()/FullHelp() implementations
- `cmd/tui/main.go` — Entry point with flag parsing, env var fallback, diagnostic banner (no Bubble Tea yet)
- `go.mod` — Added charm.land/bubbletea/v2 v2.0.6, charm.land/bubbles/v2 v2.1.0, charm.land/lipgloss/v2 v2.0.3
- `go.sum` — Updated with Charm v2 dependency hashes

## Decisions Made

- Used Bubbles v2 `key.Binding` with `key.NewBinding(key.WithKeys(...), key.WithHelp(...))` instead of the researched `help.KeyBinding` — the actual v2 API uses the `key` subpackage with functional options pattern
- Wrapped `lipgloss.LightDark(isDark)` return value through a local `ld()` helper that converts from `image/color.Color` — the function returns `func(light, dark color.Color) color.Color` and raw hex strings do not implement `color.Color`
- Single `http.Client` with configurable timeout for all gateway operations; the client is shared across all methods of `GatewayClient`
- `Stop()` uses a goroutine+channel pattern with `time.After` for configurable shutdown timeout; does NOT send SIGKILL on timeout (per graceful shutdown design)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Bubbles v2 help package API mismatch**
- **Found during:** Task 2 (keymap.go creation)
- **Issue:** Plan specified `help.KeyBinding` and `help.NewKeyBinding`, but Bubbles v2 uses `key.Binding` from `charm.land/bubbles/v2/key` with `key.NewBinding(key.WithKeys(...), key.WithHelp(...))` functional options pattern
- **Fix:** Changed import from `charm.land/bubbles/v2/help` to `charm.land/bubbles/v2/key`, replaced `help.KeyBinding` with `key.Binding`, replaced `help.NewKeyBinding()` with `key.NewBinding(key.WithKeys(...), key.WithHelp(...))`. Also added `ShortHelp()` and `FullHelp()` methods implementing `help.KeyMap` interface instead of the planned standalone `FullHelp()` method
- **Files modified:** cmd/tui/keymap.go
- **Verification:** `go build ./cmd/tui/...` exits 0
- **Committed in:** ab1c82b (Task 2 commit)

**2. [Rule 3 - Blocking] Lipgloss v2 LightDark requires color.Color values, not strings**
- **Found during:** Task 2 (styles.go creation after build failure)
- **Issue:** `lipgloss.LightDark(isDark)` returns `func(light, dark color.Color) color.Color`. The initial code passed hex strings directly (e.g., `ld(isDark, "#cccccc", "#333333")`) but strings do not implement `color.Color` interface
- **Fix:** Wrapped all hex color values with `lipgloss.Color("#...")` which returns `color.Color`. Added local `ld()` helper function that imports `image/color` and uses `color.Color` parameter types. Removed unused import of `image/color` (it was already imported via the `color.Color` type use)
- **Files modified:** cmd/tui/styles.go
- **Verification:** `go build ./cmd/tui/...` exits 0
- **Committed in:** ab1c82b (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (both Rule 3 - Blocking)
**Impact on plan:** Both auto-fixes were necessary to match the actual Charm v2 APIs versus the researched documentation. No scope creep — these are exact API corrections.

## Issues Encountered

- Charm v2 API documentation in research was partially inaccurate: `help.KeyBinding` does not exist in Bubbles v2; replaced with `key.Binding` from the `key` subpackage
- `lipgloss.LightDark` API requires `color.Color` interface values, not raw strings; resolved by wrapping with `lipgloss.Color()` calls
- Bubbles v2 help package uses a `KeyMap` interface (with `ShortHelp()` and `FullHelp()` methods) rather than direct `KeyBinding` types; keymap struct now implements this interface

## User Setup Required

None — no external service configuration required. All dependencies resolved via `go get`.

## Next Phase Readiness

- Foundation ready for Plan 02 (interactive TUI model with tab switching)
- GatewayClient and GatewayLifecycle are complete but untested in this plan — Plan 02 will wire them into the Bubble Tea model
- styles.go, keymap.go, and config.go are ready for import by the main TUI model in Plan 02
- `go build ./cmd/tui/...` and `go vet ./cmd/tui/...` both pass

---
*Phase: 09-tui-gateway-manager*
*Completed: 2026-05-25*

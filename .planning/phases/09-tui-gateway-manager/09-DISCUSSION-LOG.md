# Phase 9: TUI 网关管理器 - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-25
**Phase:** 09-tui-gateway-manager
**Areas discussed:** TUI Framework, Gateway Process Lifecycle, Config & Provider Editing UX, Navigation & Layout Structure

---

## TUI Framework

| Option | Description | Selected |
|--------|-------------|----------|
| Bubble Tea + Bubbles | Elm Architecture, largest Go TUI ecosystem, rich component library | ✓ |
| tview | Immediate-mode with built-in widget primitives | |
| Raw bubbletea (no Bubbles) | Lighter but requires custom components | |

**User's choice:** Bubble Tea + Bubbles (Recommended)
**Notes:** —

---

## TUI Code Layout

| Option | Description | Selected |
|--------|-------------|----------|
| New cmd/tui/ directory | Separate binary, clean separation from gw and CLI | ✓ |
| Extend cmd/cli/ with TUI mode | Simpler packaging but mixes concerns | |
| Separate top-level tui/ directory | Maximum isolation, more complex | |

**User's choice:** New cmd/tui/ directory (Recommended)

---

## TUI Visual Style

| Option | Description | Selected |
|--------|-------------|----------|
| Dashboard with live panels | Multi-pane layout, status bar, live metrics, lipgloss styling | ✓ |
| Menu-driven wizard style | Sequential menus and forms, less info density | |
| Minimal terminal integration | Keyboard-driven, minimal visual output | |

**User's choice:** Dashboard with live panels (Recommended)

---

## Keyboard Navigation

| Option | Description | Selected |
|--------|-------------|----------|
| Vim-style + arrows | j/k, h/l, tab, / for search, q/esc for back | ✓ |
| Arrow keys + Enter only | Simplest and most accessible | |
| Custom configurable bindings | Most flexible but complex | |

**User's choice:** Vim-style + arrows (Recommended)

---

## Gateway Process Lifecycle

| Option | Description | Selected |
|--------|-------------|----------|
| Start as child + API health check | os/exec spawn, PID + /health polling, gw survives TUI exit | ✓ |
| Connect-only mode | Only connect to already-running gw | |
| Both modes | Auto-detect: connect or offer to start | |

**User's choice:** Start as child + API health check (Recommended)

---

## Gateway Binary Discovery

| Option | Description | Selected |
|--------|-------------|----------|
| Build from source + path config | go build ./cmd/ + configurable binary path | ✓ |
| Pre-built binary path only | Manual build step before TUI can start | |
| go run cmd/main.go | Can't easily detach process | |

**User's choice:** Build from source + path config (Recommended)

---

## Restart Behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Graceful stop + fresh start | SIGTERM, wait for shutdown, then start new process | ✓ |
| Kill + start | SIGKILL, loses in-flight requests | |
| SIGHUP reload | Would need gw changes | |

**User's choice:** Graceful stop + fresh start (Recommended)

---

## Status Display

| Option | Description | Selected |
|--------|-------------|----------|
| Live polling with status indicator | Poll /health + /admin/node/status every N sec | ✓ |
| On-demand refresh only | Manual key press to refresh | |
| WebSocket push | Would need gw changes | |

**User's choice:** Live polling with status indicator (Recommended)

---

## Config & Provider Editing UX

| Option | Description | Selected |
|--------|-------------|----------|
| Form-based with validation | Structured forms, field validation, prevents YAML errors | ✓ |
| Built-in YAML text editor | More power but easy to make syntax mistakes | |
| Hybrid: forms + raw editor | Most complex, most flexible | |

**User's choice:** Form-based with validation (Recommended)

---

## Adding New Providers/Models

| Option | Description | Selected |
|--------|-------------|----------|
| Guided wizard with templates | Step-by-step: protocol → URL → key → models | ✓ |
| Manual YAML + validate | Less hand-holding, more flexible | |
| Clone existing + modify | Fast for similar providers | |

**User's choice:** Guided wizard with templates (Recommended)

---

## Config Validation Timing

| Option | Description | Selected |
|--------|-------------|----------|
| Real-time field + save-time YAML | Validate as user types + full parse on save | ✓ |
| Save-time only | All errors discovered at once | |
| Save + pre-restart dry-run | Most thorough, needs gw support | |

**User's choice:** Real-time field validation + save-time YAML check (Recommended)

---

## Config Scope

| Option | Description | Selected |
|--------|-------------|----------|
| Models + plugins config | configs/models/ + configs/plugins/, main config view-only | ✓ |
| All config files | Full control but more complex | |
| Models only | Simplest but least helpful | |

**User's choice:** Models + plugins config (Recommended)

---

## Navigation & Layout Structure

| Option | Description | Selected |
|--------|-------------|----------|
| Tab bar at top | Horizontal tabs: [Dashboard] [Providers] [Plugins] | ✓ |
| Sidebar + content area | IDE-like, wastes terminal width | |
| Single dashboard with popups | Maximum density, harder to scale | |

**User's choice:** Tab bar at top (Recommended)

---

## Tab Selection

| Option | Description | Selected |
|--------|-------------|----------|
| Dashboard + Providers + Plugins | Clean, focused, covers stated needs | ✓ |
| Dashboard + Providers + Plugins + Logs | Log tab adds debugging but more complex | |
| Dashboard + Providers + Plugins + Settings | Settings tab for operational preferences | |

**User's choice:** Dashboard + Providers + Plugins (Recommended)

---

## Color Scheme

| Option | Description | Selected |
|--------|-------------|----------|
| Auto-detect terminal theme | Lipgloss adaptive, green/yellow/red/blue | ✓ |
| Dark theme only | Fixed dark palette | |
| Minimal/no color | Monochrome, less visually informative | |

**User's choice:** Auto-detect terminal theme (Recommended)

---

## Plugin Enable/Disable UX

| Option | Description | Selected |
|--------|-------------|----------|
| Toggle via admin API | POST/DELETE plugin endpoints, status refreshes | ✓ |
| Toggle + edit plugin config on disk | More powerful but risky | |
| View-only plugin status | Simpler but doesn't fully meet requirements | |

**User's choice:** Toggle via admin API (Recommended)

---

## Claude's Discretion

- Specific Bubble Tea component choices for each view
- Form field definitions and validation rules (derived from existing config structs)
- Polling interval default and configuration mechanism
- Error handling patterns for API call failures in TUI
- TUI binary build configuration

## Deferred Ideas

None — discussion stayed within phase scope.

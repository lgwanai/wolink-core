# Phase 9: TUI 网关管理器 - Context

**Gathered:** 2026-05-25
**Status:** Ready for planning

<domain>
## Phase Boundary

Build a standalone TUI binary (`cmd/tui/`) that manages the wolink gateway independently. The TUI runs as a separate process — TUI exit does not affect the running gateway service. It manages gateway lifecycle (start/stop/restart via os/exec), configures providers and models (YAML file editing via structured forms), views node status (live polling of admin API), and enables/disables plugins (via admin API calls).

The TUI is NOT an HTTP server — it is an interactive terminal application that communicates with the gateway through existing HTTP APIs (`/admin/*`, `/health`) and directly reads/writes YAML config files on disk.

</domain>

<decisions>
## Implementation Decisions

### TUI Framework
- **D-01:** Use Bubble Tea + Bubbles (Elm Architecture) as the TUI framework. Largest Go TUI ecosystem with rich component library.
- **D-02:** TUI code lives in `cmd/tui/` as a separate binary entry point, distinct from `cmd/main.go` (gateway) and `cmd/cli/main.go` (CLI tool).
- **D-03:** Dashboard style with live panels — multi-pane layout with status bar and live-updating metrics using lipgloss for styling.
- **D-04:** Vim-style keyboard navigation with arrow key fallback — j/k for up/down, tab/h/l for left/right, / for search, q/esc for back.

### Gateway Process Lifecycle
- **D-05:** TUI spawns gateway as a child process via os/exec, monitors via PID + periodic `/health` API polling. TUI can start/stop/restart. If TUI exits, gateway continues running (separate process group).
- **D-06:** Build gateway binary from source (`go build ./cmd/`) plus support a configurable binary path via env var for pre-built binaries.
- **D-07:** Graceful restart: send SIGTERM, wait for graceful shutdown with configurable timeout, then start new process with same config. Show restart progress in TUI.
- **D-08:** Live polling with status indicator: poll `/health` and `/admin/node/status` every N seconds (configurable, default 3s). Colored status dot + uptime + goroutines + memory in status bar.

### Config & Provider Editing UX
- **D-09:** Form-based config editing with real-time field validation and save-time YAML parse check. Prevents YAML syntax errors through structured forms.
- **D-10:** Guided wizard for adding new providers/models: select protocol → base URL → API key → then add individual models. Generates valid provider YAML on save.
- **D-11:** Real-time field validation (e.g., port range, URL format) plus save-time complete YAML validation. Warn if gateway is running (restart needed for changes to take effect).
- **D-12:** TUI manages models config (`configs/models/*.yaml`) and plugins config (`configs/plugins/*.yaml`). Main `config.yaml` is view-only with a pointer to edit manually (rarely changed).

### Navigation & Layout
- **D-13:** Tab bar navigation at top — horizontal tabs switch between full-screen views. Standard for operations tools, works well with Bubble Tea.
- **D-14:** Three tabs: Dashboard (gw status, uptime, metrics), Providers (list/add/edit/delete providers and models), Plugins (list loaded plugins, enable/disable, health status).
- **D-15:** Auto-detect terminal theme colors via lipgloss adaptive colors. Status colors: green=ok, yellow=warning, red=error, blue=info.
- **D-16:** Plugin enable/disable via gateway admin API — calls `POST /admin/plugins/:protocol/reload` and `DELETE /admin/plugins/:protocol`. Status refreshes after each action.

### Claude's Discretion
- Specific Bubble Tea component choices (which Bubbles components for each view)
- Form field definitions and validation rules (derived from existing config structs)
- Polling interval default and configuration mechanism
- Error handling patterns for API call failures in TUI
- TUI binary build configuration (Makefile or go build integration)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Gateway Codebase
- `internal/api/routes.go` — All gateway HTTP endpoints the TUI will call
- `internal/api/handlers/plugin_handler.go` — Plugin list/reload/unload handler signatures
- `internal/services/node_service.go` — Node status and restart service
- `internal/config/config.go` — Config struct definitions for form schemas
- `internal/models/models.go` — ModelConfig, ProviderFile, ModelDef structs
- `internal/plugins/interface.go` — Plugin interface and PluginInfo struct
- `configs/config.yaml` — Main gateway config (view-only reference)
- `configs/models/` — Model config YAML directory (managed by TUI)
- `configs/plugins/` — Plugin config YAML directory (managed by TUI)
- `cmd/cli/main.go` — Existing CLI tool (TUI replaces/extends this)

### Bubble Tea Framework
- https://github.com/charmbracelet/bubbletea — Bubble Tea core
- https://github.com/charmbracelet/bubbles — Bubbles component library
- https://github.com/charmbracelet/lipgloss — Lipgloss styling library

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **admin API endpoints**: `/admin/node/status` (GET, X-Admin-Token header), `/admin/node/restart` (POST), `/admin/plugins` (GET), `/admin/plugins/:protocol/reload` (POST), `/admin/plugins/:protocol` (DELETE), `/health` (GET) — all available for TUI to call
- **Config loading functions**: `config.Load()`, `config.LoadPluginConfigs()` in `internal/config/config.go` — TUI can reuse for validation
- **Model config types**: `models.ModelConfig`, `models.ProviderFile`, `models.ModelDef` — form schemas can be derived from these structs

### Established Patterns
- **Go project layout**: `cmd/` for binary entry points, `internal/` for shared code
- **Admin auth**: `X-Admin-Token` header pattern (from middleware/admin_token.go)
- **Config loading**: Viper-based with env var override (prefix `AI_GATEWAY`)
- **YAML model config**: Two formats — provider file (protocol + base_url + models[]) and single model file (meta.protocol + conn_config)

### Integration Points
- TUI → gateway communication: HTTP calls to `localhost:{port}` (configurable)
- TUI → filesystem: read/write YAML files in `configs/models/` and `configs/plugins/`
- TUI → gateway process: `os/exec` for start, `os.FindProcess` + `Signal(SIGTERM)` for stop
- No new gateway endpoints needed — TUI uses existing admin API surface

</code_context>

<specifics>
## Specific Ideas

- The TUI should feel like an operations dashboard — similar to `htop` or `k9s` in spirit
- Status bar at bottom showing: gateway status dot, uptime, connection status to gw API
- Provider form must include the two YAML formats the gateway supports: provider file (multi-model) and single model file
- Plugin tab should show: name, protocol, version, loaded status, healthy status (from PluginInfo struct)
- On TUI exit, the gateway process must keep running (detach child process)

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 09-tui-gateway-manager*
*Context gathered: 2026-05-25*

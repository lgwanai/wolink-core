# Phase 9: TUI Gateway Manager - Research

**Researched:** 2026-05-25
**Domain:** Terminal UI (TUI) with Bubble Tea v2, gateway process lifecycle management, form-based configuration editing
**Confidence:** MEDIUM

## Summary

This phase builds a standalone TUI binary (`cmd/tui/`) that manages the wolink gateway lifecycle, configuration, plugins, and status monitoring through an interactive terminal dashboard. The TUI runs as a separate process from the gateway -- exiting the TUI does not stop the running gateway service.

**Primary recommendation:** Use Charm's Bubble Tea v2 ecosystem (`charm.land/bubbletea/v2`, `charm.land/bubbles/v2`, `charm.land/lipgloss/v2`) which provides the Elm Architecture pattern, rich component library, and terminal-aware styling. This is a fresh binary with no existing dependencies, and v2 is the current stable line as of early 2026.

**Critical version finding:** All Charm libraries have moved to v2 with new module paths (`charm.land/.../v2`). This changes:
- Lipgloss v2 removes `AdaptiveColor` -- must use `lipgloss.LightDark` + `tea.RequestBackgroundColor` [CITED: charmbracelet/lipgloss v2.0.0 release notes]
- Bubbles v2 uses functional options (`New(WithWidth(80))`) and `NewDefaultStyles(isDark bool)` instead of exported fields [CITED: charmbracelet/bubbles v2.0.0 release notes]
- Bubble Tea v2 changes `Init()` to return only a command and uses `tea.NewView(...)` instead of raw string returns [CITED: charmbracelet/bubbletea v2.0.0 release notes]

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### TUI Framework
- **D-01:** Use Bubble Tea + Bubbles (Elm Architecture) as the TUI framework.
- **D-02:** TUI code lives in `cmd/tui/` as a separate binary entry point, distinct from `cmd/main.go` (gateway) and `cmd/cli/main.go` (CLI tool).
- **D-03:** Dashboard style with live panels -- multi-pane layout with status bar and live-updating metrics using lipgloss for styling.
- **D-04:** Vim-style keyboard navigation with arrow key fallback -- j/k for up/down, tab/h/l for left/right, / for search, q/esc for back.

#### Gateway Process Lifecycle
- **D-05:** TUI spawns gateway as a child process via os/exec, monitors via PID + periodic `/health` API polling. TUI can start/stop/restart. If TUI exits, gateway continues running (separate process group).
- **D-06:** Build gateway binary from source (`go build ./cmd/`) plus support a configurable binary path via env var for pre-built binaries.
- **D-07:** Graceful restart: send SIGTERM, wait for graceful shutdown with configurable timeout, then start new process with same config. Show restart progress in TUI.
- **D-08:** Live polling with status indicator: poll `/health` and `/admin/node/status` every N seconds (configurable, default 3s). Colored status dot + uptime + goroutines + memory in status bar.

#### Config & Provider Editing UX
- **D-09:** Form-based config editing with real-time field validation and save-time YAML parse check.
- **D-10:** Guided wizard for adding new providers/models: select protocol, base URL, API key, then add individual models. Generates valid provider YAML on save.
- **D-11:** Real-time field validation (e.g., port range, URL format) plus save-time complete YAML validation. Warn if gateway is running (restart needed for changes to take effect).
- **D-12:** TUI manages models config (`configs/models/*.yaml`) and plugins config (`configs/plugins/*.yaml`). Main `config.yaml` is view-only with a pointer to edit manually (rarely changed).

#### Navigation & Layout
- **D-13:** Tab bar navigation at top -- horizontal tabs switch between full-screen views.
- **D-14:** Three tabs: Dashboard, Providers, Plugins.
- **D-15:** Auto-detect terminal theme colors via lipgloss adaptive colors. Status colors: green=ok, yellow=warning, red=error, blue=info.
- **D-16:** Plugin enable/disable via gateway admin API -- calls `POST /admin/plugins/:protocol/reload` and `DELETE /admin/plugins/:protocol`. Status refreshes after each action.

### Claude's Discretion
- Specific Bubble Tea component choices (which Bubbles components for each view)
- Form field definitions and validation rules (derived from existing config structs)
- Polling interval default and configuration mechanism
- Error handling patterns for API call failures in TUI
- TUI binary build configuration (Makefile or go build integration)

### Deferred Ideas (OUT OF SCOPE)
None.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| TUI-01 | Gateway lifecycle management (start/stop/restart) via TUI | os/exec with Setsid+Setpgid for process detachment; SIGTERM for graceful shutdown; Process.Release() to avoid zombie |
| TUI-02 | Provider and model configuration editing via forms | Bubbles textinput + list components; ProviderFile and ModelConfigFile YAML structs from internal/models |
| TUI-03 | Live status monitoring with health polling | /admin/node/status returns NodeStatus JSON; /health returns status=ok; periodic tea.Cmd for polling |
| TUI-04 | Plugin enable/disable via admin API | POST /admin/plugins/:protocol/reload and DELETE /admin/plugins/:protocol from existing routes.go |
| TUI-05 | TUI runs as independent process, exit does not affect gateway | Setsid=true+Setpgid=true in SysProcAttr; Process.Release() after start; separate process group |

</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Gateway lifecycle management | TUI (Client/Orchestrator) | -- | os/exec spawn/kill; the TUI is the orchestrator, not part of the gateway |
| Configuration file management | TUI (Client/Orchestrator) | Storage (filesystem) | TUI reads/writes YAML files in configs/models/ and configs/plugins/ |
| Provider/model editing forms | TUI (Client/Orchestrator) | -- | All form logic lives in the TUI binary; structs from internal/models/ define the schema |
| Plugin enable/disable | TUI (Client/Orchestrator) | API/Backend (gateway) | TUI calls existing admin API endpoints; gateway owns the plugin runtime state |
| Health and status monitoring | TUI (Client/Orchestrator) | API/Backend (gateway) | TUI polls /health and /admin/node/status; periodic commands via Bubble Tea ticker |
| Terminal theme detection | TUI (Client/Orchestrator) | -- | lipgloss.LightDark + tea.RequestBackgroundColor; pure terminal capability |

**Key insight:** All TUI logic lives in the orchestrator tier. The gateway provides the API surface. There is no middleware tier -- the TUI communicates directly with the gateway's existing HTTP API.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `charm.land/bubbletea/v2` | v2.0.x | TUI framework (Elm Architecture: Model-Update-View) | Largest Go TUI ecosystem, active maintainer (Charm) |
| `charm.land/bubbles/v2` | v2.0.x | Pre-built UI components (textinput, list, table, viewport, spinner, help) | Official companion library from the same maintainer |
| `charm.land/lipgloss/v2` | v2.0.x | Terminal styling (colors, layout, borders) | Standard Charm styling library, required by Bubbles |

**Note on v2 module paths:** Bubble Tea v2 uses `charm.land/bubbletea/v2` instead of the old `github.com/charmbracelet/bubbletea`. Same pattern for Bubbles and Lipgloss. This is a module path change, not a fork -- still maintained by the Charm team. [CITED: charmbracelet/bubbletea v2.0.0 release notes]

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `gopkg.in/yaml.v3` | v3.0.1 (already in go.mod) | YAML serialization/deserialization for reading/writing config files | Saving model configs, plugin configs, validating YAML on save |
| `net/http` (stdlib) | Go 1.25 | HTTP client for admin API calls to gateway | All TUI-to-gateway communication (health, status, plugins) |
| `os/exec` (stdlib) | Go 1.25 | Gateway process spawn and management | Starting/stopping/restarting gateway process |
| `github.com/spf13/viper` | v1.16.0 (already in go.mod) | Config loading for TUI's own settings | TUI could reuse viper for its own polling interval config |

### Version Verification
All v2 Charm packages are `[ASSUMED]` -- slopcheck was unavailable at research time, and the exact module paths (`charm.land/*/v2`) were confirmed via web search of official Charm release notes but not directly via Go module proxy verification.

## Package Legitimacy Audit

> slopcheck was unavailable at research time (python3 install failed, not in PATH). All packages below are tagged [ASSUMED]. The planner MUST gate each install behind a `checkpoint:human-verify` task.

**Packages to add to go.mod:**

| Package | Version Claim | Source | slopcheck | Disposition |
|---------|--------------|--------|-----------|-------------|
| `charm.land/bubbletea/v2` | v2.0.x | [ASSUMED] - web search of official releases | N/A | Flagged -- planner must add checkpoint |
| `charm.land/bubbles/v2` | v2.0.x | [ASSUMED] - web search of official releases | N/A | Flagged -- planner must add checkpoint |
| `charm.land/lipgloss/v2` | v2.0.x | [ASSUMED] - web search of official releases | N/A | Flagged -- planner must add checkpoint |

**Installation commands (for human verification):**
```bash
cd /Users/wuliang/workspace/wolink-core
go get charm.land/bubbletea/v2@latest
go get charm.land/bubbles/v2@latest
go get charm.land/lipgloss/v2@latest
```

## Architecture Patterns

### Bubble Tea v2 Pattern (Elm Architecture)

Bubble Tea follows the Elm Architecture: a single `Model` holds state, `Update` handles messages and returns new state (plus optional commands), `View` renders the screen.

**v2 key changes from v1:**
- `Init()` returns `tea.Cmd` only (no `tea.Model`) [CITED: v2.0.0 notes]
- `View()` uses `tea.NewView(...)` builder instead of raw string return [CITED: v2.0.0 notes]
- Background color detection: `tea.RequestBackgroundColor` message [CITED: lipgloss v2.0.0 notes]

```go
// Basic v2 pattern
type model struct {
    styles  styles
    tab     int // 0=Dashboard, 1=Providers, 2=Plugins
    width   int
    height  int
}

func (m model) Init() tea.Cmd {
    return tea.Batch(
        tea.RequestBackgroundColor,
        tea.WindowSize(),
        pollHealthCmd,  // start health polling
    )
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        return m, nil
    case tea.BackgroundColorMsg:
        m.styles = newStyles(msg.IsDark())
        return m, nil
    case tea.KeyMsg:
        switch msg.String() {
        case "tab", "l":
            m.tab = (m.tab + 1) % 3
        case "shift+tab", "h":
            m.tab = (m.tab - 1 + 3) % 3
        case "q", "esc":
            return m, tea.Quit
        }
    }
    return m, nil
}

func (m model) View() string {
    return tea.NewView(
        renderTabBar(m),
        renderContent(m),
        renderStatusBar(m),
    ).String()
}
```

### Recommended Project Structure
```
cmd/tui/
├── main.go                    # Entry point: flag parsing, env loading, program start
├── model.go                   # Top-level model, Init, Update, View
├── commands.go                # tea.Cmd factories (health poll, HTTP calls, process mgmt)
├── styles.go                  # Lipgloss style definitions (light/dark)
├── keymap.go                  # Keybinding definitions and help view
├── tabs/
│   ├── dashboard.go           # Dashboard tab: status panels, metrics, uptime
│   ├── providers.go           # Providers tab: list, add/edit/delete providers/models
│   └── plugins.go             # Plugins tab: list loaded plugins, enable/disable toggles
├── forms/
│   ├── provider_form.go       # Add/edit provider form (protocol, url, key, models)
│   ├── model_form.go          # Add/edit single model form
│   └── validation.go          # Field validation rules derived from config structs
├── gateway/
│   ├── lifecycle.go           # os/exec: build, start, stop, restart logic
│   └── client.go              # HTTP client: health, status, plugin API calls
└── config.go                  # TUI-specific config (polling interval, gateway URL, binary path)
```

### Pattern 1: Gateway Process Lifecycle Management
**What:** Start gateway as child, detach on TUI exit, graceful restart via SIGTERM.

**Key mechanics:**
- `os/exec.Cmd` with `SysProcAttr{Setsid: true, Setpgid: true}` creates a new process group -- prevents SIGHUP from reaching child when TUI exits [CITED: golang.org/pkg/syscall/#SysProcAttr]
- `cmd.Process.Release()` after start disconnects parent from child -- OS reaps the child via init [CITED: golang.org/pkg/os/#Process.Release]
- For graceful restart: send `SIGTERM`, poll `/health` until unavailable (shutdown timeout), then start new process

```go
// Source: derived from golang.org/pkg/os/exec and syscall docs
func (m *GatewayManager) StartGateway() error {
    cmd := exec.Command(m.binaryPath)
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Setsid:  true,   // new session, no controlling terminal
        Setpgid: true,   // new process group
    }
    cmd.Dir = m.workingDir
    cmd.Stdout = m.logWriter
    cmd.Stderr = m.logWriter

    if err := cmd.Start(); err != nil {
        return fmt.Errorf("gateway start failed: %w", err)
    }

    m.pid = cmd.Process.Pid

    // Release so child continues after TUI exits
    if err := cmd.Process.Release(); err != nil {
        return fmt.Errorf("process release failed: %w", err)
    }

    return nil
}
```

### Pattern 2: Live Polling with Periodic Commands
**What:** Use `tea.Every` (or `time.Tick`-based cmd) to poll gateway health on a timer.

```go
// Source: adapted from bubbletea v2 docs -- batch pattern
func pollCmd() tea.Cmd {
    return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
        return pollTickMsg(t)
    })
}

// In Update:
case pollTickMsg:
    m.status = m.gwClient.CheckHealth()
    return m, pollCmd() // re-schedule next tick

case tea.WindowSizeMsg:
    m.width = msg.Width
    m.height = msg.Height
    return m, nil
```

### Pattern 3: Bubbles v2 Functional Options Pattern
**What:** Bubbles v2 components use constructor functions with `With*` options instead of exported field assignment.

```go
// Source: charmbracelet/bubbles v2.0.0 README
import "charm.land/bubbles/v2/textinput"

ti := textinput.New(
    textinput.WithWidth(50),
    textinput.WithPrompt("Base URL: "),
)

// Getters/setters instead of exported fields
ti.SetValue("http://localhost:8080")
val := ti.Value()

// Get style variants for dark/light
import "charm.land/bubbles/v2/list"
listStyles := list.NewDefaultStyles(isDark)
```

### Pattern 4: Adaptive Colors with Lipgloss v2
**What:** Background color detection + lipgloss.LightDark for terminal-aware styling.

```go
// Source: charmbracelet/lipgloss v2.0.0 CHANGELOG
func newStyles(isDark bool) styles {
    lightDark := lipgloss.LightDark(isDark)
    return styles{
        statusGreen: lipgloss.NewStyle().
            Foreground(lightDark("#00aa00", "#00ff00")),
        statusRed: lipgloss.NewStyle().
            Foreground(lightDark("#aa0000", "#ff0000")),
        tabActive: lipgloss.NewStyle().
            Background(lightDark("#cccccc", "#333333")).
            Foreground(lightDark("#000000", "#ffffff")),
    }
}
```

### Anti-Patterns to Avoid
- **Global state:** Do not use package-level vars for model state. Bubble Tea expects state in the Model struct.
- **Blocking HTTP calls in View():** View() must be pure/synchronous. All HTTP calls go in Update() via tea.Cmd.
- **Ignoring terminal resize:** Always handle `tea.WindowSizeMsg` to adapt layout to terminal dimensions.
- **Using exported Bubbles fields (v1 pattern):** v2 uses getters/setters and functional options. The old exported-field pattern will not compile.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| TUI framework | Custom terminal event loop | Bubble Tea (charm.land/bubbletea/v2) | Signal handling, terminal resize, mouse support, raw mode -- all battle-tested |
| TUI components | Custom list, text input, table | Bubbles (charm.land/bubbles/v2) | textinput, list, table, viewport, spinner, help -- all pre-built and composable |
| Terminal styling | ANSI escape code concatenation | Lipgloss (charm.land/lipgloss/v2) | Color profile detection, adaptive colors, layout padding/border/margin, cross-platform |
| YAML reading/writing | Custom YAML parser | gopkg.in/yaml.v3 (already in go.mod) | Already used by gateway config system; struct tags match existing ProviderFile/ModelConfigFile types |
| HTTP client foundation | Custom transport layer | net/http (stdlib) | Simple REST calls to localhost gateway; no need for advanced clients |

**Key insight:** The Bubble Tea ecosystem eliminates the two hardest parts of building a TUI: event loop correctness (raw terminal mode, signal handling) and input component state management (cursor position, clipboard, scrolling). Attempting these with raw terminal escape codes is a significant source of bugs in terminal applications.

## Common Pitfalls

### Pitfall 1: Gateway Process Becomes Zombie
**What goes wrong:** TUI starts gateway via os/exec, TUI exits, gateway keeps running but becomes a zombie process (no parent reaping).
**Why it happens:** Without `Process.Release()`, the parent retains a reference. When parent exits, child becomes orphan but may not be properly reaped by init on all systems.
**How to avoid:** Always call `cmd.Process.Release()` after `cmd.Start()` succeeds. The init process (PID 1) will reap orphaned children.
**Warning signs:** `ps aux` shows gateway process with status "Z" (zombie).

### Pitfall 2: SIGHUP Kills Gateway on TUI Exit
**What goes wrong:** When TUI terminal closes, the gateway child process receives SIGHUP and dies.
**Why it happens:** By default, child processes inherit the parent's controlling terminal. SIGHUP is sent to all processes in the terminal session when the terminal closes.
**How to avoid:** Use `SysProcAttr{Setsid: true}` to make the child a session leader with no controlling terminal [VERIFIED: golang.org/pkg/syscall/#SysProcAttr]. Also use `Setpgid: true` for a new process group.
**Warning signs:** Gateway stops when TUI window is closed.

### Pitfall 3: Blocking HTTP Calls Freeze the TUI
**What goes wrong:** A long HTTP request to the gateway (e.g., timeout) blocks the entire TUI, making it unresponsive.
**Why it happens:** If HTTP calls are made synchronously in Update() or (worse) View(), they block the Bubble Tea event loop.
**How to avoid:** Wrap all HTTP calls in `tea.Cmd` functions. Use `http.Client` with a reasonable timeout (e.g., 5s for health polls). Set `http.DefaultClient.Timeout` (as existing CLI does in cmd/cli/main.go init()).
**Warning signs:** TUI freezes when gateway is unreachable or slow.

### Pitfall 4: Terminal Not in Raw Mode When Exiting
**What goes wrong:** TUI crashes or exits without restoring terminal to cooked mode, leaving the terminal in a broken state.
**Why it happens:** Bubble Tea enters raw mode (no echo, no line buffering) on start. If the program crashes before cleanup, the terminal stays in raw mode.
**How to avoid:** Bubble Tea's `tea.Quit` command handles this. Wrap risky operations in deferred recovery. Always use `tea.Program.Run()` which guarantees cleanup. If using `tea.Program.Start()` (v1), the problem is more common -- v2's `Run()` is safer.

### Pitfall 5: Bubbles v1 API Usage in v2
**What goes wrong:** Code that assigns to exported struct fields (like `m.list.Title = "Items"`) silently fails or panics.
**Why it happens:** Bubbles v2 switched from exported fields to getters/setters (`m.list.SetTitle("Items")`) and functional options (`list.New(list.WithTitle("Items"))`).
**How to avoid:** Always use the v2 API. Constructor pattern: `component.New(component.WithX(val))`. Mutation pattern: `component.SetX(val)`. Check the v2 docs before coding.

## Code Examples

### Verified Patterns from Official Sources

### Common Operation 1: Bubble Tea v2 Tab Switching

```go
// Source: adapted from bubbletea v2 README examples
type activeTab int

const (
    tabDashboard activeTab = iota
    tabProviders
    tabPlugins
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "tab", "l":
            m.activeTab = (m.activeTab + 1) % 3
            return m, nil
        case "shift+tab", "h":
            m.activeTab = (m.activeTab - 1 + 3) % 3
            return m, nil
        case "1":
            m.activeTab = tabDashboard
        case "2":
            m.activeTab = tabProviders
        case "3":
            m.activeTab = tabPlugins
        }
    }
    return m, nil
}
```

### Common Operation 2: Bubbles v2 Table Component

```go
// Source: derived from charmbracelet/bubbles table component v2 docs
import "charm.land/bubbles/v2/table"

func newPluginsTable(isDark bool) table.Model {
    columns := []table.Column{
        {Title: "Name", Width: 20},
        {Title: "Protocol", Width: 12},
        {Title: "Version", Width: 10},
        {Title: "Loaded", Width: 8},
        {Title: "Healthy", Width: 8},
    }

    t := table.New(
        table.WithColumns(columns),
        table.WithRows([]table.Row{}),
        table.WithFocused(true),
        table.WithHeight(10),
    )

    s := table.DefaultStyles(isDark)
    s.Header = s.Header.
        BorderStyle(lipgloss.NormalBorder()).
        BorderForeground(lipgloss.Color("240"))
    s.Selected = s.Selected.
        Foreground(lipgloss.Color("229")).
        Background(lipgloss.Color("57"))
    t.SetStyles(s)

    return t
}
```

### Common Operation 3: Admin API HTTP Call via tea.Cmd

```go
// Source: derived from cmd/cli/main.go HTTP pattern + bubbletea v2 cmd pattern
func fetchNodeStatusCmd(gwURL, adminToken string) tea.Cmd {
    return func() tea.Msg {
        req, _ := http.NewRequest("GET", gwURL+"/admin/node/status", nil)
        req.Header.Set("X-Admin-Token", adminToken)

        client := &http.Client{Timeout: 5 * time.Second}
        resp, err := client.Do(req)
        if err != nil {
            return statusErrorMsg{err: err}
        }
        defer resp.Body.Close()

        var result struct {
            Data services.NodeStatus `json:"data"`
        }
        if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
            return statusErrorMsg{err: err}
        }
        return statusUpdateMsg{status: result.Data}
    }
}
```

### Common Operation 4: Form-Based Provider Add with Validation

```go
// Source: derived from config structs in internal/models/models.go
// and bubbles textinput component (v2)

type providerFormModel struct {
    protocol textinput.Model  // WithPrompt("Protocol: ") -- "openai", "deepseek", etc.
    baseURL  textinput.Model  // WithPrompt("Base URL: ")
    apiKey   textinput.Model  // WithPrompt("API Key: "), WithEchoMode(textinput.EchoPassword)
    focus    int              // 0=protocol, 1=baseURL, 2=apiKey
    done     bool
}

func newProviderForm() providerFormModel {
    protocol := textinput.New(
        textinput.WithWidth(50),
        textinput.WithPrompt("Protocol (openai/deepseek/claude): "),
    )
    baseURL := textinput.New(
        textinput.WithWidth(50),
        textinput.WithPrompt("Base URL: "),
    )
    apiKey := textinput.New(
        textinput.WithWidth(60),
        textinput.WithPrompt("API Key: "),
        textinput.WithEchoMode(textinput.EchoPassword),
    )
    return providerFormModel{
        protocol: protocol,
        baseURL:  baseURL,
        apiKey:   apiKey,
    }
}

// Validation on save
func (f providerFormModel) validate() []string {
    var errs []string
    if f.protocol.Value() == "" {
        errs = append(errs, "Protocol is required")
    }
    if f.baseURL.Value() == "" {
        errs = append(errs, "Base URL is required")
    } else if !strings.HasPrefix(f.baseURL.Value(), "http") {
        errs = append(errs, "Base URL must start with http:// or https://")
    }
    if f.apiKey.Value() == "" {
        errs = append(errs, "API Key is required")
    }
    return errs
}
```

### Common Operation 5: YAML Config Save with Struct Tags

```go
// Source: derived from ProviderFile struct in internal/models/models.go
// and existing yaml.v3 usage pattern

import "gopkg.in/yaml.v3"

func (f providerFormModel) toProviderFile() *models.ProviderFile {
    return &models.ProviderFile{
        ID:       generateID(f.name.Value()),
        Name:     f.name.Value(),
        Protocol: f.protocol.Value(),
        BaseURL:  f.baseURL.Value(),
        APIKey:   f.apiKey.Value(),
        Models:   f.collectedModels,
    }
}

func saveProviderFile(pf *models.ProviderFile, path string) error {
    data, err := yaml.Marshal(pf)
    if err != nil {
        return fmt.Errorf("yaml marshal: %w", err)
    }

    // Validate by parsing back
    var check models.ProviderFile
    if err := yaml.Unmarshal(data, &check); err != nil {
        return fmt.Errorf("save-time yaml validation failed: %w", err)
    }

    return os.WriteFile(path, data, 0644)
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Bubble Tea v1 (`github.com/charmbracelet/bubbletea`) | Bubble Tea v2 (`charm.land/bubbletea/v2`) | Early 2026 | New module path, Init/View changes, no global renderer state |
| Bubbles exported struct fields | Bubbles getters/setters + functional options | Feb 2026 | All v1 code assigning to fields will not compile |
| lipgloss.AdaptiveColor | lipgloss.LightDark + tea.RequestBackgroundColor | Mar 2026 | Need to handle background color message in Update |
| Global lipgloss renderer | Per-style explicit rendering | v2.0.0 | No more renderer race conditions, but need explicit Println/Sprint |

**Deprecated/outdated:**
- `github.com/charmbracelet/bubbletea` (v1): Use `charm.land/bubbletea/v2` instead
- `github.com/charmbracelet/bubbles` (v1): Use `charm.land/bubbles/v2` instead
- `github.com/charmbracelet/lipgloss` (v1): Use `charm.land/lipgloss/v2` instead
- `lipgloss.AdaptiveColor`: Replaced by `compat.AdaptiveColor` or `lipgloss.LightDark`
- Bubbles `NewModel` constructors: Use `New(WithOptions...)` instead

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Charm v2 module paths are `charm.land/bubbletea/v2`, `charm.land/bubbles/v2`, `charm.land/lipgloss/v2` | Standard Stack | Go build fails -- planner uses wrong import path. Mitigation: checkpoint for human to verify `go get` succeeds |
| A2 | Bubbles v2 table, textinput, list, viewport, spinner, help components exist and work as documented | Architecture Patterns | Planner designs using wrong component APIs. Mitigation: verified via web search of official v2.0.0 release notes |
| A3 | SysProcAttr{Setsid: true, Setpgid: true} detaches child from terminal on macOS (Darwin) and Linux | Architectural Patterns | Gateway dies on TUI exit on macOS. Mitigation: Setsid is supported on both Linux and macOS (Darwin) per syscall docs |
| A4 | `Process.Release()` prevents zombie processes | Common Pitfalls | Gateway becomes zombie on early TUI crash. Mitigation: Process.Release is well-documented in Go stdlib |

## Open Questions [RESOLVED]

1. **Bubble Tea v2 exact version compatibility** [RESOLVED]
   - What we know: v2.0.x is stable as of early 2026
   - What's unclear: The exact latest patch version (2.0.6 for bubbletea, 2.0.2 for lipgloss, 2.1.0 for bubbles per web search)
   - Resolution: Run `go get charm.land/bubbletea/v2@latest` at build time (Plan 01, Task 0). Exact version resolved at install time; import paths are stable regardless of patch version.

2. **Tab bar rendering approach** [RESOLVED]
   - What we know: Bubbles does not provide a built-in tab bar component
   - What's unclear: Whether to build tabs with lipgloss inline styling, use a third-party component like `stickers` (flexbox layouts), or reuse the `list` component styled as tabs
   - Resolution: Build tab bar with lipgloss inline (row of styled blocks with active/inactive colors). No third-party dependencies. This matches the Charm team's own example patterns. See Plan 02, model.go helper functions.

3. **Config file change detection for running gateway** [RESOLVED]
   - What we know: Gateway reads config files at startup and watches for changes via fsnotify (implied by PluginService watching plugin source files)
   - What's unclear: Whether the gateway detects config file changes at runtime or requires restart. The decision says "warn if gateway is running" so the planner should assume restart is required.
   - Resolution: TUI shows "restart required" indicator after config changes while gateway is running (per D-11). No file change polling in TUI -- too complex and out of scope. Gateway restart handled via Ctrl+R on Dashboard tab.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| go toolchain | Building cmd/tui/ binary | not tested on this machine | go.mod says go 1.25 | -- |
| Charm bubbletea v2 | TUI framework | Not installed yet | v2.0.x (ASSUMED) | -- |
| Charm bubbles v2 | TUI components | Not installed yet | v2.0.x (ASSUMED) | -- |
| Charm lipgloss v2 | Terminal styling | Not installed yet | v2.0.x (ASSUMED) | -- |
| gopkg.in/yaml.v3 | YAML config read/write | Already in go.mod | v3.0.1 | -- |
| Running gateway instance | TUI to manage | Not running | -- | TUI can start it |

**Missing dependencies with no fallback:**
- Go toolchain (go 1.25) -- must be installed to build the TUI binary. The project go.mod already specifies go 1.25.0.
- Charm v2 libraries -- must be added to go.mod via `go get`

## Validation Architecture

> nyquist_validation enabled in config.json.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify (github.com/stretchr/testify) |
| Config file | None needed -- standard Go _test.go pattern |
| Quick run command | `go test ./cmd/tui/... -count=1 -short` |
| Full suite command | `go test ./cmd/tui/... -count=1` |

### Bubble Tea Testing Pattern
Bubble Tea models are testable by sending messages directly to `Update()` and asserting on the returned model state:

```go
// Testing pattern for Bubble Tea models
func TestDashboardUpdate(t *testing.T) {
    m := newModel()
    result, _ := m.Update(statusUpdateMsg{
        status: services.NodeStatus{
            Status:  "healthy",
            Uptime:  "5m",
            Version: "v1.0.0",
        },
    })
    updated := result.(model)
    assert.Equal(t, "healthy", updated.gwStatus)
}
```

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | Existing? |
|--------|----------|-----------|-------------------|-----------|
| TUI-01 | Gateway starts via os/exec | unit | `go test ./cmd/tui/gateway/... -run TestStartGateway` | No -- new |
| TUI-01 | Gateway stops via SIGTERM | unit | `go test ./cmd/tui/gateway/... -run TestStopGateway` | No -- new |
| TUI-01 | Gateway continues after TUI exit (process release) | unit | `go test ./cmd/tui/gateway/... -run TestProcessRelease` | No -- new |
| TUI-02 | Form validates URL format | unit | `go test ./cmd/tui/forms/... -run TestURLValidation` | No -- new |
| TUI-02 | Form validates required fields | unit | `go test ./cmd/tui/forms/... -run TestRequiredFields` | No -- new |
| TUI-02 | Save produces valid YAML | unit | `go test ./cmd/tui/forms/... -run TestYAMLSave` | No -- new |
| TUI-03 | Health polling cmd returns correct type | unit | `go test ./cmd/tui/... -run TestHealthPollCmd` | No -- new |
| TUI-03 | Status message updates model | unit | `go test ./cmd/tui/... -run TestStatusUpdate` | No -- new |
| TUI-03 | Ctrl+S starts gateway (lifecycle) | unit | `go test ./cmd/tui/... -run TestLifecycleStart` | No -- new |
| TUI-03 | Ctrl+R triggers restart with progress | unit | `go test ./cmd/tui/... -run TestLifecycleRestartProgress` | No -- new |
| TUI-04 | Plugin reload calls correct endpoint | unit | `go test ./cmd/tui/... -run TestPluginReload` | No -- new |
| TUI-04 | Plugin unload calls correct endpoint | unit | `go test ./cmd/tui/... -run TestPluginUnload` | No -- new |
| TUI-05 | Process starts with Setsid+Setpgid | unit | `go test ./cmd/tui/gateway/... -run TestStartWithGroup` | No -- new |
| TUI-05 | Process survives parent exit | integration | Manual E2E test | No -- new |

### Sampling Rate
- **Per task commit:** `go build ./cmd/tui/...` (verify it compiles)
- **Per wave merge:** `go test ./cmd/tui/... -count=1`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `cmd/tui/gateway/lifecycle_test.go` -- gateway process lifecycle tests
- [ ] `cmd/tui/forms/validation_test.go` -- form validation and YAML save tests
- [ ] `cmd/tui/model_test.go` -- top-level model Update tests for message handling
- [ ] Test fixtures: sample config YAML files in testdata/ directory

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | TUI reads `admin.token` from config to authenticate admin API calls |
| V3 Session Management | no | TUI is a local process, no sessions |
| V4 Access Control | yes | TUI uses X-Admin-Token header for admin API access |
| V5 Input Validation | yes | Form field validation (URL format, port range, required fields) |
| V6 Cryptography | no | Admin token is transmitted plaintext over localhost HTTP (acceptable) |

### Known Threat Patterns for Local TUI Process

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Admin token in process env/memory | Information disclosure | Token loaded from config only; not printed or exposed in TUI output |
| Config file permissions | Tampering | TUI respects filesystem permissions (same user running TUI has access to config dirs) |
| Localhost API access | Spoofing | The admin API is bound to localhost by gateway config; no external network exposure |
| Gateway startup args injection | Tampering | Binary path from known location or env var; no argument injection vector via os/exec |
| Sensitive data in form fields (API keys) | Information disclosure | Password echo mode on API key input fields |

## Sources

### Primary (MEDIUM confidence)
- [Official charmbracelet/bubbletea v2 release notes](https://github.com/charmbracelet/bubbletea/releases) -- v2.0.x module path and API changes (confirmed via web search)
- [Official charmbracelet/bubbles v2 release notes](https://github.com/charmbracelet/bubbles/releases/tag/v2.0.0) -- component list, functional options, NewDefaultStyles (confirmed via web search)
- [Official charmbracelet/lipgloss v2 release notes](https://github.com/charmbracelet/lipgloss/releases/tag/v2.0.0) -- AdaptiveColor removal, LightDark helper, compat package (confirmed via web search)

### Secondary (HIGH confidence)
- [Golang os/exec docs](https://pkg.go.dev/os/exec) -- Command, Start, Process.Release
- [Golang syscall docs](https://pkg.go.dev/syscall#SysProcAttr) -- SysProcAttr.Setsid, Setpgid
- Project source: `internal/api/routes.go` -- all admin API endpoints
- Project source: `internal/api/handlers/plugin_handler.go` -- plugin API request/response formats
- Project source: `internal/services/node_service.go` -- NodeStatus struct fields
- Project source: `internal/models/models.go` -- ProviderFile, ModelConfigFile, ModelDef structs
- Project source: `internal/plugins/interface.go` -- PluginInfo struct fields
- Project source: `internal/config/config.go` -- Config, AdminConfig structs
- Project source: `internal/api/middleware/admin_token.go` -- X-Admin-Token auth pattern
- Project source: `cmd/cli/main.go` -- existing HTTP client pattern to reuse
- Project source: `go.mod` -- existing Go version (1.25.0) and dependencies

## Metadata

**Confidence breakdown:**
- Standard stack: LOW -- Charm v2 module paths not verified via Go module proxy; from web search of release notes only
- Architecture: HIGH -- Bubble Tea Elm Architecture is well-established and documented in official README
- Pitfalls: HIGH -- Process management, terminal handling, blocking call patterns are well-understood Go/systems topics

**Research date:** 2026-05-25
**Valid until:** 2026-06-25 (30 days -- Charm ecosystem is stable but fast-moving)

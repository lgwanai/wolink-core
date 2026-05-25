---
phase: 09-tui-gateway-manager
plan: 09-03
subsystem: tui
tags:
  - bubbletea
  - providers
  - yaml-crud
  - forms
  - validation
dependency_graph:
  requires:
    - 09-02 (model.go base structure, tab constants, gateway client/lifecycle)
  provides:
    - Provider form wizard and model form components
    - Provider list with YAML CRUD operations
    - Updated model with Providers tab wired into Update/View
  affects:
    - cmd/tui/providers.go (new)
    - cmd/tui/model.go (updated)
    - cmd/tui/forms/ (new package)
tech-stack:
  added:
    - cmd/tui/forms/ package with validation, provider_form, model_form modules
  patterns:
    - Bubbles v2 textinput with direct field assignment (not functional options)
    - Bubble Tea v2 KeyMsg interface with KeyPressMsg type assertions
    - Wizard-style form progression with step validation
    - YAML marshal + re-parse validation pattern
key-files:
  created:
    - cmd/tui/forms/validation.go (56 lines)
    - cmd/tui/forms/provider_form.go (270 lines)
    - cmd/tui/forms/model_form.go (202 lines)
    - cmd/tui/providers.go (285 lines)
    - cmd/tui/providers_test.go (177 lines)
  modified:
    - cmd/tui/model.go (provider fields + Update/View wiring)
decisions:
  - providers.go placed in cmd/tui/ (not cmd/tui/tabs/) because the unexported model type requires same-package access (environment note)
  - Textinput fields assigned directly via exported fields (Prompt, EchoMode) and SetWidth() setter; bubbles v2.1.0 does not expose functional options for textinput
  - provider list rendered with simple string builder instead of Bubbles table component for reduced complexity
  - shift+tab in form sub-state handled by providers handler instead of tab-switching (model.go special case)
metrics:
  duration: ~45 min
  completed_date: "2026-05-25"
---

# Phase 9 Plan 3: Providers Tab with Form-Based Config Editing

**One-liner:** Provider and model configuration management with guided wizard forms, real-time field validation, and YAML file CRUD operations, wired into the TUI as the second tab.

## Verification

- [x] `go build ./cmd/tui/...` compiles successfully
- [x] `go vet ./cmd/tui/...` passes
- [x] Provider-specific tests pass (5/5)
- [x] Provider tab lists providers from configs/models/*.yaml
- [x] Add/edit provider form progresses through all 6 steps
- [x] Save produces valid YAML that re-parses correctly (round-trip tests)
- [x] Delete removes the file from filesystem
- [x] Restart-required flag set after config changes when gateway is running

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Head on main branch after worktree reset**
- **Found during:** Task 1 pre-commit
- **Issue:** The initial `git reset --hard` switched worktree HEAD to `main` instead of the per-agent branch `worktree-agent-*`
- **Fix:** Re-pointed `refs/heads/worktree-agent-*` to current HEAD and updated `HEAD` symbolic-ref
- **Files modified:** None (git refs only)

**2. [Rule 3 - Blocking] Bubbles v2.1.0 textinput uses unexported `width` field**
- **Found during:** Task 1 build
- **Issue:** `ti.Width = 50` fails because `Width` is unexported as `width` in v2.1.0; `SetWidth()` setter must be used instead
- **Fix:** Changed `ti.Width = w` to `ti.SetWidth(w)` in `newTextInput()` and `newPasswordInput()`
- **Files modified:** `cmd/tui/forms/provider_form.go`

**3. [Environment note] File path difference: `tabs/providers.go` -> `providers.go`**
- **Found during:** Planning
- **Issue:** Plan specifies `cmd/tui/tabs/providers.go` but the unexported `model` type requires `package main`, which is only accessible in `cmd/tui/` directory
- **Fixed by:** Placing providers.go in `cmd/tui/` (package main) instead of `cmd/tui/tabs/`
- **Files modified:** N/A — path chosen per environment note

## TDD Gate Compliance

- RED gate commit: `ed9ca0d test(09-03): add failing tests for providers tab functions`
- GREEN gate commit: `aca14ad feat(09-03): implement providers tab with YAML CRUD and wire into model`
- REFACTOR gate: Not needed — GREEN phase passed directly

## Known Stubs

None. All planned functionality is implemented:

- **Provider form wizard**: 6-step (name, protocol, base URL, API key, description, confirm)
- **Model form**: 6-field (ID, display name, upstream model name, type, mode, route)
- **Provider list**: Loads from disk, shows providers and single models
- **CRUD operations**: list, save (provider + single model), delete
- **Validation**: Protocol, URL, port, required fields, model ID, YAML parse check

## Commits

| Hash | Message |
|------|---------|
| a39e47d | feat(09-03): create form validation rules and provider/model form components |
| ed9ca0d | test(09-03): add failing tests for providers tab functions |
| aca14ad | feat(09-03): implement providers tab with YAML CRUD and wire into model |

## Self-Check: PASSED

All created files verified:
- [x] cmd/tui/forms/validation.go exists
- [x] cmd/tui/forms/provider_form.go exists
- [x] cmd/tui/forms/model_form.go exists
- [x] cmd/tui/providers.go exists
- [x] cmd/tui/providers_test.go exists
- [x] All 3 commits found in git log
- [x] `go build ./cmd/tui/...` succeeds
- [x] `go vet ./cmd/tui/...` passes

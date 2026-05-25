package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"wolink-core/cmd/tui/forms"
	"wolink-core/cmd/tui/gateway"
	"wolink-core/internal/services"
)

// ---------------------------------------------------------------------------
// Message types
// ---------------------------------------------------------------------------

// pollTickMsg is sent by the polling timer to trigger health+status polling.
type pollTickMsg struct {
	time.Time
}

// healthMsg carries the result of a /health API call.
type healthMsg struct {
	ok         bool
	statusText string
	err        error
}

// statusUpdateMsg carries the result of a /admin/node/status API call.
type statusUpdateMsg struct {
	status *services.NodeStatus
	err    error
}

// errMsg carries a polling or general error to display in the status bar.
type errMsg struct {
	err error
}

// startGatewayMsg is sent after a gateway Start() call completes.
type startGatewayMsg struct {
	err error
}

// stopGatewayMsg is sent after a gateway Stop() call completes.
type stopGatewayMsg struct {
	err error
}

// restartProgressMsg is sent during restart to update the step text.
type restartProgressMsg struct {
	step string
}

// restartCompleteMsg is sent after restart finishes (success or error).
type restartCompleteMsg struct {
	err error
}

// ---------------------------------------------------------------------------
// Screen enumeration
// ---------------------------------------------------------------------------

type screen int

const (
	screenHome screen = iota
	screenDashboard
	screenProviders
	screenPlugins
)

const numHomeItems = 3

// ---------------------------------------------------------------------------
// Model
// ---------------------------------------------------------------------------

// model is the top-level Bubble Tea model for the TUI application.
type model struct {
	cfg         *TUIConfig
	gwClient    *gateway.Client
	gwLifecycle *gateway.Lifecycle
	styles      styles
	width       int
	height      int

	// Navigation
	currentScreen screen
	homeCursor    int // 0=Dashboard, 1=Providers, 2=Plugins
	contentCursor int // cursor within content (list items, action buttons)

	// Gateway state
	gwStatus       string // "healthy", "degraded", "down", "unknown"
	healthText     string
	nodeStatus     *services.NodeStatus
	statusErr      string
	polling        bool
	darkTheme      bool
	ready          bool
	lifecycleState string // "idle", "starting", "stopping", "restarting"
	lifecycleErr   string
	lifecycleStep  string

	// Providers tab state
	providersState      providersTabState
	providerListItems   []providerListItem
	singleModelItems    []singleModelItem
	providerForm        forms.ProviderFormModel
	modelForm           forms.ModelFormModel
	selectedProviderIdx int
	selectedModelIdx    int
	restartRequired     bool

	// Plugins tab state
	pluginsState      pluginsTabState
	pluginListItems   []pluginListItem
	selectedPluginIdx int
	pluginErr         string
}

func newModel(cfg TUIConfig, client *gateway.Client, lifecycle *gateway.Lifecycle) model {
	return model{
		cfg:            &cfg,
		gwClient:       client,
		gwLifecycle:    lifecycle,
		gwStatus:       "unknown",
		lifecycleState: "idle",
		pluginsState:   pluginsStateIdle,
		currentScreen:  screenHome,
	}
}

// ---------------------------------------------------------------------------
// Init
// ---------------------------------------------------------------------------

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tea.RequestBackgroundColor,
		pollCmd(m.cfg.PollInterval),
	)
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case tea.BackgroundColorMsg:
		m.styles = newStyles(msg.IsDark())
		m.darkTheme = msg.IsDark()
		return m, nil

	case pollTickMsg:
		return m, tea.Batch(
			pollHealthCmd(m.gwClient),
			fetchStatusCmd(m.gwClient),
			pollCmd(m.cfg.PollInterval),
		)

	case healthMsg:
		if msg.err != nil {
			m.gwStatus = "down"
			m.healthText = "Gateway not reachable"
		} else if msg.ok {
			m.gwStatus = "healthy"
			if msg.statusText != "" {
				m.healthText = msg.statusText
			} else {
				m.healthText = "Gateway is healthy"
			}
		} else {
			m.gwStatus = "degraded"
			m.healthText = msg.statusText
		}
		return m, nil

	case statusUpdateMsg:
		if msg.err != nil {
			m.nodeStatus = nil
			m.statusErr = ""
		} else {
			m.nodeStatus = msg.status
			m.statusErr = ""
		}
		return m, nil

	case errMsg:
		m.statusErr = msg.err.Error()
		return m, nil

	case startGatewayMsg:
		if msg.err != nil {
			m.lifecycleErr = msg.err.Error()
			m.lifecycleState = "idle"
		} else {
			m.lifecycleState = "running"
		}
		return m, nil

	case stopGatewayMsg:
		if msg.err != nil {
			m.lifecycleErr = msg.err.Error()
		}
		m.lifecycleState = "idle"
		return m, nil

	case restartProgressMsg:
		m.lifecycleStep = msg.step
		return m, nil

	case restartCompleteMsg:
		if msg.err != nil {
			m.lifecycleErr = msg.err.Error()
			m.lifecycleState = "idle"
		} else {
			m.lifecycleState = "running"
		}
		return m, nil

	case pluginsUpdateMsg:
		if msg.err != nil {
			m.pluginErr = msg.err.Error()
			m.pluginsState = pluginsStateError
		} else {
			m.pluginListItems = make([]pluginListItem, len(msg.plugins))
			for i, p := range msg.plugins {
				m.pluginListItems[i] = pluginListItem{info: p}
			}
			m.pluginsState = pluginsStateList
			m.pluginErr = ""
		}
		return m, nil

	case pluginsReloadMsg:
		if msg.err != nil {
			m.pluginErr = msg.err.Error()
			m.pluginsState = pluginsStateError
		} else {
			m.pluginErr = ""
			m.pluginsState = pluginsStateLoading
			return m, refreshPluginsCmd(m.gwClient)
		}
		return m, nil

	case pluginsUnloadMsg:
		if msg.err != nil {
			m.pluginErr = msg.err.Error()
			m.pluginsState = pluginsStateError
		} else {
			m.pluginErr = ""
			m.pluginsState = pluginsStateLoading
			return m, refreshPluginsCmd(m.gwClient)
		}
		return m, nil

	case tea.KeyMsg:
		return handleKeyMsg(m, msg)
	}

		// Route unhandled messages to active form. Internal Bubbles messages
		// (FocusMsg, BlurMsg) need this to reach the textinput.
		if m.currentScreen == screenProviders {
			switch m.providersState {
			case pAddProvider, pEditProvider:
				var cmd tea.Cmd
				m.providerForm, cmd = m.providerForm.Update(msg)
				return m, cmd
			case pAddModel, pEditModel:
				var cmd tea.Cmd
				m.modelForm, cmd = m.modelForm.Update(msg)
				return m, cmd
			}
		}

		return m, nil
}

// ---------------------------------------------------------------------------
// Key dispatch
// ---------------------------------------------------------------------------

func handleKeyMsg(m model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Quit is always available.
	if key == "q" || key == "ctrl+c" {
		return m, tea.Quit
	}

	// Forms get exclusive key control — pass raw KeyMsg through.
	if m.currentScreen == screenProviders && m.providersState != pList {
		return handleProvidersKeyMsg(m, msg)
	}

	// Home screen keys.
	if m.currentScreen == screenHome {
		return handleHomeKeys(m, key)
	}

	// Content screen keys.
	return handleContentKeys(m, key, msg)
}

// ---------------------------------------------------------------------------
// Home screen keys
// ---------------------------------------------------------------------------

func handleHomeKeys(m model, key string) (tea.Model, tea.Cmd) {
	switch key {
	case "j", "down":
		m.homeCursor = (m.homeCursor + 1) % numHomeItems
		return m, nil
	case "k", "up":
		m.homeCursor = (m.homeCursor - 1 + numHomeItems) % numHomeItems
		return m, nil
	case "enter", " ":
		return switchToScreen(m, m.homeCursor)
	case "1":
		return switchToScreen(m, 0)
	case "2":
		return switchToScreen(m, 1)
	case "3":
		return switchToScreen(m, 2)
	}
	return m, nil
}

func switchToScreen(m model, idx int) (tea.Model, tea.Cmd) {
	m.contentCursor = 0
	switch idx {
	case 0:
		m.currentScreen = screenDashboard
		return m, nil
	case 1:
		m.currentScreen = screenProviders
		providers, singles, _ := listProviderFiles(m.cfg.ModelsDir)
		m.providerListItems = providers
		m.singleModelItems = singles
		return m, nil
	case 2:
		m.currentScreen = screenPlugins
		var cmds []tea.Cmd
		if m.pluginsState == pluginsStateIdle {
			m.pluginsState = pluginsStateLoading
			cmds = append(cmds, refreshPluginsCmd(m.gwClient))
		}
		return m, tea.Batch(cmds...)
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// Content screen keys
// ---------------------------------------------------------------------------

func handleContentKeys(m model, key string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key {
	case "j", "down":
		count := contentItemCount(m)
		if count > 0 {
			m.contentCursor = (m.contentCursor + 1) % count
		}
		return m, nil
	case "k", "up":
		count := contentItemCount(m)
		if count > 0 {
			m.contentCursor = (m.contentCursor - 1 + count) % count
		}
		return m, nil
	case "enter":
		return handleContentEnter(m)
	case "esc":
		m.currentScreen = screenHome
		m.contentCursor = 0
		return m, nil
	default:
		// Delegate to screen-specific handlers for remaining keys.
		switch m.currentScreen {
		case screenDashboard:
			return handleDashboardKeys(m, key)
		case screenProviders:
			return handleProvidersKeyMsg(m, msg)
		case screenPlugins:
			return handlePluginsKeyMsg(m, msg)
		}
	}
	return m, nil
}

func handleContentEnter(m model) (tea.Model, tea.Cmd) {
	switch m.currentScreen {
	case screenDashboard:
		return handleDashboardContentEnter(m)
	case screenProviders:
		return handleProvidersContentEnter(m)
	case screenPlugins:
		return handlePluginsContentEnter(m)
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// Dashboard content: lifecycle actions as content items
// ---------------------------------------------------------------------------

func handleDashboardKeys(m model, key string) (tea.Model, tea.Cmd) {
	return m, nil
}

func handleDashboardContentEnter(m model) (tea.Model, tea.Cmd) {
	c := m.contentCursor
	if c == 0 {
		return triggerLifecycleAction(m, 0) // Start / Stop
	}
	if c == 1 {
		return triggerLifecycleAction(m, 1) // Restart (only available when running)
	}
	return m, nil
}

func triggerLifecycleAction(m model, actionIdx int) (tea.Model, tea.Cmd) {
	if m.lifecycleState == "idle" && actionIdx == 0 {
		m.lifecycleState = "starting"
		m.lifecycleErr = ""
		return m, startGatewayCmd(m.gwLifecycle, m.cfg.GatewayURL)
	}
	if m.lifecycleState == "running" {
		if actionIdx == 0 {
			m.lifecycleState = "stopping"
			return m, stopGatewayCmd(m.gwLifecycle, 30*time.Second)
		}
		if actionIdx == 1 {
			m.lifecycleState = "restarting"
			m.lifecycleErr = ""
			return m, restartGatewayCmd(m.gwLifecycle, 30*time.Second)
		}
	}
	return m, nil
}

// contentItemCount returns how many selectable items are in the current content area.
func contentItemCount(m model) int {
	switch m.currentScreen {
	case screenHome:
		return 0
	case screenDashboard:
		// Lifecycle action buttons as selectable items
		if m.lifecycleState == "idle" {
			return 1 // [Start Gateway]
		}
		if m.lifecycleState == "running" {
			return 2 // [Stop Gateway], [Restart Gateway]
		}
		return 0
	case screenProviders:
		if m.providersState != pList {
			return 0
		}
		return 3 + len(m.providerListItems) + len(m.singleModelItems)
	case screenPlugins:
		if m.pluginsState != pluginsStateList && m.pluginsState != pluginsStateError {
			return 0
		}
		return 3 + len(m.pluginListItems)
	}
	return 0
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func (m model) View() tea.View {
	if !m.ready {
		return tea.NewView("")
	}

	var body string
	if m.currentScreen == screenHome {
		body = renderHome(m)
	} else {
		body = renderContent(m)
	}

	help := renderHelpBar(m)
	statusBar := renderStatusBar(m)

	full := lipgloss.JoinVertical(
		lipgloss.Top,
		body,
		statusBar,
		help,
	)
	return tea.NewView(full)
}

// ---------------------------------------------------------------------------
// Help bar
// ---------------------------------------------------------------------------

func renderHelpBar(m model) string {
	var parts []string
	if m.currentScreen == screenHome {
		parts = append(parts, m.styles.actionKey.Render("j/k")+": navigate")
		parts = append(parts, m.styles.actionKey.Render("enter")+": select")
		parts = append(parts, m.styles.actionKey.Render("1-3")+": jump")
	} else {
		parts = append(parts, m.styles.actionKey.Render("j/k")+": select item")
		parts = append(parts, m.styles.actionKey.Render("enter")+": confirm")
		parts = append(parts, m.styles.actionKey.Render("esc")+": back to menu")
	}
	parts = append(parts, m.styles.actionKey.Render("q")+": quit")
	return m.styles.helpBar.Width(m.width).Render(" " + strings.Join(parts, " | ") + " ")
}

// ---------------------------------------------------------------------------
// Status bar
// ---------------------------------------------------------------------------

func renderStatusBar(m model) string {
	dot := m.styles.RenderStatusDot(m.gwStatus)

	var healthPart string
	if m.healthText != "" {
		healthPart = m.healthText
	} else if m.nodeStatus != nil {
		healthPart = m.nodeStatus.Status
	} else {
		healthPart = "Unknown"
	}

	errPart := ""
	if m.statusErr != "" {
		errPart = " | " + m.styles.errorStyle.Render(m.statusErr)
	}

	bar := fmt.Sprintf("%s %s%s", dot, healthPart, errPart)
	return m.styles.statusBar.Width(m.width).Render(bar)
}

// ---------------------------------------------------------------------------
// renderContent dispatches to the active screen's render function
// ---------------------------------------------------------------------------

func renderContent(m model) string {
	switch m.currentScreen {
	case screenDashboard:
		return renderDashboard(m)
	case screenProviders:
		return renderProvidersContent(m)
	case screenPlugins:
		return renderPluginsContent(m)
	default:
		return ""
	}
}

// ---------------------------------------------------------------------------
// Helper: renderText
// ---------------------------------------------------------------------------

func renderText(m model, text string) string {
	return m.styles.helpStyle.
		Width(m.width).
		Height(m.height - 3).
		Align(lipgloss.Center).
		Render(text)
}

// ---------------------------------------------------------------------------
// Helper: pollCmd
// ---------------------------------------------------------------------------

func pollCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return pollTickMsg{t}
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// ---------------------------------------------------------------------------
// Entry point
// ---------------------------------------------------------------------------

func startTUI(cfg TUIConfig) {
	gwClient := gateway.NewClient(cfg.GatewayURL, cfg.AdminToken, 10*time.Second)
	gwLifecycle := gateway.NewLifecycle(cfg.GatewayBinary, cfg.WorkingDir, 30*time.Second)
	m := newModel(cfg, gwClient, gwLifecycle)
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}

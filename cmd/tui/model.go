package main

import (
	"fmt"
	"os"
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
// Tab enumeration
// ---------------------------------------------------------------------------

type activeTab int

const (
	tabDashboard activeTab = iota
	tabProviders
	tabPlugins
)

// ---------------------------------------------------------------------------
// Model
// ---------------------------------------------------------------------------

// model is the top-level Bubble Tea model for the TUI application.
type model struct {
	cfg             *TUIConfig
	gwClient        *gateway.Client
	gwLifecycle     *gateway.Lifecycle
	styles          styles
	keymap          keymap
	width           int
	height          int
	activeTab       activeTab
	gwStatus        string  // "healthy", "degraded", "down", "unknown"
	healthText      string  // human-readable health description
	nodeStatus      *services.NodeStatus
	statusErr       string
	polling         bool
	darkTheme       bool
	ready           bool
	lifecycleState  string // "idle", "starting", "stopping", "restarting"
	lifecycleErr    string
	lifecycleStep   string // current restart step text

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
		keymap:         NewKeymap(),
		gwStatus:       "unknown",
		lifecycleState: "idle",
		pluginsState:   pluginsStateIdle,
	}
}

// ---------------------------------------------------------------------------
// Init
// ---------------------------------------------------------------------------

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tea.RequestBackgroundColor, // will fire BackgroundColorMsg when terminal responds
		pollCmd(m.cfg.PollInterval), // first tick after pollInterval
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
			m.healthText = msg.err.Error()
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
			m.statusErr = msg.err.Error()
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
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab", "l":
			m.activeTab = (m.activeTab + 1) % 3
			var cmds []tea.Cmd
			if m.activeTab == tabProviders {
				m.providersState = pList
				providers, singles, _ := listProviderFiles(m.cfg.ModelsDir)
				m.providerListItems = providers
				m.singleModelItems = singles
			}
			if m.activeTab == tabPlugins && m.pluginsState == pluginsStateIdle {
				m.pluginsState = pluginsStateLoading
				cmds = append(cmds, refreshPluginsCmd(m.gwClient))
			}
			return m, tea.Batch(cmds...)
		case "shift+tab", "h":
			// When inside a form on the providers tab, let the providers handler
			// manage shift+tab for form navigation instead of switching tabs.
			if m.activeTab == tabProviders && m.providersState != pList {
				return handleProvidersKeyMsg(m, msg)
			}
			m.activeTab = (m.activeTab - 1 + 3) % 3
			var cmds []tea.Cmd
			if m.activeTab == tabProviders {
				m.providersState = pList
				providers, singles, _ := listProviderFiles(m.cfg.ModelsDir)
				m.providerListItems = providers
				m.singleModelItems = singles
			}
			if m.activeTab == tabPlugins && m.pluginsState == pluginsStateIdle {
				m.pluginsState = pluginsStateLoading
				cmds = append(cmds, refreshPluginsCmd(m.gwClient))
			}
			return m, tea.Batch(cmds...)
		case "1":
			m.activeTab = tabDashboard
			return m, nil
		case "2":
			m.activeTab = tabProviders
			if m.providersState == pList {
				providers, singles, _ := listProviderFiles(m.cfg.ModelsDir)
				m.providerListItems = providers
				m.singleModelItems = singles
			}
			return m, nil
		case "3":
			m.activeTab = tabPlugins
			var cmds3 []tea.Cmd
			if m.pluginsState == pluginsStateIdle {
				m.pluginsState = pluginsStateLoading
				cmds3 = append(cmds3, refreshPluginsCmd(m.gwClient))
			}
			return m, tea.Batch(cmds3...)
		default:
			// Delegate to active tab's key handler
			switch m.activeTab {
			case tabDashboard:
				return handleDashboardKeyMsg(m, msg)
			case tabProviders:
				return handleProvidersKeyMsg(m, msg)
			case tabPlugins:
				return handlePluginsKeyMsg(m, msg)
			}
		}
	}

	return m, nil
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func (m model) View() tea.View {
	if !m.ready {
		return tea.NewView("")
	}

	content := lipgloss.JoinVertical(
		lipgloss.Top,
		renderTabBar(m),
		renderContent(m),
		renderStatusBar(m),
	)
	return tea.NewView(content)
}

// ---------------------------------------------------------------------------
// Helper: newModel (re-exported for tests)
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Helper: renderTabBar
// ---------------------------------------------------------------------------

func renderTabBar(m model) string {
	tabs := []struct {
		label string
		tab   activeTab
	}{
		{" Dashboard ", tabDashboard},
		{" Providers ", tabProviders},
		{" Plugins ", tabPlugins},
	}

	var rendered string
	for _, t := range tabs {
		if m.activeTab == t.tab {
			rendered += m.styles.tabActive.Render(t.label)
		} else {
			rendered += m.styles.tabInactive.Render(t.label)
		}
		rendered += " "
	}
	return rendered
}

// ---------------------------------------------------------------------------
// Helper: renderContent dispatches to the active tab's render function
// ---------------------------------------------------------------------------

func renderContent(m model) string {
	switch m.activeTab {
	case tabDashboard:
		return renderDashboard(m)
	case tabProviders:
		return renderProvidersContent(m)
	case tabPlugins:
		return renderPluginsContent(m)
	default:
		return ""
	}
}

// ---------------------------------------------------------------------------
// Helper: renderStatusBar
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
// Helper: renderText — centered placeholder text for tabs not yet implemented
// ---------------------------------------------------------------------------

func renderText(m model, text string) string {
	return m.styles.helpStyle.
		Width(m.width).
		Height(m.height - 3).
		Align(lipgloss.Center).
		Render(text)
}

// ---------------------------------------------------------------------------
// Helper: pollCmd — schedules the next polling tick
// ---------------------------------------------------------------------------

func pollCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return pollTickMsg{t}
	})
}

// ---------------------------------------------------------------------------
// Entry point: startTUI — creates and runs the Bubble Tea program
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

package main

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"wolink-core/cmd/tui/gateway"
	"wolink-core/internal/plugins"
)

// ---------------------------------------------------------------------------
// State types
// ---------------------------------------------------------------------------

// pluginsTabState represents the current sub-state of the Plugins tab.
type pluginsTabState int

const (
	pluginsStateIdle    pluginsTabState = iota
	pluginsStateLoading
	pluginsStateList
	pluginsStateError
)

// pluginListItem wraps a single plugin.PluginInfo for display in the list.
type pluginListItem struct {
	info plugins.PluginInfo
}

// ---------------------------------------------------------------------------
// Message types
// ---------------------------------------------------------------------------

// pluginsUpdateMsg carries the result of a ListPlugins() call.
type pluginsUpdateMsg struct {
	plugins []plugins.PluginInfo
	err     error
}

// pluginsReloadMsg carries the result of a ReloadPlugin() call.
type pluginsReloadMsg struct {
	protocol string
	err      error
}

// pluginsUnloadMsg carries the result of an UnloadPlugin() call.
type pluginsUnloadMsg struct {
	protocol string
	err      error
}

// ---------------------------------------------------------------------------
// Rendering
// ---------------------------------------------------------------------------

// renderPluginsContent returns the full rendered content for the Plugins tab,
// dispatching to the sub-state's render function.
func renderPluginsContent(m model) string {
	switch m.pluginsState {
	case pluginsStateLoading:
		return renderPluginsLoading(m)
	case pluginsStateError:
		return renderPluginsError(m)
	default:
		return renderPluginsList(m)
	}
}

// renderPluginsLoading renders a centered "Loading plugins..." message.
func renderPluginsLoading(m model) string {
	return m.styles.helpStyle.
		Width(m.width).
		Height(m.height - 3).
		Align(lipgloss.Center).
		Render("Loading plugins...")
}

// renderPluginsError renders the plugin list with an error message.
func renderPluginsError(m model) string {
	var b strings.Builder

	heading := m.styles.titleStyle.Render("Plugins")
	b.WriteString(fmt.Sprintf(" %s\n", heading))
	b.WriteString(" " + strings.Repeat("─", clampWidth(m.width-2, 60)) + "\n\n")

	if m.pluginErr != "" {
		b.WriteString(" " + m.styles.errorStyle.Render(m.pluginErr) + "\n")
	}

	return b.String()
}

// renderPluginsList renders the plugin list table with action items.
func renderPluginsList(m model) string {
	var b strings.Builder

	heading := m.styles.titleStyle.Render("Plugins")
	b.WriteString(fmt.Sprintf(" %s\n", heading))
	b.WriteString(" " + strings.Repeat("─", clampWidth(m.width-2, 60)) + "\n\n")

	// Action items at top
	b.WriteString(renderPluginsActions(m))
	b.WriteString("\n")

	// Column headers
	b.WriteString(fmt.Sprintf("   %-20s %-12s %-10s %-8s %-8s\n", "Name", "Protocol", "Version", "Loaded", "Healthy"))
	b.WriteString(fmt.Sprintf("   %s\n", strings.Repeat("─", clampWidth(m.width-2, 60))))

	if len(m.pluginListItems) == 0 {
		b.WriteString("   No plugins loaded.\n")
		return b.String()
	}

	for i, item := range m.pluginListItems {
		cursorIdx := 3 + i // action buttons are 0,1,2
		cursor := "  "
		if m.contentCursor == cursorIdx {
			cursor = m.styles.actionActive.Render(" >")
		}

		loaded := m.styles.statusGreen.Render("yes")
		if !item.info.Loaded {
			loaded = m.styles.statusRed.Render("no")
		}
		healthy := m.styles.statusGreen.Render("yes")
		if !item.info.Healthy {
			healthy = m.styles.statusRed.Render("no")
		}

		b.WriteString(fmt.Sprintf("   %s %-20s %-12s %-10s %-8s %-8s\n",
			cursor,
			item.info.Name,
			item.info.Protocol,
			item.info.Version,
			loaded,
			healthy))
	}

	return b.String()
}

// renderPluginsActions renders the visible action buttons for the plugins tab.
func renderPluginsActions(m model) string {
	actions := []string{"[Unload Selected]", "[Refresh List]", "[Health Check All]"}
	var rendered []string
	for i, label := range actions {
		if m.contentCursor == i {
			rendered = append(rendered, m.styles.actionActive.Render(label))
		} else {
			rendered = append(rendered, m.styles.actionItem.Render(label))
		}
	}
	return "  " + strings.Join(rendered, "  ") + "\n"
}

// ---------------------------------------------------------------------------
// Content enter handling
// ---------------------------------------------------------------------------

func handlePluginsContentEnter(m model) (tea.Model, tea.Cmd) {
	c := m.contentCursor

	// Action buttons (0, 1, 2)
	if c == 0 {
		// Unload selected plugin
		if len(m.pluginListItems) == 0 {
			return m, nil
		}
		idx := m.contentCursor - 3
		if idx < 0 {
			idx = 0
		}
		if idx < len(m.pluginListItems) {
			protocol := m.pluginListItems[idx].info.Protocol
			m.pluginsState = pluginsStateLoading
			return m, unloadPluginCmd(m.gwClient, protocol)
		}
		return m, nil
	}
	if c == 1 {
		// Refresh list
		m.pluginsState = pluginsStateLoading
		return m, refreshPluginsCmd(m.gwClient)
	}
	if c == 2 {
		// Health check all
		m.pluginsState = pluginsStateLoading
		return m, healthCheckPluginsCmd(m.gwClient)
	}

	// Plugin list items (reload)
	pluginIdx := c - 3
	if pluginIdx >= 0 && pluginIdx < len(m.pluginListItems) {
		protocol := m.pluginListItems[pluginIdx].info.Protocol
		m.pluginsState = pluginsStateLoading
		return m, reloadPluginCmd(m.gwClient, protocol)
	}

	return m, nil
}

// ---------------------------------------------------------------------------
// Key handling (minimal — list navigation handled by model.go)
// ---------------------------------------------------------------------------

func handlePluginsKeyMsg(m model, msg tea.KeyMsg) (model, tea.Cmd) {
	// No plugin-specific keys in list mode — all navigation via model.go focus system.
	return m, nil
}

// ---------------------------------------------------------------------------
// tea.Cmd factories
// ---------------------------------------------------------------------------

// refreshPluginsCmd creates a tea.Cmd that calls gwClient.ListPlugins() and
// returns a pluginsUpdateMsg.
func refreshPluginsCmd(gwClient *gateway.Client) tea.Cmd {
	return func() tea.Msg {
		plugins, err := gwClient.ListPlugins()
		return pluginsUpdateMsg{plugins: plugins, err: err}
	}
}

// reloadPluginCmd creates a tea.Cmd that calls gwClient.ReloadPlugin() and
// returns a pluginsReloadMsg.
func reloadPluginCmd(gwClient *gateway.Client, protocol string) tea.Cmd {
	return func() tea.Msg {
		err := gwClient.ReloadPlugin(protocol)
		return pluginsReloadMsg{protocol: protocol, err: err}
	}
}

// unloadPluginCmd creates a tea.Cmd that calls gwClient.UnloadPlugin() and
// returns a pluginsUnloadMsg.
func unloadPluginCmd(gwClient *gateway.Client, protocol string) tea.Cmd {
	return func() tea.Msg {
		err := gwClient.UnloadPlugin(protocol)
		return pluginsUnloadMsg{protocol: protocol, err: err}
	}
}

// healthCheckPluginsCmd creates a tea.Cmd that calls
// gwClient.HealthCheckPlugins() and returns a pluginsUpdateMsg.
func healthCheckPluginsCmd(gwClient *gateway.Client) tea.Cmd {
	return func() tea.Msg {
		plugins, err := gwClient.HealthCheckPlugins()
		return pluginsUpdateMsg{plugins: plugins, err: err}
	}
}

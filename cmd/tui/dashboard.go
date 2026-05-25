package main

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ---------------------------------------------------------------------------
// Dashboard render
// ---------------------------------------------------------------------------

// renderDashboard renders the Dashboard tab content with status panels,
// live metrics, lifecycle action buttons, and error display.
func renderDashboard(m model) string {
	panel1 := renderGatewayStatusPanel(m)
	panel2 := renderNodeMetricsPanel(m)
	panel3 := renderQuickActionsPanel(m)
	panel4 := renderLifecyclePanel(m)
	panel5 := renderErrorPanel(m)

	// Collect only non-empty panels
	var panels []string
	for _, p := range []string{panel1, panel2, panel3, panel4, panel5} {
		if p != "" {
			panels = append(panels, p)
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Top, panels...)

	// Constrain height to available terminal space
	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height - 3).
		Render(content)
}

// renderGatewayStatusPanel displays the gateway health status with a colored dot.
func renderGatewayStatusPanel(m model) string {
	header := m.styles.headerStyle.Render("--- Gateway Status ---") + "\n"

	dot := m.styles.RenderStatusDot(m.gwStatus)
	var statusLine string
	if m.healthText != "" {
		statusLine = fmt.Sprintf("%s %s", dot, m.healthText)
	} else {
		statusLine = fmt.Sprintf("%s %s", dot, "Unknown")
	}

	return lipgloss.NewStyle().Margin(0, 2, 0, 1).Render(header + statusLine)
}

// renderNodeMetricsPanel displays the node metrics in a table-style layout.
func renderNodeMetricsPanel(m model) string {
	header := m.styles.headerStyle.Render("--- Node Metrics ---") + "\n"

	if m.nodeStatus == nil {
		body := m.styles.helpStyle.Render("No status data")
		return lipgloss.NewStyle().Margin(0, 2, 0, 1).Render(header + body)
	}

	// Build labeled rows with fixed column width for alignment
	labelWidth := 12
	var rows []string
	addRow := func(label, value string) {
		labelStyled := lipgloss.NewStyle().Width(labelWidth).Render(label)
		rows = append(rows, fmt.Sprintf("  %s %s", labelStyled, value))
	}

	addRow("Node ID:", m.nodeStatus.NodeID)
	addRow("Status:", m.nodeStatus.Status)
	addRow("Uptime:", m.nodeStatus.Uptime)
	addRow("Version:", m.nodeStatus.Version)
	addRow("Goroutines:", fmt.Sprintf("%d", m.nodeStatus.Goroutines))
	addRow("Memory:", fmt.Sprintf("%d MB", m.nodeStatus.MemoryUsageMB))

	body := strings.Join(rows, "\n")
	return lipgloss.NewStyle().Margin(0, 2, 0, 1).Render(header + body + "\n")
}

// renderQuickActionsPanel displays keybinding hints for common actions.
func renderQuickActionsPanel(m model) string {
	header := m.styles.headerStyle.Render("--- Quick Actions ---") + "\n"

	actions := []string{
		"  Tab / l : Next tab",
		"  q / Ctrl+C : Quit",
		"  Ctrl+S : Start gateway",
		"  Ctrl+X : Stop gateway",
		"  Ctrl+R : Restart gateway",
	}
	body := m.styles.helpStyle.Render(strings.Join(actions, "\n"))

	return lipgloss.NewStyle().Margin(0, 2, 0, 1).Render(header + body + "\n")
}

// renderLifecyclePanel displays the current lifecycle state and spinner.
// Only rendered when lifecycleState is not "idle".
func renderLifecyclePanel(m model) string {
	if m.lifecycleState == "idle" {
		return ""
	}

	header := m.styles.headerStyle.Render("--- Lifecycle ---") + "\n"

	var body string
	switch m.lifecycleState {
	case "starting":
		body = m.styles.infoStyle.Render(
			fmt.Sprintf("%s Starting gateway...", m.spinnerModel.View()),
		)
	case "stopping":
		body = m.styles.infoStyle.Render(
			fmt.Sprintf("%s Stopping gateway...", m.spinnerModel.View()),
		)
	case "restarting":
		stepText := m.lifecycleStep
		if stepText == "" {
			stepText = "Restarting..."
		}
		body = m.styles.infoStyle.Render(
			fmt.Sprintf("%s %s", m.spinnerModel.View(), stepText),
		)
	case "running":
		body = m.styles.successStyle.Render("Gateway is running")
	}

	// Show lifecycle error if one exists
	if m.lifecycleErr != "" {
		body += "\n" + m.styles.errorStyle.Render("Error: "+m.lifecycleErr)
	}

	return lipgloss.NewStyle().Margin(0, 2, 0, 1).Render(header + body + "\n")
}

// renderErrorPanel displays any non-lifecycle errors.
// Only rendered when statusErr is non-empty.
func renderErrorPanel(m model) string {
	if m.statusErr == "" {
		return ""
	}

	header := m.styles.errorStyle.Render("--- Errors ---") + "\n"
	body := m.styles.errorStyle.Render("  " + m.statusErr)

	return lipgloss.NewStyle().Margin(0, 2, 0, 1).Render(header + body + "\n")
}

// ---------------------------------------------------------------------------
// Dashboard key handler
// ---------------------------------------------------------------------------

// handleDashboardKeyMsg handles key events when the Dashboard tab is active.
// It maps Ctrl+S, Ctrl+X, and Ctrl+R to lifecycle actions.
func handleDashboardKeyMsg(m model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Block lifecycle keys while a lifecycle operation is in progress
	if m.lifecycleState != "idle" && m.lifecycleState != "running" {
		return m, nil
	}

	switch msg.String() {
	case "ctrl+s":
		if m.lifecycleState == "idle" {
			m.lifecycleState = "starting"
			m.lifecycleErr = ""
			return m, tea.Batch(
				startGatewayCmd(m.gwLifecycle, m.cfg.GatewayURL),
				func() tea.Msg { return m.spinnerModel.Tick() },
			)
		}
		return m, nil

	case "ctrl+x":
		if m.lifecycleState != "idle" {
			m.lifecycleState = "stopping"
			return m, tea.Batch(
				stopGatewayCmd(m.gwLifecycle, 30*time.Second),
				func() tea.Msg { return m.spinnerModel.Tick() },
			)
		}
		return m, nil

	case "ctrl+r":
		if m.lifecycleState == "idle" || m.lifecycleState == "running" {
			m.lifecycleState = "restarting"
			m.lifecycleErr = ""
			return m, tea.Batch(
				restartGatewayCmd(m.gwLifecycle, 30*time.Second),
				func() tea.Msg { return m.spinnerModel.Tick() },
			)
		}
		return m, nil
	}

	return m, nil
}

package main

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// renderDashboard renders the Dashboard tab content with status panels,
// live metrics, lifecycle action buttons, and error display.
func renderDashboard(m model) string {
	s := strings.Builder{}

	// Panel 1: Gateway Status
	s.WriteString(renderGatewayStatusPanel(m))

	// Panel 2: Node Metrics
	s.WriteString(renderNodeMetricsPanel(m))

	// Panel 3: Quick Actions
	s.WriteString(renderQuickActionsPanel(m))

	// Panel 4: Lifecycle State (only if active)
	if lifecycle := renderLifecyclePanel(m); lifecycle != "" {
		s.WriteString(lifecycle)
	}

	// Panel 5: Errors
	if errPanel := renderErrorPanel(m); errPanel != "" {
		s.WriteString(errPanel)
	}

	return s.String()
}

func renderGatewayStatusPanel(m model) string {
	header := m.styles.headerStyle.Render("--- Gateway Status ---") + "\n"
	dot := m.styles.RenderStatusDot(m.gwStatus)
	statusLine := fmt.Sprintf("%s %s", dot, m.healthText)
	if m.healthText == "" {
		statusLine = fmt.Sprintf("%s Unknown", dot)
	}
	return lipgloss.NewStyle().Margin(0, 2, 0, 1).Render(header + statusLine + "\n")
}

func renderNodeMetricsPanel(m model) string {
	header := m.styles.headerStyle.Render("--- Node Metrics ---") + "\n"
	if m.nodeStatus == nil {
		body := m.styles.helpStyle.Render("No status data")
		return lipgloss.NewStyle().Margin(0, 2, 0, 1).Render(header + body + "\n")
	}
	labelWidth := 12
	var rows []string
	addRow := func(label, value string) {
		rows = append(rows, fmt.Sprintf("  %s %s", lipgloss.NewStyle().Width(labelWidth).Render(label), value))
	}
	addRow("Node ID:", m.nodeStatus.NodeID)
	addRow("Status:", m.nodeStatus.Status)
	addRow("Uptime:", m.nodeStatus.Uptime)
	addRow("Version:", m.nodeStatus.Version)
	addRow("Goroutines:", fmt.Sprintf("%d", m.nodeStatus.Goroutines))
	addRow("Memory:", fmt.Sprintf("%d MB", m.nodeStatus.MemoryUsageMB))
	return lipgloss.NewStyle().Margin(0, 2, 0, 1).Render(header + strings.Join(rows, "\n") + "\n")
}

func renderQuickActionsPanel(m model) string {
	header := m.styles.headerStyle.Render("--- Quick Actions ---") + "\n"
	actions := []string{
		"  Tab / l : Next tab",
		"  q / Ctrl+C : Quit",
		"  Ctrl+S : Start gateway",
		"  Ctrl+X : Stop gateway",
		"  Ctrl+R : Restart gateway",
	}
	return lipgloss.NewStyle().Margin(0, 2, 0, 1).Render(header + m.styles.helpStyle.Render(strings.Join(actions, "\n")) + "\n")
}

func renderLifecyclePanel(m model) string {
	if m.lifecycleState == "idle" || m.lifecycleState == "running" {
		return ""
	}
	header := m.styles.headerStyle.Render("--- Lifecycle ---") + "\n"
	var body string
	switch m.lifecycleState {
	case "starting":
		body = m.styles.infoStyle.Render("Starting gateway...")
	case "stopping":
		body = m.styles.infoStyle.Render("Stopping gateway...")
	case "restarting":
		stepText := m.lifecycleStep
		if stepText == "" {
			stepText = "Restarting..."
		}
		body = m.styles.infoStyle.Render(stepText)
	}
	if m.lifecycleErr != "" {
		body += "\n" + m.styles.errorStyle.Render("Error: "+m.lifecycleErr)
	}
	return lipgloss.NewStyle().Margin(0, 2, 0, 1).Render(header + body + "\n")
}

func renderErrorPanel(m model) string {
	if m.statusErr == "" {
		return ""
	}
	header := m.styles.errorStyle.Render("--- Errors ---") + "\n"
	return lipgloss.NewStyle().Margin(0, 2, 0, 1).Render(header + m.styles.errorStyle.Render("  "+m.statusErr) + "\n")
}

// handleDashboardKeyMsg handles key events when the Dashboard tab is active.
func handleDashboardKeyMsg(m model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.lifecycleState != "idle" && m.lifecycleState != "running" {
		return m, nil
	}
	switch msg.String() {
	case "ctrl+s":
		if m.lifecycleState == "idle" {
			m.lifecycleState = "starting"
			m.lifecycleErr = ""
			return m, startGatewayCmd(m.gwLifecycle, m.cfg.GatewayURL)
		}
	case "ctrl+x":
		if m.lifecycleState != "idle" {
			m.lifecycleState = "stopping"
			return m, stopGatewayCmd(m.gwLifecycle, 30*time.Second)
		}
	case "ctrl+r":
		if m.lifecycleState == "idle" || m.lifecycleState == "running" {
			m.lifecycleState = "restarting"
			m.lifecycleErr = ""
			return m, restartGatewayCmd(m.gwLifecycle, 30*time.Second)
		}
	}
	return m, nil
}

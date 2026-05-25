package main

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// renderDashboard renders the Dashboard screen content with status panels,
// lifecycle actions, live metrics, and error display.
func renderDashboard(m model) string {
	// Lifecycle actions as visible menu items
	actions := renderDashboardActions(m)

	panel1 := renderGatewayStatusPanel(m)
	panel2 := renderNodeMetricsPanel(m)
	panel3 := renderLifecyclePanel(m)
	panel4 := renderErrorPanel(m)

	var panels []string
	if actions != "" {
		panels = append(panels, actions)
	}
	for _, p := range []string{panel1, panel2, panel3, panel4} {
		if p != "" {
			panels = append(panels, p)
		}
	}

	return lipgloss.JoinVertical(lipgloss.Top, panels...)
}

func renderDashboardActions(m model) string {
	var labels []string
	if m.lifecycleState == "idle" {
		labels = []string{"[Start Gateway]"}
	} else if m.lifecycleState == "running" {
		labels = []string{"[Stop Gateway]", "[Restart Gateway]"}
	}
	if len(labels) == 0 {
		return ""
	}
	return renderContentActions(m, labels)
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

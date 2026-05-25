package main

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// renderHome renders the welcome/home screen with 3 menu options.
func renderHome(m model) string {
	var b strings.Builder

	title := m.styles.homeTitle.Render("wolink AI Gateway Manager")
	b.WriteString(title)
	b.WriteString("\n\n")

	subtitle := m.styles.helpStyle.Render("Select an option:")
	b.WriteString(subtitle)
	b.WriteString("\n\n")

	items := []struct {
		label       string
		description string
	}{
		{"Dashboard", "View gateway status, node metrics, and manage lifecycle"},
		{"Providers", "Add, edit, or delete provider and model configurations"},
		{"Plugins", "View, reload, unload, and health-check plugins"},
	}

	for i, item := range items {
		prefix := "  "
		label := item.label
		desc := item.description

		if i == m.homeCursor {
			prefix = m.styles.homeActive.Render("> ")
			label = m.styles.homeActive.Render(item.label)
			desc = m.styles.homeActiveDesc.Render(item.description)
		} else {
			label = m.styles.homeItem.Render(item.label)
			desc = m.styles.helpStyle.Render(item.description)
		}

		b.WriteString(fmt.Sprintf("%s%s\n", prefix, label))
		b.WriteString(fmt.Sprintf("    %s\n\n", desc))
	}

	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height-3).
		Padding(4, 4).
		Render(b.String())
}

// renderContentActions renders visible action buttons for the current screen.
// Used on dashboard (lifecycle) and plugin/provider screens.
func renderContentActions(m model, labels []string) string {
	if len(labels) == 0 {
		return ""
	}
	var rendered []string
	for i, label := range labels {
		if m.contentCursor == i {
			rendered = append(rendered, m.styles.homeActive.Render(label))
		} else {
			rendered = append(rendered, m.styles.homeItem.Render(label))
		}
	}
	return "  " + strings.Join(rendered, "  ") + "\n"
}

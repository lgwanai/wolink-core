package main

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// styles holds all Lipgloss style definitions for the TUI.
// Colors adapt to light/dark terminal themes via lipgloss.LightDark.
type styles struct {
	// Tab bar
	tabActive   lipgloss.Style
	tabInactive lipgloss.Style

	// Status indicators
	statusGreen  lipgloss.Style
	statusYellow lipgloss.Style
	statusRed    lipgloss.Style
	statusBlue   lipgloss.Style

	// Layout
	headerStyle  lipgloss.Style
	contentArea  lipgloss.Style
	statusBar    lipgloss.Style
	helpStyle    lipgloss.Style

	// Semantic
	errorStyle   lipgloss.Style
	successStyle lipgloss.Style
	titleStyle   lipgloss.Style
	infoStyle    lipgloss.Style
}

// ld is a shorthand for lipgloss.LightDark.
func ld(isDark bool, light, dark color.Color) color.Color {
	return lipgloss.LightDark(isDark)(light, dark)
}

// newStyles creates a styles struct with colors adapted to the terminal theme.
func newStyles(isDark bool) styles {
	return styles{
		tabActive: lipgloss.NewStyle().
			Background(ld(isDark, lipgloss.Color("#cccccc"), lipgloss.Color("#333333"))).
			Foreground(ld(isDark, lipgloss.Color("#000000"), lipgloss.Color("#ffffff"))),
		tabInactive: lipgloss.NewStyle().
			Background(ld(isDark, lipgloss.Color("#eeeeee"), lipgloss.Color("#111111"))).
			Foreground(ld(isDark, lipgloss.Color("#666666"), lipgloss.Color("#aaaaaa"))),

		statusGreen: lipgloss.NewStyle().
			Foreground(ld(isDark, lipgloss.Color("#00aa00"), lipgloss.Color("#00ff00"))),
		statusYellow: lipgloss.NewStyle().
			Foreground(ld(isDark, lipgloss.Color("#aa6600"), lipgloss.Color("#ffaa00"))),
		statusRed: lipgloss.NewStyle().
			Foreground(ld(isDark, lipgloss.Color("#aa0000"), lipgloss.Color("#ff0000"))),
		statusBlue: lipgloss.NewStyle().
			Foreground(ld(isDark, lipgloss.Color("#0066aa"), lipgloss.Color("#00aaff"))),

		headerStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(ld(isDark, lipgloss.Color("#000000"), lipgloss.Color("#ffffff"))),
		contentArea: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			Padding(1, 2),
		statusBar: lipgloss.NewStyle().
			BorderTop(true).
			Foreground(ld(isDark, lipgloss.Color("#333333"), lipgloss.Color("#cccccc"))),
		helpStyle: lipgloss.NewStyle().
			Faint(true).Italic(true).
			Foreground(ld(isDark, lipgloss.Color("#888888"), lipgloss.Color("#888888"))),

		errorStyle: lipgloss.NewStyle().
			Foreground(ld(isDark, lipgloss.Color("#aa0000"), lipgloss.Color("#ff0000"))),
		successStyle: lipgloss.NewStyle().
			Foreground(ld(isDark, lipgloss.Color("#00aa00"), lipgloss.Color("#00ff00"))),
		titleStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(ld(isDark, lipgloss.Color("#000000"), lipgloss.Color("#ffffff"))),
		infoStyle: lipgloss.NewStyle().
			Foreground(ld(isDark, lipgloss.Color("#0066aa"), lipgloss.Color("#00aaff"))),
	}
}

// RenderStatusDot returns a colored dot character using the appropriate
// status style. status should be one of: "healthy"/"ok" (green),
// "warning" (yellow), "error" (red), or anything else (blue/info).
func (s styles) RenderStatusDot(status string) string {
	switch status {
	case "healthy", "ok":
		return s.statusGreen.Render("●")
	case "warning":
		return s.statusYellow.Render("●")
	case "error", "unhealthy":
		return s.statusRed.Render("●")
	default:
		return s.statusBlue.Render("●")
	}
}

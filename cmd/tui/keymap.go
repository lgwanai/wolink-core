package main

import (
	"charm.land/bubbles/v2/key"
)

// keymap defines all keybindings for the TUI application.
// It implements the help.KeyMap interface for rendering help text.
type keymap struct {
	Quit      key.Binding
	Back      key.Binding
	Confirm   key.Binding
	FocusNext key.Binding
	FocusPrev key.Binding
	TabNext   key.Binding
	TabPrev   key.Binding
	Refresh   key.Binding
	Search    key.Binding
	Up        key.Binding
	Down      key.Binding
}

// NewKeymap returns a populated keymap with all navigation bindings.
func NewKeymap() keymap {
	return keymap{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q/ctrl+c", "quit"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		TabNext: key.NewBinding(
			key.WithKeys("tab", "l"),
			key.WithHelp("tab/l", "next tab"),
		),
		TabPrev: key.NewBinding(
			key.WithKeys("shift+tab", "h"),
			key.WithHelp("shift+tab/h", "prev tab"),
		),
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k/up", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j/down", "down"),
		),
		Confirm: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "confirm"),
		),
		FocusNext: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next"),
		),
		FocusPrev: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "prev"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
	}
}

// ShortHelp returns a subset of keybindings for the short help view.
func (k keymap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Up, k.Down, k.Confirm, k.Back, k.Quit,
	}
}

// FullHelp returns all keybindings grouped for the full help view.
func (k keymap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Confirm, k.Back},
		{k.TabNext, k.TabPrev, k.FocusNext, k.FocusPrev},
		{k.Refresh, k.Search, k.Quit},
	}
}

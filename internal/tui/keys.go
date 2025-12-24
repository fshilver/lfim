package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all keyboard shortcuts
type KeyMap struct {
	Up         key.Binding
	Down       key.Binding
	New        key.Binding
	Edit       key.Binding
	Close      key.Binding
	Discard    key.Binding
	Analyze    key.Binding
	Plan       key.Binding
	Review     key.Binding
	PlanReview key.Binding
	Implement  key.Binding
	Refresh    key.Binding
	Filter     key.Binding
	Quit       key.Binding
	Enter      key.Binding
	Escape     key.Binding
	Yes        key.Binding
	No         key.Binding
}

// DefaultKeyMap returns the default key bindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("k/↑", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("j/↓", "down"),
		),
		New: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "new"),
		),
		Edit: key.NewBinding(
			key.WithKeys("e", "enter"),
			key.WithHelp("e/↵", "edit"),
		),
		Close: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "close"),
		),
		Discard: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "discard"),
		),
		Analyze: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "analyze"),
		),
		Plan: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "plan"),
		),
		Review: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("R", "review"),
		),
		PlanReview: key.NewBinding(
			key.WithKeys("P"),
			key.WithHelp("P", "plan-review"),
		),
		Implement: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "implement"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Filter: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "filter"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
		),
		Yes: key.NewBinding(
			key.WithKeys("y", "Y"),
		),
		No: key.NewBinding(
			key.WithKeys("n", "N"),
		),
	}
}

// ShortHelp returns keybindings to be shown in the mini help view
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.New, k.Analyze, k.Review, k.Plan, k.PlanReview, k.Implement, k.Close, k.Discard, k.Edit, k.Filter, k.Quit}
}

// FullHelp returns keybindings for the expanded help view
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.New, k.Edit},
		{k.Analyze, k.Plan, k.Review, k.PlanReview},
		{k.Implement, k.Close, k.Discard, k.Filter},
		{k.Refresh, k.Quit},
	}
}

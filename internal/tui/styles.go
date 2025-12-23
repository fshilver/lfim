package tui

import "github.com/charmbracelet/lipgloss"

// Styles defines all visual styles for the TUI
type Styles struct {
	// Layout
	App          lipgloss.Style
	ListPanel    lipgloss.Style
	PreviewPanel lipgloss.Style

	// Header/Footer
	Header    lipgloss.Style
	Footer    lipgloss.Style
	StatusBar lipgloss.Style

	// List items
	SelectedItem   lipgloss.Style
	NormalItem     lipgloss.Style
	ProcessingItem lipgloss.Style

	// Preview
	PreviewTitle  lipgloss.Style
	PreviewBorder lipgloss.Style

	// Popup
	PopupBorder lipgloss.Style
	PopupTitle  lipgloss.Style

	// Input
	InputPrompt lipgloss.Style
	InputText   lipgloss.Style
}

// DefaultStyles returns the default style configuration
func DefaultStyles() Styles {
	return Styles{
		App: lipgloss.NewStyle(),

		ListPanel: lipgloss.NewStyle().
			Padding(0, 1),

		PreviewPanel: lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderLeft(true).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1),

		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212")).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Padding(0, 1),

		StatusBar: lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Padding(0, 1),

		SelectedItem: lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("230")),

		NormalItem: lipgloss.NewStyle(),

		ProcessingItem: lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")),

		PreviewTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212")),

		PreviewBorder: lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()),

		PopupBorder: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2),

		PopupTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212")),

		InputPrompt: lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")),

		InputText: lipgloss.NewStyle().
			Foreground(lipgloss.Color("255")),
	}
}

// Status icons for each issue status
var StatusIcons = map[string]string{
	"open":     "○",
	"analyzed": "◐",
	"planned":  "●",
	"closed":   "✓",
	"invalid":  "✗",
}

// Spinner frames for processing animation
var SpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

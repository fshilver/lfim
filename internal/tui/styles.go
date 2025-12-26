package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/lunit-heesungyang/issue-manager/internal/ui"
)

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
			BorderForeground(ui.ColorBorder).
			Padding(0, 1),

		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(ui.ColorPrimary).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(ui.ColorBorder).
			Padding(0, 1),

		StatusBar: lipgloss.NewStyle().
			Foreground(ui.ColorWarning).
			Padding(0, 1),

		SelectedItem: lipgloss.NewStyle().
			Background(ui.ColorSecondary).
			Foreground(ui.ColorTextLight),

		NormalItem: lipgloss.NewStyle(),

		ProcessingItem: lipgloss.NewStyle().
			Foreground(ui.ColorWarning),

		PreviewTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(ui.ColorPrimary),

		PreviewBorder: lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()),

		PopupBorder: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ui.ColorSecondary).
			Padding(1, 2),

		PopupTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(ui.ColorPrimary),

		InputPrompt: lipgloss.NewStyle().
			Foreground(ui.ColorPrimary),

		InputText: lipgloss.NewStyle().
			Foreground(ui.ColorTextWhite),
	}
}

// OverlayIcons defines icons used in overlay popups
var OverlayIcons = struct {
	Confirm string
	Success string
	Input   string
	Commit  string
}{
	Confirm: ui.IconConfirm,
	Success: ui.IconSuccess,
	Input:   ui.IconInput,
	Commit:  ui.IconCommit,
}

// OverlayStyles defines styles for overlay popups
var OverlayStyles = struct {
	Container lipgloss.Style
	Title     lipgloss.Style
	Content   lipgloss.Style
	Footer    lipgloss.Style
	Hint      lipgloss.Style
	Selected  lipgloss.Style
	Option    lipgloss.Style
	Separator lipgloss.Style
}{
	Container: lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColorSecondary).
		Padding(1, 2),
	Title: lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.ColorPrimary).
		MarginBottom(1),
	Content: lipgloss.NewStyle().
		Foreground(ui.ColorText),
	Footer: lipgloss.NewStyle().
		Foreground(ui.ColorMuted).
		MarginTop(1),
	Hint: lipgloss.NewStyle().
		Foreground(ui.ColorMuted),
	Selected: lipgloss.NewStyle().
		Foreground(ui.ColorPrimary).
		Bold(true),
	Option: lipgloss.NewStyle().
		Foreground(ui.ColorText),
	Separator: lipgloss.NewStyle().
		Foreground(ui.ColorBorder),
}

// OptionSelectStyles defines styles for option selection screen
var OptionSelectStyles = struct {
	// Panel styles
	LeftPanel      lipgloss.Style
	RightPanel     lipgloss.Style
	PanelTitle     lipgloss.Style
	PanelBorder    lipgloss.Style
	SummaryContent lipgloss.Style

	// Option list styles
	OptionCursor      lipgloss.Style
	OptionNormal      lipgloss.Style
	OptionRecommended lipgloss.Style
	OptionSelected    lipgloss.Style
	CheckboxChecked   string
	CheckboxUnchecked string
	RecommendedBadge  string

	// Detail panel styles
	DetailTitle       lipgloss.Style
	DetailDescription lipgloss.Style
	ProLabel          lipgloss.Style
	ConLabel          lipgloss.Style
	ProItem           lipgloss.Style
	ConItem           lipgloss.Style
}{
	LeftPanel: lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderRight(true).
		BorderForeground(ui.ColorBorder).
		Padding(0, 1),
	RightPanel: lipgloss.NewStyle().
		Padding(0, 1),
	PanelTitle: lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.ColorPrimary).
		MarginBottom(1),
	PanelBorder: lipgloss.NewStyle().
		Foreground(ui.ColorBorder),
	SummaryContent: lipgloss.NewStyle().
		Foreground(ui.ColorText),

	OptionCursor: lipgloss.NewStyle().
		Background(ui.ColorSecondary).
		Foreground(ui.ColorTextLight).
		Bold(true),
	OptionNormal: lipgloss.NewStyle().
		Foreground(ui.ColorText),
	OptionRecommended: lipgloss.NewStyle().
		Foreground(ui.ColorWarning).
		Bold(true),
	OptionSelected: lipgloss.NewStyle().
		Foreground(ui.ColorSuccess).
		Bold(true),
	CheckboxChecked:   ui.IconCheckboxChecked,
	CheckboxUnchecked: ui.IconCheckboxUnchecked,
	RecommendedBadge:  ui.IconRecommendedBadge,

	DetailTitle: lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.ColorPrimary).
		MarginBottom(1),
	DetailDescription: lipgloss.NewStyle().
		Foreground(ui.ColorText).
		MarginBottom(1),
	ProLabel: lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.ColorSuccess),
	ConLabel: lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.ColorError),
	ProItem: lipgloss.NewStyle().
		Foreground(ui.ColorText).
		PaddingLeft(2),
	ConItem: lipgloss.NewStyle().
		Foreground(ui.ColorText).
		PaddingLeft(2),
}

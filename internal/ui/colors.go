package ui

import "github.com/charmbracelet/lipgloss"

// Color palette for consistent styling across the TUI
var (
	// Primary colors
	ColorPrimary   = lipgloss.Color("212") // Pink/magenta for titles and highlights
	ColorSecondary = lipgloss.Color("62")  // Blue for selection and borders

	// Text colors
	ColorText      = lipgloss.Color("252") // Light gray for normal text
	ColorTextLight = lipgloss.Color("230") // Very light for selected items
	ColorTextWhite = lipgloss.Color("255") // White for input text

	// Border and muted colors
	ColorBorder = lipgloss.Color("240") // Gray for borders and footers
	ColorMuted  = lipgloss.Color("241") // Slightly different gray for hints

	// Semantic colors
	ColorSuccess = lipgloss.Color("46")  // Green for success/pros
	ColorError   = lipgloss.Color("196") // Red for errors/cons
	ColorWarning = lipgloss.Color("214") // Orange for warnings and processing
)

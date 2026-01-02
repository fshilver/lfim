package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/lunit-heesungyang/issue-manager/internal/model"
	"github.com/lunit-heesungyang/issue-manager/internal/ui"
)

// cursorIndicator returns a cursor string if the index matches, otherwise spaces
func cursorIndicator(currentIdx, targetIdx int) string {
	if currentIdx == targetIdx {
		return "► "
	}
	return "  "
}

// placeOverlay places the overlay centered on top of the background
func placeOverlay(width, height int, overlay, background string) string {
	overlayWidth := lipgloss.Width(overlay)
	overlayHeight := lipgloss.Height(overlay)

	x := (width - overlayWidth) / 2
	y := (height - overlayHeight) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	bgLines := strings.Split(background, "\n")
	overlayLines := strings.Split(overlay, "\n")

	// Ensure background has enough lines
	for len(bgLines) < height {
		bgLines = append(bgLines, strings.Repeat(" ", width))
	}

	// Place overlay lines onto background
	for i, overlayLine := range overlayLines {
		bgY := y + i
		if bgY >= len(bgLines) {
			break
		}

		bgLine := bgLines[bgY]
		bgRunes := []rune(bgLine)

		// Pad background line if needed
		for len(bgRunes) < width {
			bgRunes = append(bgRunes, ' ')
		}

		// Build new line: before overlay + overlay + after overlay
		var newLine strings.Builder

		// Part before overlay
		currentWidth := 0
		runeIdx := 0
		for runeIdx < len(bgRunes) && currentWidth < x {
			r := bgRunes[runeIdx]
			rw := runewidth.RuneWidth(r)
			if currentWidth+rw > x {
				// Partial character, add spaces
				for currentWidth < x {
					newLine.WriteRune(' ')
					currentWidth++
				}
				break
			}
			newLine.WriteRune(r)
			currentWidth += rw
			runeIdx++
		}

		// Pad to x if needed
		for currentWidth < x {
			newLine.WriteRune(' ')
			currentWidth++
		}

		// Overlay content
		newLine.WriteString(overlayLine)
		currentWidth += runewidth.StringWidth(overlayLine)

		// Part after overlay
		afterX := x + overlayWidth
		bgWidth := 0
		for idx, r := range bgRunes {
			rw := runewidth.RuneWidth(r)
			if bgWidth+rw > afterX {
				// Start copying from here
				for j := idx; j < len(bgRunes); j++ {
					newLine.WriteRune(bgRunes[j])
				}
				break
			}
			bgWidth += rw
		}

		bgLines[bgY] = newLine.String()
	}

	return strings.Join(bgLines, "\n")
}

// renderBaseOverlay creates a consistently styled overlay popup
// title: The header text for the overlay
// content: The main content (can include viewport, input fields, etc.)
// footer: The hint/action keys shown at the bottom
// width: The desired width of the overlay (0 for auto)
func (m Model) renderBaseOverlay(title, content, footer string, width int) string {
	// Calculate width based on terminal size if not specified
	if width == 0 {
		width = m.width - 10
	}
	if width < 40 {
		width = 40
	}
	if width > 100 {
		width = 100
	}

	// Build the overlay content
	var parts []string

	// Title section
	if title != "" {
		parts = append(parts, OverlayStyles.Title.Render(title))
	}

	// Content section
	if content != "" {
		parts = append(parts, OverlayStyles.Content.Render(content))
	}

	// Footer section (action hints)
	if footer != "" {
		parts = append(parts, OverlayStyles.Footer.Render(footer))
	}

	innerContent := strings.Join(parts, "\n")

	return OverlayStyles.Container.Width(width).Render(innerContent)
}

func (m Model) renderInputOverlay() string {
	// Determine title based on input mode
	var title string
	switch m.inputMode {
	case InputNewIssue:
		title = fmt.Sprintf("%s New Issue", OverlayIcons.Input)
	case InputReview:
		title = "Review Feedback"
	case InputPlanReview:
		title = "Plan Feedback"
	case InputChangeReason:
		title = "Update Change Log"
	default:
		title = "Input"
	}

	// For review modes with content preview
	if (m.inputMode == InputReview && m.reviewAnalysis != "") ||
		(m.inputMode == InputPlanReview && m.reviewPlan != "") {
		// Calculate width based on terminal size
		popupWidth := m.width - 10
		if popupWidth < 60 {
			popupWidth = 60
		}
		if popupWidth > 100 {
			popupWidth = 100
		}

		// Scroll indicator
		scrollPercent := m.viewport.ScrollPercent() * 100
		scrollInfo := fmt.Sprintf(" %3.0f%% ", scrollPercent)

		// Build content with viewport and input
		separator := OverlayStyles.Separator.Render(strings.Repeat("─", popupWidth-10))
		inputSection := fmt.Sprintf("%s\n%s",
			m.styles.InputPrompt.Render(m.inputPrompt),
			m.textInput.View(),
		)

		content := fmt.Sprintf("%s\n%s%s\n\n%s",
			m.viewport.View(),
			separator,
			OverlayStyles.Hint.Render(scrollInfo),
			inputSection,
		)

		footer := "[Enter] Submit    [Esc] Back"

		return m.renderBaseOverlay(title, content, footer, popupWidth)
	}

	// Simple input overlay (e.g., new issue title)
	content := fmt.Sprintf("%s\n%s",
		m.styles.InputPrompt.Render(m.inputPrompt),
		m.textInput.View(),
	)

	footer := "[Enter] Submit    [Esc] Cancel"

	return m.renderBaseOverlay(title, content, footer, 60)
}

func (m Model) renderConfirmOverlay() string {
	// Build content with icon
	content := fmt.Sprintf("%s %s", OverlayIcons.Confirm, m.confirmMsg)

	// Footer with action hints
	footer := "[y] Yes    [n] No    [Esc] Cancel"

	return m.renderBaseOverlay("Confirm", content, footer, 50)
}

func (m Model) renderTypeSelectOverlay() string {
	// Build options with icons on separate lines
	options := fmt.Sprintf("  [f] Feature   %s\n  [b] Bug       %s\n  [r] Refactor  %s",
		model.TypeFeature.Icon(),
		model.TypeBug.Icon(),
		model.TypeRefactor.Icon(),
	)

	// Footer with cancel hint
	footer := "[Esc] Cancel"

	return m.renderBaseOverlay("Select Issue Type", options, footer, 40)
}

func (m Model) renderModelSelectOverlay() string {
	issueID := ""
	if m.pendingRetryIssue != nil {
		issueID = m.pendingRetryIssue.ID
	}

	// Build options with cursor indicator
	options := fmt.Sprintf(
		"%s[o] Opus   - Highest quality, slower\n"+
			"%s[s] Sonnet - Balanced performance\n"+
			"%s[h] Haiku  - Fast, cost-effective (Default)",
		cursorIndicator(m.modelCursor, 0),
		cursorIndicator(m.modelCursor, 1),
		cursorIndicator(m.modelCursor, 2),
	)

	// Warning about code modification
	content := options + "\n\nThis will modify code files."

	// Footer with navigation hints
	footer := "[Enter] Start    [j/k] Navigate    [Esc] Cancel"

	// Header includes issue ID for context
	header := fmt.Sprintf("Implement %s - Select Model", issueID)

	return m.renderBaseOverlay(header, content, footer, 50)
}

func (m Model) renderReviewPreviewOverlay() string {
	// Calculate width based on terminal size
	popupWidth := m.width - 10
	if popupWidth < 60 {
		popupWidth = 60
	}
	if popupWidth > 100 {
		popupWidth = 100
	}

	// Scroll indicators
	scrollPercent := m.viewport.ScrollPercent() * 100
	scrollInfo := fmt.Sprintf(" V:%3.0f%% ", scrollPercent)

	// Horizontal scroll indicator
	hScrollInfo := ""
	if m.maxLineWidth > m.viewport.Width {
		hScrollInfo = fmt.Sprintf(" H:%d ", m.hOffset)
	}

	// Build content with viewport and scroll info
	separator := OverlayStyles.Separator.Render(strings.Repeat("─", popupWidth-10))
	scrollHints := OverlayStyles.Hint.Render(scrollInfo + hScrollInfo)
	content := fmt.Sprintf("%s\n%s%s", m.viewport.View(), separator, scrollHints)

	// Footer with action hints
	footer := "[e] Edit    [f] Feedback    [c] Close    ↑↓ Scroll    ←→ Pan"

	return m.renderBaseOverlay("Review Analysis", content, footer, popupWidth)
}

func (m Model) renderPlanPreviewOverlay() string {
	// Calculate width based on terminal size
	popupWidth := m.width - 10
	if popupWidth < 60 {
		popupWidth = 60
	}
	if popupWidth > 100 {
		popupWidth = 100
	}

	// Scroll indicators
	scrollPercent := m.viewport.ScrollPercent() * 100
	scrollInfo := fmt.Sprintf(" V:%3.0f%% ", scrollPercent)

	// Horizontal scroll indicator
	hScrollInfo := ""
	if m.maxLineWidth > m.viewport.Width {
		hScrollInfo = fmt.Sprintf(" H:%d ", m.hOffset)
	}

	// Build content with viewport and scroll info
	separator := OverlayStyles.Separator.Render(strings.Repeat("─", popupWidth-10))
	scrollHints := OverlayStyles.Hint.Render(scrollInfo + hScrollInfo)
	content := fmt.Sprintf("%s\n%s%s", m.viewport.View(), separator, scrollHints)

	// Footer with action hints
	footer := "[e] Edit    [f] Feedback    [c] Close    ↑↓ Scroll    ←→ Pan"

	return m.renderBaseOverlay("Review Plan", content, footer, popupWidth)
}

func (m Model) renderCommitConfirmOverlay() string {
	// Calculate width based on terminal size
	popupWidth := m.width - 10
	if popupWidth < 60 {
		popupWidth = 60
	}
	if popupWidth > 100 {
		popupWidth = 100
	}

	// Setup viewport for commit message if not already set
	viewportHeight := m.height - 10
	if viewportHeight < 5 {
		viewportHeight = 5
	}
	if viewportHeight > 20 {
		viewportHeight = 20
	}

	// Wrap commit message for display
	wrapped := wrapText(m.pendingCommitMsg, popupWidth-10)
	lines := strings.Split(wrapped, "\n")
	if len(lines) > viewportHeight {
		lines = lines[:viewportHeight]
	}
	displayMsg := strings.Join(lines, "\n")

	// Build title with issue info
	title := fmt.Sprintf("%s Commit Message", OverlayIcons.Commit)
	if m.pendingCloseIssue != nil {
		title = fmt.Sprintf("%s Commit [%s]", OverlayIcons.Commit, m.pendingCloseIssue.ID)
	}

	// Content with separator and message
	separator := OverlayStyles.Separator.Render(strings.Repeat("─", popupWidth-10))
	content := fmt.Sprintf("%s\n\n%s", separator, displayMsg)

	footer := "[y] Commit    [n] Cancel    ↑↓ Scroll"

	return m.renderBaseOverlay(title, content, footer, popupWidth)
}

func (m Model) renderCommitGeneratingOverlay() string {
	spinner := ui.SpinnerFrames[m.spinnerFrame]

	// Build title with issue info
	title := "Closing Issue"
	if m.pendingCloseIssue != nil {
		title = fmt.Sprintf("Closing Issue [%s]", m.pendingCloseIssue.ID)
	}

	// Content with spinner animation
	content := fmt.Sprintf("%s  Generating commit message...", spinner)

	// No footer during processing (input is blocked)
	footer := OverlayStyles.Hint.Render("Please wait...")

	return m.renderBaseOverlay(title, content, footer, 50)
}

func (m Model) renderUncommittedChangesErrorOverlay() string {
	// Build error message with warning icon
	title := fmt.Sprintf("%s Implementation Blocked", OverlayIcons.Error)

	// Content explaining the issue
	content := "Cannot start new implementation.\n\n" +
		"You have uncommitted changes in your repository.\n" +
		"Please commit or stash your changes before starting\n" +
		"a new implementation to avoid mixing code changes\n" +
		"from different issues.\n\n" +
		"Actions:\n" +
		"  • Run 'git status' to see uncommitted changes\n" +
		"  • Commit changes: 'git add . && git commit'\n" +
		"  • Or stash changes: 'git stash'"

	// Footer with dismiss hint
	footer := "[Enter/Esc] Dismiss"

	return m.renderBaseOverlay(title, content, footer, 60)
}

func (m Model) renderStagedChangesErrorOverlay() string {
	// Build error message with error icon
	title := fmt.Sprintf("%s Action Blocked", OverlayIcons.Error)

	// Build list of staged files
	fileList := ""
	maxFiles := 10
	for i, file := range m.stagedFiles {
		if i >= maxFiles {
			remaining := len(m.stagedFiles) - maxFiles
			fileList += fmt.Sprintf("  ... and %d more file(s)\n", remaining)
			break
		}
		fileList += fmt.Sprintf("  • %s\n", file)
	}

	// Content explaining the issue
	content := "Cannot proceed with this action.\n\n" +
		"You have staged changes in your repository.\n" +
		"Please commit or unstage them first to avoid\n" +
		"mixing code changes from different issues.\n\n" +
		"Staged files:\n" +
		fileList + "\n" +
		"Actions:\n" +
		"  • Commit changes: 'git commit -m \"message\"'\n" +
		"  • Unstage changes: 'git reset HEAD'\n" +
		"  • Or stash changes: 'git stash'"

	// Footer with dismiss hint
	footer := "[Enter/Esc] Dismiss"

	return m.renderBaseOverlay(title, content, footer, 65)
}

func (m Model) renderStagedChangesWarningOverlay() string {
	// Build warning message with warning icon
	title := fmt.Sprintf("%s Staged Changes Detected", OverlayIcons.Confirm)

	// Build list of staged files
	fileList := ""
	maxFiles := 10
	for i, file := range m.stagedFiles {
		if i >= maxFiles {
			remaining := len(m.stagedFiles) - maxFiles
			fileList += fmt.Sprintf("  ... and %d more file(s)\n", remaining)
			break
		}
		fileList += fmt.Sprintf("  • %s\n", file)
	}

	// Content explaining the warning with option to proceed
	content := "You have staged changes in your repository.\n\n" +
		"Staged files:\n" +
		fileList + "\n" +
		"WARNING: Proceeding will mix these staged changes\n" +
		"with implementation changes. This may make it harder\n" +
		"to separate changes later.\n\n" +
		"Recommended actions:\n" +
		"  • Commit staged changes first: 'git commit -m \"...\"'\n" +
		"  • Or unstage them: 'git reset HEAD'\n\n" +
		"Do you want to proceed anyway?"

	// Footer with proceed/cancel options
	footer := "[y] Proceed Anyway    [n/Esc] Cancel"

	return m.renderBaseOverlay(title, content, footer, 65)
}

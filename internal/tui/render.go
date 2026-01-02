package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/lunit-heesungyang/issue-manager/internal/model"
	"github.com/lunit-heesungyang/issue-manager/internal/ui"
)

// View implements tea.Model
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	// Handle full-screen states
	if m.state == StateOptionSelect {
		return m.renderOptionSelectView()
	}

	// Calculate layout - reserve lines for header(1) + footer(1) + status(1) + optional warning(1)
	reservedLines := 3
	if m.gitWarning != "" {
		reservedLines = 4
	}
	listWidth := m.width / 2
	previewWidth := m.width - listWidth
	contentHeight := m.height - reservedLines
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Render header with scroll indicator
	headerText := fmt.Sprintf("Issue Manager [%s]", m.filterMode)
	// Add scroll indicator if there are more issues than visible
	if len(m.issues) > contentHeight {
		scrollInfo := fmt.Sprintf(" (%d-%d/%d)", m.listVOffset+1, min(m.listVOffset+contentHeight, len(m.issues)), len(m.issues))
		headerText += scrollInfo
	}
	// Add horizontal scroll indicator if scrolled
	if m.listHOffset > 0 {
		headerText += fmt.Sprintf(" H:%d", m.listHOffset)
	}
	header := m.styles.Header.Render(headerText)

	// Render list panel
	listContent := m.renderList(listWidth-2, contentHeight)
	listPanel := m.styles.ListPanel.
		Width(listWidth).
		Render(listContent)

	// Render preview panel
	previewContent := m.renderPreview(previewWidth-4, contentHeight)
	previewPanel := m.styles.PreviewPanel.
		Width(previewWidth).
		Render(previewContent)

	// Combine panels horizontally
	content := lipgloss.JoinHorizontal(lipgloss.Top, listPanel, previewPanel)

	// Render footer
	keys := "[n]ew [a]nalyze [R]eview [p]lan [P]lan-review [i]mplement [u]pdate-log [c]lose [d]iscard [e]dit [f]ilter [q]uit"
	footer := m.styles.Footer.Render(keys)
	status := m.styles.StatusBar.Render(m.statusMsg)

	// Render git warning banner if present
	var gitWarningBanner string
	if m.gitWarning != "" {
		gitWarningBanner = m.styles.Warning.Render(m.gitWarning)
	}

	// Handle special states
	var overlay string
	switch m.state {
	case StateInput:
		overlay = m.renderInputOverlay()
	case StateConfirm:
		overlay = m.renderConfirmOverlay()
	case StateTypeSelect:
		overlay = m.renderTypeSelectOverlay()
	case StateModelSelect:
		overlay = m.renderModelSelectOverlay()
	case StateReviewPreview:
		overlay = m.renderReviewPreviewOverlay()
	case StatePlanPreview:
		overlay = m.renderPlanPreviewOverlay()
	case StateCommitConfirm:
		overlay = m.renderCommitConfirmOverlay()
	case StateCommitGenerating:
		overlay = m.renderCommitGeneratingOverlay()
	case StateUncommittedChangesError:
		overlay = m.renderUncommittedChangesErrorOverlay()
	case StateStagedChangesError:
		overlay = m.renderStagedChangesErrorOverlay()
	case StateStagedChangesWarning:
		overlay = m.renderStagedChangesWarningOverlay()
	}

	// Combine vertically
	parts := []string{header, content, footer, status}
	if gitWarningBanner != "" {
		parts = append(parts, gitWarningBanner)
	}
	view := lipgloss.JoinVertical(lipgloss.Left, parts...)

	// Force exact terminal height to prevent scrolling issues
	lines := strings.Split(view, "\n")
	if len(lines) > m.height {
		lines = lines[:m.height]
	}
	for len(lines) < m.height {
		lines = append(lines, "")
	}

	view = strings.Join(lines, "\n")

	// Overlay popup on top of background
	if overlay != "" {
		view = placeOverlay(m.width, m.height, overlay, view)
	}

	return view
}

func (m Model) renderList(width, height int) string {
	var lines []string

	if len(m.issues) == 0 {
		lines = append(lines, fmt.Sprintf("No %s issues", m.filterMode))
	} else {
		// Calculate the visible range based on vertical scroll offset
		startIdx := m.listVOffset
		endIdx := m.listVOffset + height
		if endIdx > len(m.issues) {
			endIdx = len(m.issues)
		}

		for i := startIdx; i < endIdx; i++ {
			issue := m.issues[i]

			// Get icon
			var icon string
			m.processingLock.Lock()
			taskType, isProcessing := m.processing[issue.ID]
			m.processingLock.Unlock()

			if isProcessing {
				icon = ui.SpinnerFrames[m.spinnerFrame]
			} else {
				icon = issue.StatusIcon()
			}

			// Format line
			typeIcon := issue.Type.Icon()
			var suffix string
			if isProcessing {
				suffix = fmt.Sprintf(" [%s...]", taskType)
			}
			line := fmt.Sprintf("%s %s [%s] %s%s", typeIcon, icon, issue.ID, issue.Title, suffix)

			// Apply horizontal scroll offset
			if m.listHOffset > 0 {
				line = applyHorizontalOffsetToLine(line, m.listHOffset)
			}

			// Truncate using display width (handles wide chars like Korean)
			if runewidth.StringWidth(line) > width {
				line = runewidth.Truncate(line, width-3, "...")
			}

			// Style
			if i == m.selected {
				line = m.styles.SelectedItem.Render(line)
			} else if isProcessing {
				line = m.styles.ProcessingItem.Render(line)
			}

			lines = append(lines, line)
		}
	}

	// Pad to fill height to prevent layout shifts
	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderPreview(width, height int) string {
	var lines []string

	if len(m.issues) == 0 || m.selected >= len(m.issues) {
		lines = append(lines, "No issue selected")
	} else {
		issue := m.issues[m.selected]

		// Title
		title := m.styles.PreviewTitle.Render(
			fmt.Sprintf("Preview: %s", issue.ID),
		)
		lines = append(lines, title)
		lines = append(lines, strings.Repeat("─", min(width, 40)))

		// Load content
		brief, err := m.storage.LoadBrief(issue.ID)
		var content string
		if err != nil || brief == nil {
			content = "brief.md not found"
		} else {
			content = brief.Content
			if content == "" {
				content = "(empty)"
			}
		}

		// Wrap content to width and add lines
		wrapped := wrapText(content, width)
		contentLines := strings.Split(wrapped, "\n")
		lines = append(lines, contentLines...)
	}

	// Pad to fill height to prevent layout shifts
	for len(lines) < height {
		lines = append(lines, "")
	}

	// Truncate if exceeds height
	if len(lines) > height {
		lines = lines[:height]
	}

	return strings.Join(lines, "\n")
}

// Option Selection View

func (m Model) renderOptionSelectView() string {
	if m.analysis == nil {
		return "No analysis loaded"
	}

	// Calculate layout
	// Header: 1 line, Footer: 2 lines (keys + status)
	contentHeight := m.height - 3
	if contentHeight < 5 {
		contentHeight = 5
	}

	leftWidth := m.width / 2
	rightWidth := m.width - leftWidth - 1

	// Left panel heights: summary takes top 40%, options take bottom 60%
	summaryHeight := contentHeight * 2 / 5
	if summaryHeight < 3 {
		summaryHeight = 3
	}
	optionsHeight := contentHeight - summaryHeight - 1 // -1 for separator

	// Render header
	issue := m.getSelectedIssue()
	headerText := "Option Selection"
	if issue != nil {
		headerText = fmt.Sprintf("Option Selection [%s]", issue.ID)
	}
	header := m.styles.Header.Render(headerText)

	// Render left panel: summary + options
	leftContent := m.renderLeftPanel(leftWidth-2, summaryHeight, optionsHeight)
	leftPanel := OptionSelectStyles.LeftPanel.
		Width(leftWidth).
		Height(contentHeight).
		Render(leftContent)

	// Render right panel: option detail
	rightContent := m.renderRightPanel(rightWidth-2, contentHeight)
	rightPanel := OptionSelectStyles.RightPanel.
		Width(rightWidth).
		Height(contentHeight).
		Render(rightContent)

	// Combine panels
	content := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	// Footer
	keys := "[↑/↓] Navigate  [Ctrl+↑↓←→/hjkl] Scroll Detail  [Enter] Select & Plan  [n] Add Option  [e] Edit  [Esc] Cancel"
	footer := m.styles.Footer.Render(keys)
	status := m.styles.StatusBar.Render(m.statusMsg)

	// Combine vertically
	view := lipgloss.JoinVertical(lipgloss.Left, header, content, footer, status)

	// Force exact terminal height
	lines := strings.Split(view, "\n")
	if len(lines) > m.height {
		lines = lines[:m.height]
	}
	for len(lines) < m.height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderLeftPanel(width, summaryHeight, optionsHeight int) string {
	// Summary section
	summaryTitle := OptionSelectStyles.PanelTitle.Render("Analysis Summary")
	summaryContent := m.renderSummarySection(width, summaryHeight-2)

	// Separator
	separator := OptionSelectStyles.PanelBorder.Render(strings.Repeat("─", width))

	// Options section
	optionsTitle := OptionSelectStyles.PanelTitle.Render("Implementation Options")
	optionsContent := m.renderOptionsSection(width, optionsHeight-2)

	return lipgloss.JoinVertical(lipgloss.Left,
		summaryTitle,
		summaryContent,
		separator,
		optionsTitle,
		optionsContent,
	)
}

func (m Model) renderSummarySection(width, height int) string {
	if m.analysis == nil {
		return "No analysis"
	}

	content := m.analysis.Summary
	if m.analysis.RootCause != "" {
		content += "\n\n" + m.analysis.RootCause
	}

	// Wrap text to width
	wrapped := wrapText(content, width)
	lines := strings.Split(wrapped, "\n")

	// Limit to height
	if len(lines) > height {
		lines = lines[:height-1]
		lines = append(lines, "...")
	}

	// Pad to height
	for len(lines) < height {
		lines = append(lines, "")
	}

	return OptionSelectStyles.SummaryContent.Render(strings.Join(lines, "\n"))
}

func (m Model) renderOptionsSection(width, height int) string {
	if m.analysis == nil || len(m.analysis.Options) == 0 {
		return "No options available"
	}

	var lines []string
	for i, opt := range m.analysis.Options {
		// Checkbox
		checkbox := OptionSelectStyles.CheckboxUnchecked
		if m.analysis.SelectedOptionID == opt.ID || (m.analysis.SelectedOptionID == "" && opt.Recommended) {
			checkbox = OptionSelectStyles.CheckboxChecked
		}

		// Option text
		optionText := fmt.Sprintf("%s %s", checkbox, opt.Title)

		// Add recommended badge
		if opt.Recommended {
			optionText += " " + OptionSelectStyles.RecommendedBadge
		}

		// Truncate to width
		if runewidth.StringWidth(optionText) > width {
			optionText = runewidth.Truncate(optionText, width-3, "...")
		}

		// Style based on cursor position
		if i == m.optionCursor {
			optionText = OptionSelectStyles.OptionCursor.Render(optionText)
		} else if opt.Recommended {
			optionText = OptionSelectStyles.OptionRecommended.Render(optionText)
		} else {
			optionText = OptionSelectStyles.OptionNormal.Render(optionText)
		}

		lines = append(lines, optionText)
	}

	// Pad to height
	for len(lines) < height {
		lines = append(lines, "")
	}

	// Limit to height
	if len(lines) > height {
		lines = lines[:height]
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderRightPanel(width, height int) string {
	// Build title with scroll indicators
	titleText := "Option Details"

	// Add scroll indicators
	scrollInfo := ""
	if m.detailViewport.TotalLineCount() > m.detailViewport.Height {
		scrollPercent := m.detailViewport.ScrollPercent() * 100
		scrollInfo += fmt.Sprintf(" V:%3.0f%%", scrollPercent)
	}
	if m.detailHOffset > 0 {
		scrollInfo += fmt.Sprintf(" H:%d", m.detailHOffset)
	}
	if scrollInfo != "" {
		titleText += OverlayStyles.Hint.Render(scrollInfo)
	}

	title := OptionSelectStyles.PanelTitle.Render(titleText)

	if m.analysis == nil || m.optionCursor >= len(m.analysis.Options) {
		return title + "\nNo option selected"
	}

	// Update viewport dimensions for current panel size
	viewportHeight := height - 3 // minus title and hint
	if viewportHeight < 1 {
		viewportHeight = 1
	}
	m.detailViewport.Width = width
	m.detailViewport.Height = viewportHeight

	// Get viewport content (respects scroll position)
	viewportContent := m.detailViewport.View()

	// Apply horizontal scroll offset
	if m.detailHOffset > 0 {
		viewportContent = applyHorizontalOffset(viewportContent, m.detailHOffset, width)
	}

	// Add scroll hint at bottom
	hint := OverlayStyles.Hint.Render("[Ctrl+↑↓←→/hjkl] Scroll detail")

	return title + "\n" + viewportContent + "\n" + hint
}

func (m Model) renderOptionDetail(option *model.AnalysisOption) string {
	var sb strings.Builder

	// Title
	sb.WriteString(OptionSelectStyles.DetailTitle.Render(option.Title))
	sb.WriteString("\n\n")

	// Description
	if option.Description != "" {
		sb.WriteString(OptionSelectStyles.DetailDescription.Render(option.Description))
		sb.WriteString("\n\n")
	}

	// Pros
	if len(option.Pros) > 0 {
		sb.WriteString(OptionSelectStyles.ProLabel.Render("Pros:"))
		sb.WriteString("\n")
		for _, pro := range option.Pros {
			sb.WriteString(OptionSelectStyles.ProItem.Render("+ " + pro))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Cons
	if len(option.Cons) > 0 {
		sb.WriteString(OptionSelectStyles.ConLabel.Render("Cons:"))
		sb.WriteString("\n")
		for _, con := range option.Cons {
			sb.WriteString(OptionSelectStyles.ConItem.Render("- " + con))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Details
	if option.Details != "" {
		sb.WriteString("Details:\n")
		sb.WriteString(option.Details)
	}

	return sb.String()
}

// Helper functions

func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}

		// Wrap using display width (handles wide chars like Korean)
		for runewidth.StringWidth(line) > width {
			// Find the wrap point
			wrapped := runewidth.Truncate(line, width, "")
			result.WriteString(wrapped)
			result.WriteString("\n")
			line = line[len(wrapped):]
		}
		result.WriteString(line)
	}

	return result.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// calculateMaxLineWidth returns the maximum display width of any line in the content
func calculateMaxLineWidth(content string) int {
	maxWidth := 0
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		w := runewidth.StringWidth(line)
		if w > maxWidth {
			maxWidth = w
		}
	}
	return maxWidth
}

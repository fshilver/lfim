package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"

	"github.com/lunit-heesungyang/issue-manager/internal/model"
)

func (m Model) handleReviewPreviewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Horizontal scroll step size
	const hScrollStep = 10

	switch msg.String() {
	// Vertical scroll keys
	case "up", "k":
		m.viewport.LineUp(1)
		return m, nil
	case "down", "j":
		m.viewport.LineDown(1)
		return m, nil
	case "pgup", "ctrl+u":
		m.viewport.HalfViewUp()
		return m, nil
	case "pgdown", "ctrl+d":
		m.viewport.HalfViewDown()
		return m, nil
	case "home", "g":
		m.viewport.GotoTop()
		return m, nil
	case "end", "G":
		m.viewport.GotoBottom()
		return m, nil

	// Horizontal scroll keys
	case "left", "h":
		if m.hOffset > 0 {
			m.hOffset -= hScrollStep
			if m.hOffset < 0 {
				m.hOffset = 0
			}
			// Update viewport content with new offset
			yOffset := m.viewport.YOffset
			m.viewport.SetContent(applyHorizontalOffset(m.reviewAnalysis, m.hOffset, m.viewport.Width))
			m.viewport.SetYOffset(yOffset)
		}
		return m, nil
	case "right", "l":
		maxOffset := m.maxLineWidth - m.viewport.Width
		if maxOffset < 0 {
			maxOffset = 0
		}
		if m.hOffset < maxOffset {
			m.hOffset += hScrollStep
			if m.hOffset > maxOffset {
				m.hOffset = maxOffset
			}
			// Update viewport content with new offset
			yOffset := m.viewport.YOffset
			m.viewport.SetContent(applyHorizontalOffset(m.reviewAnalysis, m.hOffset, m.viewport.Width))
			m.viewport.SetYOffset(yOffset)
		}
		return m, nil

	// Action keys
	case "e":
		if m.isViewOnly {
			m.statusMsg = "Read-only: cannot edit closed issue"
			return m, nil
		}
		return m.editAnalysis()
	case "f":
		if m.isViewOnly {
			m.statusMsg = "Read-only: cannot provide feedback for closed issue"
			return m, nil
		}
		// Switch to feedback input mode
		m.state = StateInput
		m.inputMode = InputReview
		m.inputPrompt = "Feedback: "
		m.textInput.Focus()
		return m, textinput.Blink
	case "c", "esc":
		m.state = StateNormal
		m.reviewAnalysis = ""
		m.hOffset = 0
		m.isViewOnly = false
		return m, nil
	}

	return m, nil
}

func (m Model) handlePlanPreviewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Horizontal scroll step size
	const hScrollStep = 10

	switch msg.String() {
	// Vertical scroll keys
	case "up", "k":
		m.viewport.LineUp(1)
		return m, nil
	case "down", "j":
		m.viewport.LineDown(1)
		return m, nil
	case "pgup", "ctrl+u":
		m.viewport.HalfViewUp()
		return m, nil
	case "pgdown", "ctrl+d":
		m.viewport.HalfViewDown()
		return m, nil
	case "home", "g":
		m.viewport.GotoTop()
		return m, nil
	case "end", "G":
		m.viewport.GotoBottom()
		return m, nil

	// Horizontal scroll keys
	case "left", "h":
		if m.hOffset > 0 {
			m.hOffset -= hScrollStep
			if m.hOffset < 0 {
				m.hOffset = 0
			}
			// Update viewport content with new offset
			yOffset := m.viewport.YOffset
			m.viewport.SetContent(applyHorizontalOffset(m.reviewPlan, m.hOffset, m.viewport.Width))
			m.viewport.SetYOffset(yOffset)
		}
		return m, nil
	case "right", "l":
		maxOffset := m.maxLineWidth - m.viewport.Width
		if maxOffset < 0 {
			maxOffset = 0
		}
		if m.hOffset < maxOffset {
			m.hOffset += hScrollStep
			if m.hOffset > maxOffset {
				m.hOffset = maxOffset
			}
			// Update viewport content with new offset
			yOffset := m.viewport.YOffset
			m.viewport.SetContent(applyHorizontalOffset(m.reviewPlan, m.hOffset, m.viewport.Width))
			m.viewport.SetYOffset(yOffset)
		}
		return m, nil

	// Action keys
	case "e":
		if m.isViewOnly {
			m.statusMsg = "Read-only: cannot edit closed issue"
			return m, nil
		}
		return m.editPlan()
	case "f":
		if m.isViewOnly {
			m.statusMsg = "Read-only: cannot provide feedback for closed issue"
			return m, nil
		}
		// Switch to feedback input mode
		m.state = StateInput
		m.inputMode = InputPlanReview
		m.inputPrompt = "Plan Feedback: "
		m.textInput.Focus()
		return m, textinput.Blink
	case "c", "esc":
		m.state = StateNormal
		m.reviewPlan = ""
		m.hOffset = 0
		m.isViewOnly = false
		return m, nil
	}

	return m, nil
}

func (m Model) handleCommitConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	// Scroll keys
	case "up", "k":
		m.viewport.LineUp(1)
		return m, nil
	case "down", "j":
		m.viewport.LineDown(1)
		return m, nil

	// Accept commit
	case "y", "enter":
		if m.pendingCloseIssue == nil {
			m.state = StateNormal
			m.statusMsg = "No pending issue"
			return m, nil
		}

		issue := m.pendingCloseIssue
		_ = m.storage.UpdateIssueStatus(issue.ID, model.StatusClosed, "")

		if m.storage.HasStagedChanges() {
			success, _ := m.storage.GitCommit(m.pendingCommitMsg)
			if success {
				m.statusMsg = fmt.Sprintf("Closed & committed %s", issue.ID)
			} else {
				m.statusMsg = fmt.Sprintf("Closed %s (commit failed)", issue.ID)
			}
		} else {
			m.statusMsg = fmt.Sprintf("Closed %s (no changes to commit)", issue.ID)
		}

		m.state = StateNormal
		m.pendingCloseIssue = nil
		m.pendingCommitMsg = ""
		return m, m.refreshIssues()

	// Cancel
	case "n", "esc":
		m.state = StateNormal
		m.pendingCloseIssue = nil
		m.pendingCommitMsg = ""
		m.statusMsg = "Cancelled"
		return m, nil
	}

	return m, nil
}

// applyHorizontalOffset applies horizontal scrolling to content
// It returns content where each line is shifted by offset and truncated to width
func applyHorizontalOffset(content string, offset int, width int) string {
	if offset <= 0 && width <= 0 {
		return content
	}

	lines := strings.Split(content, "\n")
	result := make([]string, len(lines))

	for i, line := range lines {
		lineWidth := runewidth.StringWidth(line)

		// If offset is beyond the line, return empty line
		if offset >= lineWidth {
			result[i] = ""
			continue
		}

		// Skip characters until we reach the offset
		currentWidth := 0
		startIdx := 0
		for _, r := range line {
			charWidth := runewidth.RuneWidth(r)
			if currentWidth+charWidth > offset {
				break
			}
			currentWidth += charWidth
			startIdx += len(string(r))
		}

		// Get the substring from offset position
		remaining := line[startIdx:]

		// If we stopped in the middle of a wide character, add a space
		if currentWidth < offset {
			remaining = " " + remaining
		}

		// Truncate to width
		if width > 0 {
			remaining = runewidth.Truncate(remaining, width, "")
		}

		result[i] = remaining
	}

	return strings.Join(result, "\n")
}

// applyHorizontalOffsetToLine applies horizontal scroll offset to a single line
func applyHorizontalOffsetToLine(line string, offset int) string {
	lineWidth := runewidth.StringWidth(line)

	// If offset is beyond the line, return empty string
	if offset >= lineWidth {
		return ""
	}

	// Skip characters until we reach the offset
	currentWidth := 0
	startIdx := 0
	for _, r := range line {
		charWidth := runewidth.RuneWidth(r)
		if currentWidth+charWidth > offset {
			break
		}
		currentWidth += charWidth
		startIdx += len(string(r))
	}

	// Get the substring from offset position
	remaining := line[startIdx:]

	// If we stopped in the middle of a wide character, add a space
	if currentWidth < offset {
		remaining = " " + remaining
	}

	return remaining
}

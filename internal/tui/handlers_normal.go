package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Calculate visible height for list (same as in View)
	listVisibleHeight := m.height - 3
	if listVisibleHeight < 1 {
		listVisibleHeight = 1
	}

	// Horizontal scroll step size
	const hScrollStep = 5

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Up):
		if m.selected > 0 {
			m.selected--
			m.ensureSelectedVisible(listVisibleHeight)
		}
		return m, nil

	case key.Matches(msg, m.keys.Down):
		if m.selected < len(m.issues)-1 {
			m.selected++
			m.ensureSelectedVisible(listVisibleHeight)
		}
		return m, nil

	case key.Matches(msg, m.keys.New):
		return m.startNewIssue()

	case key.Matches(msg, m.keys.Edit):
		return m.editIssue()

	case key.Matches(msg, m.keys.Close):
		return m.confirmClose()

	case key.Matches(msg, m.keys.Discard):
		return m.confirmDiscard()

	case key.Matches(msg, m.keys.Analyze):
		return m.analyzeIssue()

	case key.Matches(msg, m.keys.Plan):
		return m.planIssue()

	case key.Matches(msg, m.keys.Review):
		return m.reviewIssue()

	case key.Matches(msg, m.keys.PlanReview):
		return m.planReviewIssue()

	case key.Matches(msg, m.keys.Implement):
		return m.implementIssue()

	case key.Matches(msg, m.keys.UpdateLog):
		return m.updateChangeLog()

	case key.Matches(msg, m.keys.Refresh):
		m.statusMsg = "Refreshed"
		return m, tea.Batch(m.refreshIssues(), m.updateGitStatusCmd())

	case key.Matches(msg, m.keys.Filter):
		m.filterMode = (m.filterMode + 1) % 3
		m.listVOffset = 0 // Reset vertical scroll on filter change
		m.listHOffset = 0 // Reset horizontal scroll on filter change
		m.statusMsg = fmt.Sprintf("Filter: %s", m.filterMode)
		return m, tea.Batch(m.refreshIssues(), m.updateGitStatusCmd())
	}

	// Handle horizontal scroll with left/right arrow keys
	listWidth := m.width/2 - 2
	switch msg.String() {
	case "left":
		if m.listHOffset > 0 {
			m.listHOffset -= hScrollStep
			if m.listHOffset < 0 {
				m.listHOffset = 0
			}
		}
		return m, nil
	case "right":
		maxOffset := m.listMaxLineWidth - listWidth
		if maxOffset < 0 {
			maxOffset = 0
		}
		if m.listHOffset < maxOffset {
			m.listHOffset += hScrollStep
			if m.listHOffset > maxOffset {
				m.listHOffset = maxOffset
			}
		}
		return m, nil
	}

	return m, nil
}

func (m *Model) ensureSelectedVisible(visibleHeight int) {
	if len(m.issues) == 0 || visibleHeight <= 0 {
		return
	}

	// Clamp selected to valid range
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= len(m.issues) {
		m.selected = len(m.issues) - 1
	}

	// If selected is above the visible area, scroll up
	if m.selected < m.listVOffset {
		m.listVOffset = m.selected
	}

	// If selected is below the visible area, scroll down
	if m.selected >= m.listVOffset+visibleHeight {
		m.listVOffset = m.selected - visibleHeight + 1
	}

	// Clamp listVOffset to valid range
	maxOffset := len(m.issues) - visibleHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.listVOffset > maxOffset {
		m.listVOffset = maxOffset
	}
	if m.listVOffset < 0 {
		m.listVOffset = 0
	}
}

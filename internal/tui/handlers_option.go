package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleOptionSelectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.analysis == nil || len(m.analysis.Options) == 0 {
		m.state = StateNormal
		m.statusMsg = "No options available"
		return m, nil
	}

	// Horizontal scroll step size for detail panel
	const detailHScrollStep = 5

	switch msg.String() {
	// Navigation (option selection)
	case "up", "k":
		if m.optionCursor > 0 {
			m.optionCursor--
			m.updateDetailViewport()
		}
		return m, nil
	case "down", "j":
		if m.optionCursor < len(m.analysis.Options)-1 {
			m.optionCursor++
			m.updateDetailViewport()
		}
		return m, nil

	// Scroll detail viewport - vertical (Ctrl + up/down/j/k)
	case "ctrl+up", "ctrl+k":
		m.detailViewport.LineUp(1)
		return m, nil
	case "ctrl+down", "ctrl+j":
		m.detailViewport.LineDown(1)
		return m, nil

	// Half page scroll
	case "ctrl+u":
		m.detailViewport.HalfViewUp()
		return m, nil
	case "ctrl+d":
		m.detailViewport.HalfViewDown()
		return m, nil

	// Scroll detail viewport - horizontal (Ctrl + left/right/h/l)
	case "ctrl+left", "ctrl+h":
		if m.detailHOffset > 0 {
			m.detailHOffset -= detailHScrollStep
			if m.detailHOffset < 0 {
				m.detailHOffset = 0
			}
		}
		return m, nil
	case "ctrl+right", "ctrl+l":
		maxOffset := m.detailMaxLineWidth - m.detailViewport.Width
		if maxOffset < 0 {
			maxOffset = 0
		}
		if m.detailHOffset < maxOffset {
			m.detailHOffset += detailHScrollStep
			if m.detailHOffset > maxOffset {
				m.detailHOffset = maxOffset
			}
		}
		return m, nil

	// Select and proceed to plan
	case "enter":
		if m.isViewOnly {
			m.statusMsg = "Read-only: cannot select option for closed issue"
			return m, nil
		}
		selectedOption := m.analysis.Options[m.optionCursor]
		issue := m.getSelectedIssue()
		if issue == nil {
			m.state = StateNormal
			m.statusMsg = "No issue selected"
			return m, nil
		}

		// Save the selected option
		if err := m.storage.UpdateSelectedOption(issue.ID, selectedOption.ID); err != nil {
			m.statusMsg = fmt.Sprintf("Failed to save selection: %v", err)
			return m, nil
		}

		// Proceed to plan
		m.state = StateNormal
		m.statusMsg = fmt.Sprintf("Selected: %s - proceeding to plan", selectedOption.Title)
		return m.executePlanWithOption(issue)

	// Add new option
	case "n":
		if m.isViewOnly {
			m.statusMsg = "Read-only: cannot add option for closed issue"
			return m, nil
		}
		m.state = StateInput
		m.inputMode = InputAddOption
		m.inputPrompt = "Describe your approach: "
		m.textInput.Focus()
		return m, nil

	// Edit analysis.json in external editor
	case "e":
		if m.isViewOnly {
			m.statusMsg = "Read-only: cannot edit closed issue"
			return m, nil
		}
		return m.editAnalysisJSON()

	// Cancel
	case "esc", "q":
		m.state = StateNormal
		m.analysis = nil
		m.isViewOnly = false
		m.statusMsg = "Cancelled option selection"
		return m, nil
	}

	return m, nil
}

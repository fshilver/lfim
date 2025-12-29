package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lunit-heesungyang/issue-manager/internal/model"
)

// availableModels is the list of AI models available for selection
var availableModels = []AIModel{ModelOpus, ModelSonnet, ModelHaiku}

func (m Model) handleInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		value := m.textInput.Value()
		m.textInput.Reset()

		// Handle based on input mode
		switch m.inputMode {
		case InputNewIssue:
			if value == "" {
				m.state = StateNormal
				m.statusMsg = "Cancelled"
				return m, nil
			}
			m.pendingTitle = value
			m.state = StateTypeSelect
			m.inputMode = InputNone
			return m, nil
		case InputReview:
			if value == "" {
				// Go back to review preview
				m.state = StateReviewPreview
				m.inputMode = InputNone
				return m, nil
			}
			m.state = StateNormal
			m.inputMode = InputNone
			m.reviewAnalysis = ""
			return m.executeReview(value)
		case InputPlanReview:
			if value == "" {
				// Go back to plan preview
				m.state = StatePlanPreview
				m.inputMode = InputNone
				return m, nil
			}
			m.state = StateNormal
			m.inputMode = InputNone
			m.reviewPlan = ""
			return m.executePlanReview(value)
		case InputAddOption:
			if value == "" {
				// Go back to option select
				m.state = StateOptionSelect
				m.inputMode = InputNone
				return m, nil
			}
			m.state = StateOptionSelect
			m.inputMode = InputNone
			return m.executeAddOption(value)
		case InputChangeReason:
			m.state = StateNormal
			m.inputMode = InputNone
			return m.executeUpdateChangeLog(value)
		default:
			m.state = StateNormal
			return m, nil
		}

	case tea.KeyEsc:
		if m.inputMode == InputReview {
			// Go back to review preview
			m.state = StateReviewPreview
			m.inputMode = InputNone
			m.textInput.Reset()
			return m, nil
		}
		if m.inputMode == InputPlanReview {
			// Go back to plan preview
			m.state = StatePlanPreview
			m.inputMode = InputNone
			m.textInput.Reset()
			return m, nil
		}
		if m.inputMode == InputAddOption {
			// Go back to option select
			m.state = StateOptionSelect
			m.inputMode = InputNone
			m.textInput.Reset()
			return m, nil
		}
		if m.inputMode == InputChangeReason {
			m.state = StateNormal
			m.inputMode = InputNone
			m.textInput.Reset()
			m.statusMsg = "Change log update cancelled"
			return m, nil
		}
		m.state = StateNormal
		m.inputMode = InputNone
		m.textInput.Reset()
		m.statusMsg = "Cancelled"
		return m, nil
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Yes):
		m.state = StateNormal

		// Handle confirm actions
		if m.confirmAction != nil {
			m.confirmAction()
		}
		m.pendingRetryIssue = nil
		return m, m.refreshIssues()

	case key.Matches(msg, m.keys.No), key.Matches(msg, m.keys.Escape):
		m.state = StateNormal
		m.pendingRetryIssue = nil
		m.statusMsg = "Cancelled"
		return m, nil
	}

	return m, nil
}

func (m Model) handleTypeSelectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "f", "1":
		return m.createIssue(model.TypeFeature)
	case "b", "2":
		return m.createIssue(model.TypeBug)
	case "r", "3":
		return m.createIssue(model.TypeRefactor)
	case "esc":
		m.state = StateNormal
		m.statusMsg = "Cancelled"
	}
	return m, nil
}

func (m Model) handleModelSelectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "o", "1":
		m.pendingModel = ModelOpus
		m.modelCursor = 0
	case "s", "2":
		m.pendingModel = ModelSonnet
		m.modelCursor = 1
	case "h", "3":
		m.pendingModel = ModelHaiku
		m.modelCursor = 2
	case "up", "k":
		m.modelCursor = (m.modelCursor - 1 + 3) % 3
		m.pendingModel = availableModels[m.modelCursor]
	case "down", "j":
		m.modelCursor = (m.modelCursor + 1) % 3
		m.pendingModel = availableModels[m.modelCursor]
	case "enter":
		// Directly execute implementation - no confirm step
		issue := m.pendingRetryIssue
		m.pendingRetryIssue = nil
		m.state = StateNormal
		return m.executeImplementFor(issue)
	case "esc", "q":
		m.state = StateNormal
		m.pendingRetryIssue = nil
		m.pendingModel = ""
		m.modelCursor = 0
		m.statusMsg = "Cancelled"
	}
	return m, nil
}

func (m Model) handleUncommittedChangesErrorKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "esc", "q":
		m.state = StateNormal
		m.statusMsg = "Implementation cancelled - commit your changes first"
		return m, nil
	}
	return m, nil
}

package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case StateInput:
		return m.handleInputKey(msg)
	case StateConfirm:
		return m.handleConfirmKey(msg)
	case StateTypeSelect:
		return m.handleTypeSelectKey(msg)
	case StateModelSelect:
		return m.handleModelSelectKey(msg)
	case StateReviewPreview:
		return m.handleReviewPreviewKey(msg)
	case StatePlanPreview:
		return m.handlePlanPreviewKey(msg)
	case StateCommitConfirm:
		return m.handleCommitConfirmKey(msg)
	case StateCommitGenerating:
		// Ignore key input while generating
		return m, nil
	case StateOptionSelect:
		return m.handleOptionSelectKey(msg)
	case StateUncommittedChangesError:
		return m.handleUncommittedChangesErrorKey(msg)
	default:
		return m.handleNormalKey(msg)
	}
}

package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lunit-heesungyang/issue-manager/internal/claude"
	"github.com/lunit-heesungyang/issue-manager/internal/model"
)

// Tick message for spinner animation
type tickMsg time.Time

func (m Model) tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Result message from async Claude task
type resultMsg claude.TaskResult

func (m Model) listenForResults() tea.Cmd {
	return func() tea.Msg {
		result := <-m.resultChan
		return resultMsg(result)
	}
}

// Refresh issues from storage
type issuesLoadedMsg []*model.Issue

// Request to refresh issues (triggers refreshIssues command)
type refreshRequestMsg struct{}

// syncAfterEditMsg triggers brief-to-index sync after editor closes
type syncAfterEditMsg struct {
	issueID string
}

// implementCompletedMsg triggers status update after implementation completes
type implementCompletedMsg struct {
	issueID string
}

func (m Model) refreshIssues() tea.Cmd {
	return func() tea.Msg {
		idx, err := m.storage.LoadIndex()
		if err != nil {
			return nil
		}

		var filtered []*model.Issue
		switch m.filterMode {
		case FilterActive:
			filtered = idx.FilterByStatus(
				model.StatusOpen,
				model.StatusAnalyzed,
				model.StatusPlanned,
				model.StatusImplemented,
			)
		case FilterAll:
			filtered = idx.Issues
		case FilterClosed:
			filtered = idx.FilterByStatus(
				model.StatusClosed,
				model.StatusInvalid,
			)
		}
		return issuesLoadedMsg(filtered)
	}
}

package tui

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"

	"github.com/lunit-heesungyang/issue-manager/internal/claude"
	"github.com/lunit-heesungyang/issue-manager/internal/model"
)

// Issue lifecycle actions

// getEditorCommand returns the user's preferred editor or "vim" as default
func getEditorCommand() string {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		return "vim"
	}
	return editor
}

// executeEditorCommand opens a file in the user's editor and returns a tea.Cmd
func executeEditorCommand(path string, callback func(error) tea.Msg) tea.Cmd {
	editor := getEditorCommand()
	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return tea.ExecProcess(cmd, callback)
}

func (m Model) startNewIssue() (Model, tea.Cmd) {
	m.state = StateInput
	m.inputMode = InputNewIssue
	m.inputPrompt = "Title: "
	m.textInput.Focus()
	return m, textinput.Blink
}

func (m Model) createIssue(issueType model.IssueType) (Model, tea.Cmd) {
	m.state = StateNormal
	issue, err := m.storage.CreateIssue(m.pendingTitle, issueType, "")
	if err != nil {
		m.statusMsg = fmt.Sprintf("Error: %v", err)
		return m, nil
	}
	m.statusMsg = fmt.Sprintf("Created %s - opening editor", issue.ID)

	// Open editor for the new issue's brief.md
	briefPath := m.storage.BriefPath(issue.ID)
	return m, executeEditorCommand(briefPath, func(err error) tea.Msg {
		return syncAfterEditMsg{issueID: issue.ID}
	})
}

func (m Model) editIssue() (Model, tea.Cmd) {
	issue := m.canModifySelectedIssue()
	if issue == nil {
		return m, nil
	}

	briefPath := m.storage.BriefPath(issue.ID)
	return m, executeEditorCommand(briefPath, func(err error) tea.Msg {
		return syncAfterEditMsg{issueID: issue.ID}
	})
}

func (m Model) editAnalysis() (Model, tea.Cmd) {
	issue := m.getSelectedIssue()
	if issue == nil {
		m.statusMsg = "No issue selected"
		return m, nil
	}

	analysisPath := m.storage.AnalysisPath(issue.ID)

	// Reset review state before opening editor
	m.state = StateNormal
	m.reviewAnalysis = ""

	return m, executeEditorCommand(analysisPath, func(err error) tea.Msg {
		return refreshRequestMsg{}
	})
}

func (m Model) editPlan() (Model, tea.Cmd) {
	issue := m.getSelectedIssue()
	if issue == nil {
		m.statusMsg = "No issue selected"
		return m, nil
	}

	planPath := m.storage.PlanPath(issue.ID)

	// Reset plan review state before opening editor
	m.state = StateNormal
	m.reviewPlan = ""

	return m, executeEditorCommand(planPath, func(err error) tea.Msg {
		return refreshRequestMsg{}
	})
}

func (m Model) editAnalysisJSON() (Model, tea.Cmd) {
	issue := m.getSelectedIssue()
	if issue == nil {
		m.statusMsg = "No issue selected"
		return m, nil
	}

	analysisPath := m.storage.AnalysisJSONPath(issue.ID)

	// Reset state before opening editor
	m.state = StateNormal
	m.analysis = nil

	return m, executeEditorCommand(analysisPath, func(err error) tea.Msg {
		return refreshRequestMsg{}
	})
}

func (m Model) confirmClose() (Model, tea.Cmd) {
	issue := m.canModifySelectedIssue()
	if issue == nil {
		return m, nil
	}

	// Check if plan exists
	if !m.storage.PlanExists(issue.ID) {
		m.statusMsg = "Plan first (press 'p')"
		return m, nil
	}

	// Check if git repo
	if !m.storage.IsGitRepo() {
		m.statusMsg = "Not a git repository"
		return m, nil
	}

	// Check if there are staged changes
	if !m.storage.HasStagedChanges() {
		m.statusMsg = "No staged changes. Run 'git add' first"
		return m, nil
	}

	// Git repo with staged changes: generate commit message with Haiku
	m.pendingCloseIssue = issue
	m.state = StateCommitGenerating
	m.statusMsg = fmt.Sprintf("Generating commit message for %s...", issue.ID)

	// Load plan.md for context
	plan, _ := m.storage.LoadPlan(issue.ID)

	prompt := claude.BuildCommitMessagePrompt(issue.ID, plan)
	m.claude.RunAsync(issue.ID, "commit", prompt, "haiku", "", m.resultChan)

	return m, nil
}

func (m Model) confirmDiscard() (Model, tea.Cmd) {
	issue := m.canModifySelectedIssue()
	if issue == nil {
		return m, nil
	}

	m.state = StateConfirm
	m.confirmMsg = fmt.Sprintf("Discard issue %s?", issue.ID)
	m.confirmAction = func() {
		_ = m.storage.UpdateIssueStatus(issue.ID, model.StatusInvalid, "Discarded by user")
		m.statusMsg = fmt.Sprintf("Discarded %s", issue.ID)
	}
	return m, nil
}

func (m Model) getSelectedIssue() *model.Issue {
	if m.selected >= 0 && m.selected < len(m.issues) {
		return m.issues[m.selected]
	}
	return nil
}

// canModifySelectedIssue checks if the selected issue can be modified.
// Returns the issue if modifiable, nil otherwise.
// Sets appropriate status message if not modifiable.
func (m *Model) canModifySelectedIssue() *model.Issue {
	issue := m.getSelectedIssue()
	if issue == nil {
		m.statusMsg = "No issue selected"
		return nil
	}
	if issue.Status.IsClosed() {
		m.statusMsg = "Cannot modify closed issue"
		return nil
	}
	return issue
}

// calculateListMaxLineWidth calculates the maximum line width for issue list items
func (m *Model) calculateListMaxLineWidth() {
	m.listMaxLineWidth = 0
	for _, issue := range m.issues {
		m.processingLock.Lock()
		taskType, isProcessing := m.processing[issue.ID]
		m.processingLock.Unlock()

		var suffix string
		if isProcessing {
			suffix = fmt.Sprintf(" [%s...]", taskType)
		}
		line := fmt.Sprintf("%s %s [%s] %s%s", issue.Type.Icon(), issue.StatusIcon(), issue.ID, issue.Title, suffix)
		w := runewidth.StringWidth(line)
		if w > m.listMaxLineWidth {
			m.listMaxLineWidth = w
		}
	}
}

// Option selection helpers

func (m *Model) updateDetailViewport() {
	if m.analysis == nil || m.optionCursor >= len(m.analysis.Options) {
		return
	}

	option := m.analysis.Options[m.optionCursor]
	content := m.renderOptionDetail(&option)
	m.detailViewport.SetContent(content)
	m.detailViewport.GotoTop()

	// Reset horizontal scroll and calculate max line width
	m.detailHOffset = 0
	m.detailMaxLineWidth = calculateMaxLineWidth(content)
}

func (m Model) enterOptionSelectState(analysis *model.Analysis) Model {
	m.state = StateOptionSelect
	m.analysis = analysis
	m.optionCursor = analysis.GetRecommendedIndex()

	// Setup viewports
	contentHeight := m.height - 3
	leftWidth := m.width / 2
	rightWidth := m.width - leftWidth - 1

	summaryHeight := contentHeight * 2 / 5
	if summaryHeight < 3 {
		summaryHeight = 3
	}

	m.summaryViewport.Width = leftWidth - 4
	m.summaryViewport.Height = summaryHeight - 2

	m.detailViewport.Width = rightWidth - 4
	m.detailViewport.Height = contentHeight - 2

	// Set initial detail content
	if len(analysis.Options) > 0 {
		m.updateDetailViewport()
	}

	return m
}

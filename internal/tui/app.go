package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/lunit-heesungyang/issue-manager/internal/claude"
	"github.com/lunit-heesungyang/issue-manager/internal/model"
	"github.com/lunit-heesungyang/issue-manager/internal/storage"
)

// FilterMode represents the current filter setting
type FilterMode int

const (
	FilterActive FilterMode = iota
	FilterAll
	FilterClosed
)

func (f FilterMode) String() string {
	switch f {
	case FilterActive:
		return "Active"
	case FilterAll:
		return "All"
	case FilterClosed:
		return "Closed"
	}
	return ""
}

// AppState represents the current UI state
type AppState int

const (
	StateNormal AppState = iota
	StateInput
	StateConfirm
	StateTypeSelect
	StateReviewPreview
	StatePlanPreview
	StateCommitConfirm
	StateCommitGenerating
)

// InputMode represents what input is being collected
type InputMode int

const (
	InputNone InputMode = iota
	InputNewIssue
	InputReview
	InputPlanReview
)

// Model is the main Bubble Tea model
type Model struct {
	// Core dependencies
	storage *storage.Storage
	claude  *claude.Client
	keys    KeyMap
	styles  Styles

	// Window dimensions
	width  int
	height int

	// Issue list state
	issues     []*model.Issue
	selected   int
	filterMode FilterMode

	// UI state
	state     AppState
	statusMsg string

	// Processing state
	processing     map[string]string // issueID -> taskType
	processingLock *sync.Mutex
	spinnerFrame   int

	// Async results channel
	resultChan chan claude.TaskResult

	// Sub-components
	textInput textinput.Model
	viewport  viewport.Model

	// Confirm state
	confirmMsg    string
	confirmAction func()

	// Input state
	inputPrompt string
	inputMode   InputMode

	// Type select state
	pendingTitle string

	// Review state
	reviewAnalysis string
	reviewPlan     string

	// Commit state
	pendingCommitMsg  string
	pendingCloseIssue *model.Issue
}

// New creates a new TUI model
func New(projectPath string) Model {
	s := storage.New(projectPath)
	_ = s.EnsureIssuesDir()

	ti := textinput.New()
	ti.CharLimit = 200
	ti.Width = 50

	vp := viewport.New(40, 20)

	return Model{
		storage:        s,
		claude:         claude.New(projectPath),
		keys:           DefaultKeyMap(),
		styles:         DefaultStyles(),
		processing:     make(map[string]string),
		processingLock: &sync.Mutex{},
		resultChan:     make(chan claude.TaskResult, 10),
		textInput:      ti,
		viewport:       vp,
		filterMode:     FilterActive,
	}
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.refreshIssues(),
		m.tickCmd(),
		m.listenForResults(),
	)
}

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

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width / 2
		m.viewport.Height = msg.Height - 4
		return m, nil

	case tickMsg:
		m.spinnerFrame = (m.spinnerFrame + 1) % len(SpinnerFrames)
		cmds = append(cmds, m.tickCmd())

	case issuesLoadedMsg:
		m.issues = msg
		if m.selected >= len(m.issues) {
			m.selected = max(0, len(m.issues)-1)
		}
		return m, nil

	case resultMsg:
		m.handleResult(claude.TaskResult(msg))
		cmds = append(cmds, m.listenForResults())
		cmds = append(cmds, m.refreshIssues())

	case refreshRequestMsg:
		return m, m.refreshIssues()
	}

	return m, tea.Batch(cmds...)
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case StateInput:
		return m.handleInputKey(msg)
	case StateConfirm:
		return m.handleConfirmKey(msg)
	case StateTypeSelect:
		return m.handleTypeSelectKey(msg)
	case StateReviewPreview:
		return m.handleReviewPreviewKey(msg)
	case StatePlanPreview:
		return m.handlePlanPreviewKey(msg)
	case StateCommitConfirm:
		return m.handleCommitConfirmKey(msg)
	case StateCommitGenerating:
		// Ignore key input while generating
		return m, nil
	default:
		return m.handleNormalKey(msg)
	}
}

func (m Model) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Up):
		if m.selected > 0 {
			m.selected--
		}
		return m, nil

	case key.Matches(msg, m.keys.Down):
		if m.selected < len(m.issues)-1 {
			m.selected++
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

	case key.Matches(msg, m.keys.Refresh):
		m.statusMsg = "Refreshed"
		return m, m.refreshIssues()

	case key.Matches(msg, m.keys.Filter):
		m.filterMode = (m.filterMode + 1) % 3
		m.statusMsg = fmt.Sprintf("Filter: %s", m.filterMode)
		return m, m.refreshIssues()
	}

	return m, nil
}

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
		if m.confirmAction != nil {
			m.confirmAction()
		}
		return m, m.refreshIssues()

	case key.Matches(msg, m.keys.No), key.Matches(msg, m.keys.Escape):
		m.state = StateNormal
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

func (m Model) handleReviewPreviewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	// Scroll keys
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

	// Action keys
	case "e":
		return m.editAnalysis()
	case "f":
		// Switch to feedback input mode
		m.state = StateInput
		m.inputMode = InputReview
		m.inputPrompt = "Feedback: "
		m.textInput.Focus()
		return m, textinput.Blink
	case "c", "esc":
		m.state = StateNormal
		m.reviewAnalysis = ""
		return m, nil
	}

	return m, nil
}

func (m Model) handlePlanPreviewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	// Scroll keys
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

	// Action keys
	case "e":
		return m.editPlan()
	case "f":
		// Switch to feedback input mode
		m.state = StateInput
		m.inputMode = InputPlanReview
		m.inputPrompt = "Plan Feedback: "
		m.textInput.Focus()
		return m, textinput.Blink
	case "c", "esc":
		m.state = StateNormal
		m.reviewPlan = ""
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

func (m *Model) handleResult(result claude.TaskResult) {
	m.processingLock.Lock()
	delete(m.processing, result.IssueID)
	m.processingLock.Unlock()

	switch result.TaskType {
	case "analyze":
		if result.Success {
			_ = m.storage.SaveAnalysis(result.IssueID, result.Result)
			if result.SessionID != "" {
				_ = m.storage.SaveSessionID(result.IssueID, result.SessionID)
			}
			_ = m.storage.UpdateIssueStatus(result.IssueID, model.StatusAnalyzed, "")
			m.statusMsg = fmt.Sprintf("Analyzed %s", result.IssueID)
		} else {
			m.statusMsg = fmt.Sprintf("Analyze %s failed", result.IssueID)
		}
	case "plan":
		if result.Success {
			_ = m.storage.SavePlan(result.IssueID, result.Result)
			_ = m.storage.UpdateIssueStatus(result.IssueID, model.StatusPlanned, "")
			m.statusMsg = fmt.Sprintf("Planned %s", result.IssueID)
		} else {
			m.statusMsg = fmt.Sprintf("Plan %s failed", result.IssueID)
		}
	case "review":
		if result.Success {
			_ = m.storage.SaveAnalysis(result.IssueID, result.Result)
			if result.SessionID != "" {
				_ = m.storage.SaveSessionID(result.IssueID, result.SessionID)
			}
			m.statusMsg = fmt.Sprintf("Reviewed %s", result.IssueID)
		} else {
			m.statusMsg = fmt.Sprintf("Review %s failed", result.IssueID)
		}
	case "plan-review":
		if result.Success {
			_ = m.storage.SavePlan(result.IssueID, result.Result)
			if result.SessionID != "" {
				_ = m.storage.SaveSessionID(result.IssueID, result.SessionID)
			}
			m.statusMsg = fmt.Sprintf("Plan reviewed %s", result.IssueID)
		} else {
			m.statusMsg = fmt.Sprintf("Plan review %s failed", result.IssueID)
		}
	case "commit":
		if result.Success {
			m.pendingCommitMsg = strings.TrimSpace(result.Result)
			m.state = StateCommitConfirm
			m.statusMsg = "Review commit message"
		} else {
			m.state = StateNormal
			m.pendingCloseIssue = nil
			m.statusMsg = fmt.Sprintf("Commit message generation failed: %s", result.IssueID)
		}
	}
}

// View implements tea.Model
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	// Calculate layout - reserve 3 lines for header(1) + footer(1) + status(1)
	listWidth := m.width / 2
	previewWidth := m.width - listWidth
	contentHeight := m.height - 3
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Render header
	header := m.styles.Header.Render(
		fmt.Sprintf("Issue Manager [%s]", m.filterMode),
	)

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
	keys := "[n]ew [a]nalyze [R]eview [p]lan [P]lan-review [i]mplement [c]lose [d]iscard [e]dit [f]ilter [q]uit"
	footer := m.styles.Footer.Render(keys)
	status := m.styles.StatusBar.Render(m.statusMsg)

	// Handle special states
	var overlay string
	switch m.state {
	case StateInput:
		overlay = m.renderInputOverlay()
	case StateConfirm:
		overlay = m.renderConfirmOverlay()
	case StateTypeSelect:
		overlay = m.renderTypeSelectOverlay()
	case StateReviewPreview:
		overlay = m.renderReviewPreviewOverlay()
	case StatePlanPreview:
		overlay = m.renderPlanPreviewOverlay()
	case StateCommitConfirm:
		overlay = m.renderCommitConfirmOverlay()
	case StateCommitGenerating:
		overlay = m.renderCommitGeneratingOverlay()
	}

	// Combine vertically
	view := lipgloss.JoinVertical(lipgloss.Left,
		header,
		content,
		footer,
		status,
	)

	if overlay != "" {
		// Center overlay on screen
		overlayStyle := lipgloss.NewStyle().
			Width(m.width).
			Height(m.height).
			Align(lipgloss.Center, lipgloss.Center)
		return overlayStyle.Render(overlay)
	}

	// Force exact terminal height to prevent scrolling issues
	lines := strings.Split(view, "\n")
	if len(lines) > m.height {
		lines = lines[:m.height]
	}
	for len(lines) < m.height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderList(width, height int) string {
	var lines []string

	if len(m.issues) == 0 {
		lines = append(lines, fmt.Sprintf("No %s issues", m.filterMode))
	} else {
		for i, issue := range m.issues {
			if i >= height {
				break
			}

			// Get icon
			var icon string
			m.processingLock.Lock()
			taskType, isProcessing := m.processing[issue.ID]
			m.processingLock.Unlock()

			if isProcessing {
				icon = SpinnerFrames[m.spinnerFrame]
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

func (m Model) renderInputOverlay() string {
	if m.inputMode == InputReview && m.reviewAnalysis != "" {
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

		return m.styles.PopupBorder.Width(popupWidth).Render(
			fmt.Sprintf("%s\n%s\n%s%s\n%s\n%s",
				m.styles.PopupTitle.Render("Current Analysis:"),
				m.viewport.View(),
				strings.Repeat("─", popupWidth-10),
				scrollInfo,
				m.styles.InputPrompt.Render(m.inputPrompt),
				m.textInput.View(),
			),
		)
	}

	return m.styles.PopupBorder.Render(
		fmt.Sprintf("%s\n%s",
			m.styles.InputPrompt.Render(m.inputPrompt),
			m.textInput.View(),
		),
	)
}

func (m Model) renderConfirmOverlay() string {
	return m.styles.PopupBorder.Render(
		fmt.Sprintf("%s\n\n[y]es / [n]o",
			m.styles.PopupTitle.Render(m.confirmMsg),
		),
	)
}

func (m Model) renderTypeSelectOverlay() string {
	return m.styles.PopupBorder.Render(
		fmt.Sprintf("%s\n\n[f]eature  [b]ug  [r]efactor\n\n[esc] cancel",
			m.styles.PopupTitle.Render("Select issue type:"),
		),
	)
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

	// Scroll indicator
	scrollPercent := m.viewport.ScrollPercent() * 100
	scrollInfo := fmt.Sprintf(" %3.0f%% ", scrollPercent)

	// Help text for actions
	helpText := "[e]dit  [f]eedback  [c]lose   ↑↓/j/k scroll"

	return m.styles.PopupBorder.Width(popupWidth).Render(
		fmt.Sprintf("%s\n%s\n%s%s\n\n%s",
			m.styles.PopupTitle.Render("Review Analysis:"),
			m.viewport.View(),
			strings.Repeat("─", popupWidth-10),
			scrollInfo,
			helpText,
		),
	)
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

	// Scroll indicator
	scrollPercent := m.viewport.ScrollPercent() * 100
	scrollInfo := fmt.Sprintf(" %3.0f%% ", scrollPercent)

	// Help text for actions
	helpText := "[e]dit  [f]eedback  [c]lose   ↑↓/j/k scroll"

	return m.styles.PopupBorder.Width(popupWidth).Render(
		fmt.Sprintf("%s\n%s\n%s%s\n\n%s",
			m.styles.PopupTitle.Render("Review Plan:"),
			m.viewport.View(),
			strings.Repeat("─", popupWidth-10),
			scrollInfo,
			helpText,
		),
	)
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
	wrapped := wrapText(m.pendingCommitMsg, popupWidth-6)
	lines := strings.Split(wrapped, "\n")
	if len(lines) > viewportHeight {
		lines = lines[:viewportHeight]
	}
	displayMsg := strings.Join(lines, "\n")

	issueInfo := ""
	if m.pendingCloseIssue != nil {
		issueInfo = fmt.Sprintf(" [%s] %s", m.pendingCloseIssue.ID, m.pendingCloseIssue.Title)
	}

	helpText := "[y]es/Enter: commit   [n]/Esc: cancel   ↑↓ scroll"

	return m.styles.PopupBorder.Width(popupWidth).Render(
		fmt.Sprintf("%s\n%s\n\n%s\n\n%s",
			m.styles.PopupTitle.Render("Commit Message:"+issueInfo),
			strings.Repeat("─", popupWidth-6),
			displayMsg,
			helpText,
		),
	)
}

func (m Model) renderCommitGeneratingOverlay() string {
	spinner := SpinnerFrames[m.spinnerFrame]
	issueInfo := ""
	if m.pendingCloseIssue != nil {
		issueInfo = fmt.Sprintf(" [%s]", m.pendingCloseIssue.ID)
	}

	return m.styles.PopupBorder.Render(
		fmt.Sprintf("%s\n\n%s Generating commit message with Haiku...",
			m.styles.PopupTitle.Render("Closing Issue"+issueInfo),
			spinner,
		),
	)
}

// Actions

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
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	cmd := exec.Command(editor, briefPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return refreshRequestMsg{}
	})
}

func (m Model) editIssue() (Model, tea.Cmd) {
	issue := m.getSelectedIssue()
	if issue == nil {
		m.statusMsg = "No issue selected"
		return m, nil
	}

	briefPath := m.storage.BriefPath(issue.ID)
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	cmd := exec.Command(editor, briefPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return refreshRequestMsg{} // Trigger refresh after editor closes
	})
}

func (m Model) editAnalysis() (Model, tea.Cmd) {
	issue := m.getSelectedIssue()
	if issue == nil {
		m.statusMsg = "No issue selected"
		return m, nil
	}

	analysisPath := m.storage.AnalysisPath(issue.ID)
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	// Reset review state before opening editor
	m.state = StateNormal
	m.reviewAnalysis = ""

	cmd := exec.Command(editor, analysisPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
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
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	// Reset plan review state before opening editor
	m.state = StateNormal
	m.reviewPlan = ""

	cmd := exec.Command(editor, planPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return refreshRequestMsg{}
	})
}

func (m Model) confirmClose() (Model, tea.Cmd) {
	issue := m.getSelectedIssue()
	if issue == nil {
		m.statusMsg = "No issue selected"
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
	issue := m.getSelectedIssue()
	if issue == nil {
		m.statusMsg = "No issue selected"
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

func (m Model) analyzeIssue() (Model, tea.Cmd) {
	issue := m.getSelectedIssue()
	if issue == nil {
		m.statusMsg = "No issue selected"
		return m, nil
	}

	m.processingLock.Lock()
	if _, ok := m.processing[issue.ID]; ok {
		m.processingLock.Unlock()
		m.statusMsg = fmt.Sprintf("%s is already processing", issue.ID)
		return m, nil
	}
	m.processing[issue.ID] = "analyze"
	m.processingLock.Unlock()

	// Load brief content
	brief, err := m.storage.LoadBrief(issue.ID)
	if err != nil || brief == nil {
		m.statusMsg = "Cannot load brief"
		return m, nil
	}

	briefPath := m.storage.BriefPath(issue.ID)
	prompt := claude.BuildAnalysisPrompt(brief.Content, briefPath)

	m.statusMsg = fmt.Sprintf("Analyzing %s...", issue.ID)
	m.claude.RunAsync(issue.ID, "analyze", prompt, "", "", m.resultChan)

	return m, nil
}

func (m Model) planIssue() (Model, tea.Cmd) {
	issue := m.getSelectedIssue()
	if issue == nil {
		m.statusMsg = "No issue selected"
		return m, nil
	}

	if !m.storage.AnalysisExists(issue.ID) {
		m.statusMsg = "Analyze first"
		return m, nil
	}

	m.processingLock.Lock()
	if _, ok := m.processing[issue.ID]; ok {
		m.processingLock.Unlock()
		m.statusMsg = fmt.Sprintf("%s is already processing", issue.ID)
		return m, nil
	}
	m.processing[issue.ID] = "plan"
	m.processingLock.Unlock()

	// Load content
	brief, _ := m.storage.LoadBrief(issue.ID)
	analysis, _ := m.storage.LoadAnalysis(issue.ID)
	sessionID, _ := m.storage.LoadSessionID(issue.ID)

	prompt := claude.BuildPlanPrompt(brief.Content, analysis)

	m.statusMsg = fmt.Sprintf("Planning %s...", issue.ID)
	m.claude.RunAsync(issue.ID, "plan", prompt, "", sessionID, m.resultChan)

	return m, nil
}

func (m Model) reviewIssue() (Model, tea.Cmd) {
	issue := m.getSelectedIssue()
	if issue == nil {
		m.statusMsg = "No issue selected"
		return m, nil
	}

	if !m.storage.AnalysisExists(issue.ID) {
		m.statusMsg = "Analyze first"
		return m, nil
	}

	m.processingLock.Lock()
	if _, ok := m.processing[issue.ID]; ok {
		m.processingLock.Unlock()
		m.statusMsg = fmt.Sprintf("%s is already processing", issue.ID)
		return m, nil
	}
	m.processingLock.Unlock()

	// Load analysis for preview
	analysis, err := m.storage.LoadAnalysis(issue.ID)
	if err != nil {
		m.statusMsg = "Failed to load analysis"
		return m, nil
	}
	m.reviewAnalysis = analysis

	// Setup viewport for scrollable analysis
	viewportHeight := m.height - 10
	if viewportHeight < 10 {
		viewportHeight = 10
	}
	viewportWidth := m.width - 14
	if viewportWidth < 50 {
		viewportWidth = 50
	}
	if viewportWidth > 96 {
		viewportWidth = 96
	}
	m.viewport.Width = viewportWidth
	m.viewport.Height = viewportHeight
	m.viewport.SetContent(analysis)
	m.viewport.GotoTop()

	// Enter review preview mode
	m.state = StateReviewPreview
	return m, nil
}

func (m Model) planReviewIssue() (Model, tea.Cmd) {
	issue := m.getSelectedIssue()
	if issue == nil {
		m.statusMsg = "No issue selected"
		return m, nil
	}

	if !m.storage.PlanExists(issue.ID) {
		m.statusMsg = "Plan first (press 'p')"
		return m, nil
	}

	m.processingLock.Lock()
	if _, ok := m.processing[issue.ID]; ok {
		m.processingLock.Unlock()
		m.statusMsg = fmt.Sprintf("%s is already processing", issue.ID)
		return m, nil
	}
	m.processingLock.Unlock()

	// Load plan for preview
	plan, err := m.storage.LoadPlan(issue.ID)
	if err != nil {
		m.statusMsg = "Failed to load plan"
		return m, nil
	}
	m.reviewPlan = plan

	// Setup viewport for scrollable plan
	viewportHeight := m.height - 10
	if viewportHeight < 10 {
		viewportHeight = 10
	}
	viewportWidth := m.width - 14
	if viewportWidth < 50 {
		viewportWidth = 50
	}
	if viewportWidth > 96 {
		viewportWidth = 96
	}
	m.viewport.Width = viewportWidth
	m.viewport.Height = viewportHeight
	m.viewport.SetContent(plan)
	m.viewport.GotoTop()

	// Enter plan preview mode
	m.state = StatePlanPreview
	return m, nil
}

func (m Model) executeReview(feedback string) (Model, tea.Cmd) {
	issue := m.getSelectedIssue()
	if issue == nil {
		m.statusMsg = "No issue selected"
		return m, nil
	}

	m.processingLock.Lock()
	if _, ok := m.processing[issue.ID]; ok {
		m.processingLock.Unlock()
		m.statusMsg = fmt.Sprintf("%s is already processing", issue.ID)
		return m, nil
	}
	m.processing[issue.ID] = "review"
	m.processingLock.Unlock()

	// Load current analysis
	analysis, _ := m.storage.LoadAnalysis(issue.ID)
	sessionID, _ := m.storage.LoadSessionID(issue.ID)

	prompt := claude.BuildReviewPrompt(analysis, feedback)

	m.statusMsg = fmt.Sprintf("Reviewing %s...", issue.ID)
	m.claude.RunAsync(issue.ID, "review", prompt, "", sessionID, m.resultChan)

	return m, nil
}

func (m Model) executePlanReview(feedback string) (Model, tea.Cmd) {
	issue := m.getSelectedIssue()
	if issue == nil {
		m.statusMsg = "No issue selected"
		return m, nil
	}

	m.processingLock.Lock()
	if _, ok := m.processing[issue.ID]; ok {
		m.processingLock.Unlock()
		m.statusMsg = fmt.Sprintf("%s is already processing", issue.ID)
		return m, nil
	}
	m.processing[issue.ID] = "plan-review"
	m.processingLock.Unlock()

	// Load current plan
	plan, _ := m.storage.LoadPlan(issue.ID)
	sessionID, _ := m.storage.LoadSessionID(issue.ID)

	prompt := claude.BuildPlanReviewPrompt(plan, feedback)

	m.statusMsg = fmt.Sprintf("Reviewing plan %s...", issue.ID)
	m.claude.RunAsync(issue.ID, "plan-review", prompt, "", sessionID, m.resultChan)

	return m, nil
}

func (m Model) implementIssue() (Model, tea.Cmd) {
	issue := m.getSelectedIssue()
	if issue == nil {
		m.statusMsg = "No issue selected"
		return m, nil
	}

	// Only planned issues can be implemented
	if issue.Status != model.StatusPlanned {
		m.statusMsg = "Only planned issues can be implemented"
		return m, nil
	}

	// Check if plan exists
	if !m.storage.PlanExists(issue.ID) {
		m.statusMsg = "Plan first (press 'p')"
		return m, nil
	}

	// Check if session exists for --resume
	sessionID, _ := m.storage.LoadSessionID(issue.ID)
	if sessionID == "" {
		m.statusMsg = "No session found. Re-analyze the issue first"
		return m, nil
	}

	// Build prompt with plan path
	planPath := m.storage.PlanPath(issue.ID)
	prompt := claude.BuildImplementPrompt(planPath)

	// Run Claude CLI in interactive mode with --resume
	cmd := exec.Command("claude", "--resume", sessionID, prompt)
	cmd.Dir = m.claude.WorkingDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	m.statusMsg = fmt.Sprintf("Implementing %s...", issue.ID)

	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return refreshRequestMsg{}
	})
}

func (m Model) getSelectedIssue() *model.Issue {
	if m.selected >= 0 && m.selected < len(m.issues) {
		return m.issues[m.selected]
	}
	return nil
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

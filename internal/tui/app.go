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

	// Horizontal scroll state
	hOffset      int // horizontal scroll offset
	maxLineWidth int // max line width in current content

	// Commit state
	pendingCommitMsg  string
	pendingCloseIssue *model.Issue

	// Retry confirmation state
	pendingRetryIssue *model.Issue
	pendingImplement  bool

	// List scroll state
	listVOffset      int // vertical scroll offset for issue list
	listHOffset      int // horizontal scroll offset for issue list
	listMaxLineWidth int // max line width in issue list
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

// syncAfterEditMsg triggers brief-to-index sync after editor closes
type syncAfterEditMsg struct {
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
		// Validate scroll offsets after resize
		listVisibleHeight := m.height - 3
		if listVisibleHeight < 1 {
			listVisibleHeight = 1
		}
		m.ensureSelectedVisible(listVisibleHeight)
		// Validate horizontal scroll offset
		listWidth := m.width/2 - 2
		maxHOffset := m.listMaxLineWidth - listWidth
		if maxHOffset < 0 {
			maxHOffset = 0
		}
		if m.listHOffset > maxHOffset {
			m.listHOffset = maxHOffset
		}
		return m, nil

	case tickMsg:
		m.spinnerFrame = (m.spinnerFrame + 1) % len(SpinnerFrames)
		cmds = append(cmds, m.tickCmd())

	case issuesLoadedMsg:
		m.issues = msg
		if m.selected >= len(m.issues) {
			m.selected = max(0, len(m.issues)-1)
		}
		// Validate vertical scroll offset
		listVisibleHeight := m.height - 3
		if listVisibleHeight < 1 {
			listVisibleHeight = 1
		}
		m.ensureSelectedVisible(listVisibleHeight)
		// Calculate max line width for horizontal scrolling
		m.calculateListMaxLineWidth()
		return m, nil

	case resultMsg:
		m.handleResult(claude.TaskResult(msg))
		cmds = append(cmds, m.listenForResults())
		cmds = append(cmds, m.refreshIssues())

	case refreshRequestMsg:
		return m, m.refreshIssues()

	case syncAfterEditMsg:
		// Sync brief.md changes to index.yaml
		_ = m.storage.SyncBriefToIndex(msg.issueID)
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

	case key.Matches(msg, m.keys.Refresh):
		m.statusMsg = "Refreshed"
		return m, m.refreshIssues()

	case key.Matches(msg, m.keys.Filter):
		m.filterMode = (m.filterMode + 1) % 3
		m.listVOffset = 0 // Reset vertical scroll on filter change
		m.listHOffset = 0 // Reset horizontal scroll on filter change
		m.statusMsg = fmt.Sprintf("Filter: %s", m.filterMode)
		return m, m.refreshIssues()
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

		// Handle pending implement (needs to return tea.Cmd for tea.ExecProcess)
		if m.pendingImplement && m.pendingRetryIssue != nil {
			issue := m.pendingRetryIssue
			m.pendingRetryIssue = nil
			m.pendingImplement = false
			return m.executeImplementFor(issue)
		}

		// Handle other confirm actions
		if m.confirmAction != nil {
			m.confirmAction()
		}
		m.pendingRetryIssue = nil
		return m, m.refreshIssues()

	case key.Matches(msg, m.keys.No), key.Matches(msg, m.keys.Escape):
		m.state = StateNormal
		m.pendingRetryIssue = nil
		m.pendingImplement = false
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
		m.hOffset = 0
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
		m.hOffset = 0
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
	keys := "[n]ew [a]nalyze [R]eview [p]lan [P]lan-review [i]mplement [c]lose [d]iscard [e]dit [f]ilter [q]uit ←→scroll"
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
		OverlayIcons.Feature,
		OverlayIcons.Bug,
		OverlayIcons.Refactor,
	)

	// Footer with cancel hint
	footer := "[Esc] Cancel"

	return m.renderBaseOverlay("Select Issue Type", options, footer, 40)
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
	spinner := SpinnerFrames[m.spinnerFrame]

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
		return syncAfterEditMsg{issueID: issue.ID}
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
	m.processingLock.Unlock()

	// Check if analysis already exists
	if m.storage.AnalysisExists(issue.ID) {
		m.state = StateConfirm
		m.confirmMsg = fmt.Sprintf("analysis.md exists for %s. Re-analyze?", issue.ID)
		m.pendingRetryIssue = issue
		m.confirmAction = func() {
			m.executeAnalyze()
		}
		return m, nil
	}

	return m.executeAnalyzeFor(issue)
}

func (m *Model) executeAnalyze() {
	if m.pendingRetryIssue == nil {
		return
	}
	issue := m.pendingRetryIssue

	m.processingLock.Lock()
	m.processing[issue.ID] = "analyze"
	m.processingLock.Unlock()

	brief, err := m.storage.LoadBrief(issue.ID)
	if err != nil || brief == nil {
		m.statusMsg = "Cannot load brief"
		return
	}

	briefPath := m.storage.BriefPath(issue.ID)
	prompt := claude.BuildAnalysisPrompt(brief.Content, briefPath)

	m.statusMsg = fmt.Sprintf("Analyzing %s...", issue.ID)
	m.claude.RunAsync(issue.ID, "analyze", prompt, "", "", m.resultChan)
}

func (m Model) executeAnalyzeFor(issue *model.Issue) (Model, tea.Cmd) {
	m.processingLock.Lock()
	m.processing[issue.ID] = "analyze"
	m.processingLock.Unlock()

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
	m.processingLock.Unlock()

	// Check if plan already exists
	if m.storage.PlanExists(issue.ID) {
		m.state = StateConfirm
		m.confirmMsg = fmt.Sprintf("plan.md exists for %s. Re-plan?", issue.ID)
		m.pendingRetryIssue = issue
		m.confirmAction = func() {
			m.executePlan()
		}
		return m, nil
	}

	return m.executePlanFor(issue)
}

func (m *Model) executePlan() {
	if m.pendingRetryIssue == nil {
		return
	}
	issue := m.pendingRetryIssue

	m.processingLock.Lock()
	m.processing[issue.ID] = "plan"
	m.processingLock.Unlock()

	brief, _ := m.storage.LoadBrief(issue.ID)
	analysis, _ := m.storage.LoadAnalysis(issue.ID)
	sessionID, _ := m.storage.LoadSessionID(issue.ID)

	prompt := claude.BuildPlanPrompt(brief.Content, analysis)

	m.statusMsg = fmt.Sprintf("Planning %s...", issue.ID)
	m.claude.RunAsync(issue.ID, "plan", prompt, "", sessionID, m.resultChan)
}

func (m Model) executePlanFor(issue *model.Issue) (Model, tea.Cmd) {
	m.processingLock.Lock()
	m.processing[issue.ID] = "plan"
	m.processingLock.Unlock()

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

	// Initialize horizontal scroll state
	m.hOffset = 0
	m.maxLineWidth = calculateMaxLineWidth(analysis)
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

	// Initialize horizontal scroll state
	m.hOffset = 0
	m.maxLineWidth = calculateMaxLineWidth(plan)
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

	// Implement always requires confirmation as it may modify code
	m.state = StateConfirm
	m.confirmMsg = fmt.Sprintf("Implement %s? This may modify code.", issue.ID)
	m.pendingRetryIssue = issue
	m.pendingImplement = true
	m.confirmAction = nil // Will be handled specially in handleConfirmKey
	return m, nil
}

func (m Model) executeImplementFor(issue *model.Issue) (Model, tea.Cmd) {
	sessionID, _ := m.storage.LoadSessionID(issue.ID)
	planPath := m.storage.PlanPath(issue.ID)
	prompt := claude.BuildImplementPrompt(planPath)

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

// ensureSelectedVisible adjusts listVOffset so that the selected item is visible
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

package tui

import (
	"fmt"
	"sync"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lunit-heesungyang/issue-manager/internal/claude"
	"github.com/lunit-heesungyang/issue-manager/internal/model"
	"github.com/lunit-heesungyang/issue-manager/internal/storage"
	"github.com/lunit-heesungyang/issue-manager/internal/ui"
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
	StateOptionSelect
	StateModelSelect
	StateUncommittedChangesError
)

// InputMode represents what input is being collected
type InputMode int

const (
	InputNone InputMode = iota
	InputNewIssue
	InputReview
	InputPlanReview
	InputAddOption
	InputChangeReason
)

// AIModel represents the AI model to use for implementation
type AIModel string

const (
	ModelOpus   AIModel = "opus"
	ModelSonnet AIModel = "sonnet"
	ModelHaiku  AIModel = "haiku"
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

	// List scroll state
	listVOffset      int // vertical scroll offset for issue list
	listHOffset      int // horizontal scroll offset for issue list
	listMaxLineWidth int // max line width in issue list

	// Option selection state
	analysis           *model.Analysis // current analysis for option selection
	optionCursor       int             // cursor position in option list
	summaryViewport    viewport.Model  // viewport for summary section
	detailViewport     viewport.Model  // viewport for option detail section
	detailHOffset      int             // horizontal scroll offset for detail panel
	detailMaxLineWidth int             // max line width in detail content

	// View-only mode for closed issues
	isViewOnly bool // true when viewing closed/invalid issues in review mode

	// Model selection state
	pendingModel AIModel // selected model for implementation
	modelCursor  int     // cursor position in model list (0=opus, 1=sonnet, 2=haiku)
}

// New creates a new TUI model
func New(projectPath string) Model {
	s := storage.New(projectPath)
	_ = s.EnsureIssuesDir()

	ti := textinput.New()
	ti.CharLimit = 200
	ti.Width = 50

	vp := viewport.New(40, 20)
	summaryVp := viewport.New(40, 10)
	detailVp := viewport.New(40, 20)

	return Model{
		storage:         s,
		claude:          claude.New(projectPath),
		keys:            DefaultKeyMap(),
		styles:          DefaultStyles(),
		processing:      make(map[string]string),
		processingLock:  &sync.Mutex{},
		resultChan:      make(chan claude.TaskResult, 10),
		textInput:       ti,
		viewport:        vp,
		summaryViewport: summaryVp,
		detailViewport:  detailVp,
		filterMode:      FilterActive,
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
		m.spinnerFrame = (m.spinnerFrame + 1) % len(ui.SpinnerFrames)
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

	case implementCompletedMsg:
		// Update status to implemented after implementation completes
		_ = m.storage.UpdateIssueStatus(msg.issueID, model.StatusImplemented, "")
		m.statusMsg = fmt.Sprintf("Implemented %s", msg.issueID)
		return m, m.refreshIssues()

	case uncommittedChangesErrorMsg:
		// Show error modal when implementation is blocked
		m.state = StateUncommittedChangesError
		return m, nil
	}

	return m, tea.Batch(cmds...)
}


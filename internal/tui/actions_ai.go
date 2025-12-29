package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lunit-heesungyang/issue-manager/internal/claude"
	"github.com/lunit-heesungyang/issue-manager/internal/model"
	"github.com/lunit-heesungyang/issue-manager/internal/storage"
)

// AI integration actions

func (m Model) analyzeIssue() (Model, tea.Cmd) {
	issue := m.canModifySelectedIssue()
	if issue == nil {
		return m, nil
	}

	m.processingLock.Lock()
	if _, ok := m.processing[issue.ID]; ok {
		m.processingLock.Unlock()
		m.statusMsg = fmt.Sprintf("%s is already processing", issue.ID)
		return m, nil
	}
	m.processingLock.Unlock()

	// Check if analysis already exists (JSON or markdown)
	if m.storage.AnalysisJSONExists(issue.ID) || m.storage.AnalysisExists(issue.ID) {
		m.state = StateConfirm
		m.confirmMsg = fmt.Sprintf("Analysis exists for %s. Re-analyze?", issue.ID)
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
	// Use JSON prompt for structured output
	prompt := claude.BuildAnalysisPromptJSON(brief.Content, briefPath)

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
	// Use JSON prompt for structured output
	prompt := claude.BuildAnalysisPromptJSON(brief.Content, briefPath)

	m.statusMsg = fmt.Sprintf("Analyzing %s...", issue.ID)
	m.claude.RunAsync(issue.ID, "analyze", prompt, "", "", m.resultChan)

	return m, nil
}

func (m Model) planIssue() (Model, tea.Cmd) {
	issue := m.canModifySelectedIssue()
	if issue == nil {
		return m, nil
	}

	// Check for analysis (JSON or markdown)
	hasJSONAnalysis := m.storage.AnalysisJSONExists(issue.ID)
	hasMDAnalysis := m.storage.AnalysisExists(issue.ID)

	if !hasJSONAnalysis && !hasMDAnalysis {
		m.statusMsg = "Analyze first"
		return m, nil
	}

	// If JSON analysis exists but no option selected, prompt to select
	if hasJSONAnalysis {
		analysis, err := m.storage.LoadAnalysisJSON(issue.ID)
		if err == nil && analysis != nil && len(analysis.Options) > 0 {
			if analysis.SelectedOptionID == "" {
				// No option selected - enter option selection
				m.statusMsg = "Select an option first (press R)"
				m = m.enterOptionSelectState(analysis)
				return m, nil
			}
		}
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
	sessionID, _ := m.storage.LoadSessionID(issue.ID)

	// Try JSON analysis with selected option first
	if m.storage.AnalysisJSONExists(issue.ID) {
		analysis, err := m.storage.LoadAnalysisJSON(issue.ID)
		if err == nil && analysis != nil {
			prompt := claude.BuildPlanPromptWithOption(brief.Content, analysis)
			m.statusMsg = fmt.Sprintf("Planning %s with selected option...", issue.ID)
			m.claude.RunAsync(issue.ID, "plan", prompt, "", sessionID, m.resultChan)
			return
		}
	}

	// Fall back to markdown analysis
	analysisContent, _ := m.storage.LoadAnalysis(issue.ID)
	prompt := claude.BuildPlanPrompt(brief.Content, analysisContent)

	m.statusMsg = fmt.Sprintf("Planning %s...", issue.ID)
	m.claude.RunAsync(issue.ID, "plan", prompt, "", sessionID, m.resultChan)
}

func (m Model) executePlanFor(issue *model.Issue) (Model, tea.Cmd) {
	m.processingLock.Lock()
	m.processing[issue.ID] = "plan"
	m.processingLock.Unlock()

	brief, _ := m.storage.LoadBrief(issue.ID)
	sessionID, _ := m.storage.LoadSessionID(issue.ID)

	// Try JSON analysis with selected option first
	if m.storage.AnalysisJSONExists(issue.ID) {
		analysis, err := m.storage.LoadAnalysisJSON(issue.ID)
		if err == nil && analysis != nil {
			prompt := claude.BuildPlanPromptWithOption(brief.Content, analysis)
			m.statusMsg = fmt.Sprintf("Planning %s with selected option...", issue.ID)
			m.claude.RunAsync(issue.ID, "plan", prompt, "", sessionID, m.resultChan)
			return m, nil
		}
	}

	// Fall back to markdown analysis
	analysisContent, _ := m.storage.LoadAnalysis(issue.ID)
	prompt := claude.BuildPlanPrompt(brief.Content, analysisContent)

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

	// Set view-only mode for closed issues
	m.isViewOnly = issue.Status.IsClosed()

	// Check for JSON analysis first (new option selection flow)
	if m.storage.AnalysisJSONExists(issue.ID) {
		analysis, err := m.storage.LoadAnalysisJSON(issue.ID)
		if err == nil && analysis != nil && len(analysis.Options) > 0 {
			m = m.enterOptionSelectState(analysis)
			return m, nil
		}
	}

	// Fall back to markdown analysis review
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

	// Set view-only mode for closed issues
	m.isViewOnly = issue.Status.IsClosed()

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

	analysisPath := m.storage.AnalysisPath(issue.ID)
	sessionID, _ := m.storage.LoadSessionID(issue.ID)

	prompt := claude.BuildReviewPrompt(analysisPath, feedback)

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

	planPath := m.storage.PlanPath(issue.ID)
	sessionID, _ := m.storage.LoadSessionID(issue.ID)

	prompt := claude.BuildPlanReviewPrompt(planPath, feedback)

	m.statusMsg = fmt.Sprintf("Reviewing plan %s...", issue.ID)
	m.claude.RunAsync(issue.ID, "plan-review", prompt, "", sessionID, m.resultChan)

	return m, nil
}

func (m Model) implementIssue() (Model, tea.Cmd) {
	issue := m.canModifySelectedIssue()
	if issue == nil {
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

	// Go to model selection (model selection serves as implicit confirmation)
	m.state = StateModelSelect
	m.pendingRetryIssue = issue
	m.pendingModel = ModelHaiku // default to fastest model
	m.modelCursor = 2          // haiku index
	return m, nil
}

func (m Model) executeImplementFor(issue *model.Issue) (Model, tea.Cmd) {
	// Stage issue files before implementation
	m.storage.StageIssueFiles(issue.ID)

	sessionID, _ := m.storage.LoadSessionID(issue.ID)
	planPath := m.storage.PlanPath(issue.ID)
	prompt := claude.BuildImplementPrompt(planPath)

	// Build command with model flag
	args := []string{"--resume", sessionID}
	if m.pendingModel != "" {
		args = append(args, "--model", string(m.pendingModel))
	}
	args = append(args, "--permission-mode", "acceptEdits", prompt)

	cmd := exec.Command("claude", args...)
	cmd.Dir = m.claude.WorkingDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	selectedModel := m.pendingModel
	m.statusMsg = fmt.Sprintf("Implementing %s with %s...", issue.ID, selectedModel)
	m.pendingModel = "" // reset after use

	issueID := issue.ID
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return implementCompletedMsg{issueID: issueID}
	})
}

func (m Model) updateChangeLog() (Model, tea.Cmd) {
	issue := m.canModifySelectedIssue()
	if issue == nil {
		return m, nil
	}

	// Only implemented issues can have change logs updated
	if issue.Status != model.StatusImplemented {
		m.statusMsg = "Only implemented issues can have change logs updated"
		return m, nil
	}

	// Check if plan exists
	if !m.storage.PlanExists(issue.ID) {
		m.statusMsg = "No plan.md found"
		return m, nil
	}

	// Prompt user for optional change reason
	m.state = StateInput
	m.inputMode = InputChangeReason
	m.inputPrompt = "Change reason (optional, press Enter to auto-detect): "
	m.textInput.Focus()
	return m, nil
}

func (m Model) executeUpdateChangeLog(changeReason string) (Model, tea.Cmd) {
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
	m.processing[issue.ID] = "update-changelog"
	m.processingLock.Unlock()

	// Load plan.md
	planContent, err := m.storage.LoadPlan(issue.ID)
	if err != nil {
		m.statusMsg = fmt.Sprintf("Failed to load plan: %v", err)
		return m, nil
	}

	// Get git diff
	gitDiff := m.storage.GetGitDiff()
	if gitDiff == "" {
		m.statusMsg = "No git changes detected"
		m.processingLock.Lock()
		delete(m.processing, issue.ID)
		m.processingLock.Unlock()
		return m, nil
	}

	// Build prompt and run Claude
	prompt := claude.BuildChangeLogPrompt(planContent, gitDiff, changeReason)
	m.statusMsg = fmt.Sprintf("Generating change log for %s...", issue.ID)
	m.claude.RunAsync(issue.ID, "update-changelog", prompt, "haiku", "", m.resultChan)

	return m, nil
}

func (m Model) executeAddOption(description string) (Model, tea.Cmd) {
	issue := m.getSelectedIssue()
	if issue == nil {
		m.statusMsg = "No issue selected"
		return m, nil
	}

	if m.analysis == nil {
		m.statusMsg = "No analysis loaded"
		return m, nil
	}

	m.processingLock.Lock()
	if _, ok := m.processing[issue.ID]; ok {
		m.processingLock.Unlock()
		m.statusMsg = fmt.Sprintf("%s is already processing", issue.ID)
		return m, nil
	}
	m.processing[issue.ID] = "add-option"
	m.processingLock.Unlock()

	sessionID, _ := m.storage.LoadSessionID(issue.ID)
	prompt := claude.BuildAddOptionPrompt(m.analysis, description)

	m.statusMsg = fmt.Sprintf("Adding option to %s...", issue.ID)
	m.claude.RunAsync(issue.ID, "add-option", prompt, "", sessionID, m.resultChan)

	return m, nil
}

func (m Model) executePlanWithOption(issue *model.Issue) (Model, tea.Cmd) {
	m.processingLock.Lock()
	if _, ok := m.processing[issue.ID]; ok {
		m.processingLock.Unlock()
		m.statusMsg = fmt.Sprintf("%s is already processing", issue.ID)
		return m, nil
	}
	m.processing[issue.ID] = "plan"
	m.processingLock.Unlock()

	brief, _ := m.storage.LoadBrief(issue.ID)
	analysis, err := m.storage.LoadAnalysisJSON(issue.ID)
	if err != nil || analysis == nil {
		m.statusMsg = "Failed to load analysis"
		m.processingLock.Lock()
		delete(m.processing, issue.ID)
		m.processingLock.Unlock()
		return m, nil
	}

	sessionID, _ := m.storage.LoadSessionID(issue.ID)
	prompt := claude.BuildPlanPromptWithOption(brief.Content, analysis)

	m.statusMsg = fmt.Sprintf("Planning %s with selected option...", issue.ID)
	m.claude.RunAsync(issue.ID, "plan", prompt, "", sessionID, m.resultChan)

	return m, nil
}

func (m *Model) handleResult(result claude.TaskResult) {
	m.processingLock.Lock()
	delete(m.processing, result.IssueID)
	m.processingLock.Unlock()

	switch result.TaskType {
	case "analyze":
		if result.Success {
			// Try to parse as JSON first
			analysis, parseErr := storage.ParseAnalysisFromRaw(result.Result)
			if parseErr == nil && analysis != nil {
				// Save as JSON
				if saveErr := m.storage.SaveAnalysisJSON(result.IssueID, analysis); saveErr == nil {
					if result.SessionID != "" {
						_ = m.storage.SaveSessionID(result.IssueID, result.SessionID)
					}
					_ = m.storage.UpdateIssueStatus(result.IssueID, model.StatusAnalyzed, "")
					m.statusMsg = fmt.Sprintf("Analyzed %s - press R to review options", result.IssueID)
					return
				}
			}

			// Fallback to markdown if JSON parsing fails
			_ = m.storage.SaveAnalysis(result.IssueID, result.Result)
			if result.SessionID != "" {
				_ = m.storage.SaveSessionID(result.IssueID, result.SessionID)
			}
			_ = m.storage.UpdateIssueStatus(result.IssueID, model.StatusAnalyzed, "")
			// Provide more informative status message when JSON parsing failed
			if parseErr != nil {
				m.statusMsg = fmt.Sprintf("Analyzed %s (JSON error: saved as text) - try 'a' again", result.IssueID)
			} else {
				m.statusMsg = fmt.Sprintf("Analyzed %s (text mode)", result.IssueID)
			}
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
	case "add-option":
		if result.Success {
			// Parse the new option
			newOption, err := storage.ExtractOptionFromRaw(result.Result)
			if err == nil && newOption != nil {
				// Load current analysis and add the option
				analysis, loadErr := m.storage.LoadAnalysisJSON(result.IssueID)
				if loadErr == nil && analysis != nil {
					analysis.AddOption(*newOption)
					if saveErr := m.storage.SaveAnalysisJSON(result.IssueID, analysis); saveErr == nil {
						m.analysis = analysis
						m.optionCursor = len(analysis.Options) - 1 // Move cursor to new option
						m.updateDetailViewport()
						m.statusMsg = fmt.Sprintf("Added option: %s", newOption.Title)
						return
					}
				}
			}
			m.statusMsg = fmt.Sprintf("Failed to add option: %v", err)
		} else {
			m.statusMsg = fmt.Sprintf("Add option failed for %s", result.IssueID)
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
	case "update-changelog":
		if result.Success {
			changeLogEntry := strings.TrimSpace(result.Result)
			// Replace {{DATE}} with actual date
			changeLogEntry = strings.ReplaceAll(changeLogEntry, "{{DATE}}", time.Now().Format("2006-01-02"))
			// Append the change log to plan.md
			if err := m.storage.AppendChangeLog(result.IssueID, changeLogEntry); err != nil {
				m.statusMsg = fmt.Sprintf("Failed to update plan.md: %v", err)
			} else {
				m.statusMsg = fmt.Sprintf("Change log updated for %s", result.IssueID)
			}
		} else {
			m.statusMsg = fmt.Sprintf("Change log generation failed: %s", result.IssueID)
		}
	}
}

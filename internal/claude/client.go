package claude

import (
	"encoding/json"
	"os/exec"
	"strings"
)

// Response represents the Claude CLI JSON response
type Response struct {
	Result    string `json:"result"`
	SessionID string `json:"session_id,omitempty"`
}

// TaskResult represents the result of an async Claude task
type TaskResult struct {
	IssueID   string
	TaskType  string // "analyze" or "plan"
	Success   bool
	Result    string
	SessionID string
}

// Client handles Claude CLI interactions
type Client struct {
	WorkingDir string
}

// New creates a new Claude client
func New(workingDir string) *Client {
	return &Client{WorkingDir: workingDir}
}

// Run executes Claude CLI and returns (success, result, sessionID)
func (c *Client) Run(prompt string, model string, resumeSession string) (bool, string, string) {
	args := []string{"--output-format", "json"}

	if model != "" {
		args = append(args, "--model", model)
	}
	if resumeSession != "" {
		args = append(args, "--resume", resumeSession)
	}
	args = append(args, "-p", prompt)

	cmd := exec.Command("claude", args...)
	cmd.Dir = c.WorkingDir

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return false, string(exitErr.Stderr), ""
		}
		return false, err.Error(), ""
	}

	return c.parseResponse(string(output))
}

// RunAsync executes Claude CLI in a goroutine and sends result to channel
func (c *Client) RunAsync(issueID, taskType, prompt, model, resumeSession string, resultChan chan<- TaskResult) {
	go func() {
		success, result, sessionID := c.Run(prompt, model, resumeSession)
		resultChan <- TaskResult{
			IssueID:   issueID,
			TaskType:  taskType,
			Success:   success,
			Result:    result,
			SessionID: sessionID,
		}
	}()
}

// parseResponse parses Claude CLI JSON output
func (c *Client) parseResponse(jsonOutput string) (bool, string, string) {
	var resp Response
	if err := json.Unmarshal([]byte(jsonOutput), &resp); err != nil {
		// Fallback: treat as plain text
		return true, strings.TrimSpace(jsonOutput), ""
	}
	return true, resp.Result, resp.SessionID
}

// IsAvailable checks if claude CLI is available
func IsAvailable() bool {
	cmd := exec.Command("claude", "--version")
	return cmd.Run() == nil
}

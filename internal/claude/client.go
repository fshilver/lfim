package claude

import (
	"encoding/json"
	"os/exec"
	"strings"
)

// Response represents the Claude CLI JSON response
type Response struct {
	Result           string                 `json:"result"`
	SessionID        string                 `json:"session_id,omitempty"`
	StructuredOutput map[string]interface{} `json:"structured_output,omitempty"`
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

// parseResponseWithSchema parses Claude CLI JSON output with structured_output
func (c *Client) parseResponseWithSchema(jsonOutput string) (bool, string, string) {
	var resp Response
	if err := json.Unmarshal([]byte(jsonOutput), &resp); err != nil {
		// Fallback: treat as plain text
		return true, strings.TrimSpace(jsonOutput), ""
	}

	// If structured_output exists and is not empty, marshal it back to JSON string
	if len(resp.StructuredOutput) > 0 {
		jsonBytes, err := json.Marshal(resp.StructuredOutput)
		if err == nil {
			return true, string(jsonBytes), resp.SessionID
		}
	}

	// Fallback to result field for backward compatibility
	return true, resp.Result, resp.SessionID
}

// IsAvailable checks if claude CLI is available
func IsAvailable() bool {
	cmd := exec.Command("claude", "--version")
	return cmd.Run() == nil
}

// compressJSON compresses a JSON string by removing all whitespace
// Returns the original string if compression fails
func compressJSON(jsonStr string) string {
	var obj interface{}
	if err := json.Unmarshal([]byte(jsonStr), &obj); err != nil {
		return jsonStr // Return original if invalid JSON
	}
	compressed, err := json.Marshal(obj)
	if err != nil {
		return jsonStr // Return original if marshaling fails
	}
	return string(compressed)
}

// RunWithJSONSchema executes Claude CLI with JSON schema and returns (success, result, sessionID)
func (c *Client) RunWithJSONSchema(prompt string, schema string, model string, resumeSession string) (bool, string, string) {
	// Compress schema for CLI argument (remove whitespace)
	compressedSchema := compressJSON(schema)
	args := []string{"--output-format", "json", "--json-schema", compressedSchema}

	if model != "" {
		args = append(args, "--model", model)
	}
	if resumeSession != "" {
		args = append(args, "--resume", resumeSession)
	}
	args = append(args, "-p", prompt)

	cmd := exec.Command("claude", args...)
	cmd.Dir = c.WorkingDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Include stderr in error message for better debugging
			stderr := string(exitErr.Stderr)
			if stderr == "" {
				stderr = string(output)
			}
			return false, stderr, ""
		}
		return false, err.Error(), ""
	}

	return c.parseResponseWithSchema(string(output))
}

// RunWithJSONSchemaAsync executes Claude CLI with JSON schema in a goroutine
func (c *Client) RunWithJSONSchemaAsync(issueID, taskType, prompt, schema, model, resumeSession string, resultChan chan<- TaskResult) {
	go func() {
		success, result, sessionID := c.RunWithJSONSchema(prompt, schema, model, resumeSession)
		resultChan <- TaskResult{
			IssueID:   issueID,
			TaskType:  taskType,
			Success:   success,
			Result:    result,
			SessionID: sessionID,
		}
	}()
}

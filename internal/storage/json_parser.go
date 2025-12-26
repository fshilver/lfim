package storage

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/lunit-heesungyang/issue-manager/internal/model"
)

// ExtractJSON extracts JSON content from a string that may contain code blocks or extra text
func ExtractJSON(rawOutput string) (string, error) {
	// First, try to extract from ```json ... ``` code block
	jsonBlockRegex := regexp.MustCompile("(?s)```json\\s*\\n?(.*?)\\n?```")
	if matches := jsonBlockRegex.FindStringSubmatch(rawOutput); len(matches) > 1 {
		return strings.TrimSpace(matches[1]), nil
	}

	// Try to extract from ``` ... ``` code block (without json tag)
	codeBlockRegex := regexp.MustCompile("(?s)```\\s*\\n?(.*?)\\n?```")
	if matches := codeBlockRegex.FindStringSubmatch(rawOutput); len(matches) > 1 {
		content := strings.TrimSpace(matches[1])
		// Verify it looks like JSON
		if strings.HasPrefix(content, "{") {
			return content, nil
		}
	}

	// Try to find raw JSON object (from first { to last })
	trimmed := strings.TrimSpace(rawOutput)
	firstBrace := strings.Index(trimmed, "{")
	lastBrace := strings.LastIndex(trimmed, "}")

	if firstBrace != -1 && lastBrace != -1 && lastBrace > firstBrace {
		return trimmed[firstBrace : lastBrace+1], nil
	}

	return "", fmt.Errorf("no valid JSON found in output")
}

// ValidateAnalysisJSON validates that the JSON has required fields
func ValidateAnalysisJSON(data []byte) error {
	var analysis model.Analysis
	if err := json.Unmarshal(data, &analysis); err != nil {
		return fmt.Errorf("invalid JSON structure: %w", err)
	}

	// Check required fields
	if analysis.Summary == "" {
		return fmt.Errorf("missing required field: summary")
	}

	if len(analysis.Options) == 0 {
		return fmt.Errorf("at least one option is required")
	}

	// Validate each option
	for i, opt := range analysis.Options {
		if opt.ID == "" {
			return fmt.Errorf("option %d: missing required field: id", i+1)
		}
		if opt.Title == "" {
			return fmt.Errorf("option %d: missing required field: title", i+1)
		}
	}

	return nil
}

// ParseAnalysisFromRaw parses analysis JSON from raw Claude output
func ParseAnalysisFromRaw(rawOutput string) (*model.Analysis, error) {
	// Extract JSON from potential code blocks
	jsonStr, err := ExtractJSON(rawOutput)
	if err != nil {
		return nil, err
	}

	// Validate the JSON
	if err := ValidateAnalysisJSON([]byte(jsonStr)); err != nil {
		return nil, err
	}

	// Parse into struct
	return model.ParseAnalysis([]byte(jsonStr))
}

// ExtractOptionFromRaw extracts a single option from raw Claude output
func ExtractOptionFromRaw(rawOutput string) (*model.AnalysisOption, error) {
	jsonStr, err := ExtractJSON(rawOutput)
	if err != nil {
		return nil, err
	}

	var option model.AnalysisOption
	if err := json.Unmarshal([]byte(jsonStr), &option); err != nil {
		return nil, fmt.Errorf("failed to parse option JSON: %w", err)
	}

	if option.ID == "" {
		return nil, fmt.Errorf("missing required field: id")
	}
	if option.Title == "" {
		return nil, fmt.Errorf("missing required field: title")
	}

	return &option, nil
}

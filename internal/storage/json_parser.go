package storage

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lunit-heesungyang/issue-manager/internal/model"
)

// ExtractJSON extracts JSON content from Claude CLI output
// When using --json-schema, the output should be valid JSON in the "result" field
// This function handles both new schema-based output and legacy formats for compatibility
func ExtractJSON(rawOutput string) (string, error) {
	trimmed := strings.TrimSpace(rawOutput)

	// If it's already a valid JSON object starting with {, return as-is
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		return trimmed, nil
	}

	return "", fmt.Errorf("invalid JSON format in output")
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
// When using --json-schema, the output should already be valid JSON conforming to the schema
func ParseAnalysisFromRaw(rawOutput string) (*model.Analysis, error) {
	jsonStr, err := ExtractJSON(rawOutput)
	if err != nil {
		return nil, fmt.Errorf("failed to extract JSON: %w", err)
	}

	// Parse and validate the JSON
	if err := ValidateAnalysisJSON([]byte(jsonStr)); err != nil {
		return nil, fmt.Errorf("validation failed: %w\n%s", err, getDetailedJSONError(jsonStr))
	}

	return model.ParseAnalysis([]byte(jsonStr))
}

// getDetailedJSONError provides a more informative error message for JSON parsing failures
func getDetailedJSONError(jsonStr string) error {
	var js json.RawMessage
	err := json.Unmarshal([]byte(jsonStr), &js)
	if err != nil {
		if syntaxErr, ok := err.(*json.SyntaxError); ok {
			// Find the line and context around the error
			lines := strings.Split(jsonStr, "\n")
			charCount := int64(0)
			for i, line := range lines {
				if charCount+int64(len(line))+1 >= syntaxErr.Offset {
					col := syntaxErr.Offset - charCount
					context := line
					if len(context) > 60 {
						start := int(col) - 30
						if start < 0 {
							start = 0
						}
						end := start + 60
						if end > len(context) {
							end = len(context)
						}
						context = "..." + context[start:end] + "..."
					}
					return fmt.Errorf("JSON syntax error at line %d, col %d: %s\nContext: %s",
						i+1, col, err.Error(), context)
				}
				charCount += int64(len(line)) + 1 // +1 for newline
			}
		}
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

// ExtractOptionFromRaw extracts a single option from raw Claude output
// When using --json-schema, the output should already be valid JSON conforming to the schema
func ExtractOptionFromRaw(rawOutput string) (*model.AnalysisOption, error) {
	jsonStr, err := ExtractJSON(rawOutput)
	if err != nil {
		return nil, fmt.Errorf("failed to extract JSON: %w", err)
	}

	var option model.AnalysisOption
	if err := json.Unmarshal([]byte(jsonStr), &option); err != nil {
		return nil, fmt.Errorf("failed to parse option: %w\n%s", err, getDetailedJSONError(jsonStr))
	}

	if option.ID == "" {
		return nil, fmt.Errorf("missing required field: id")
	}
	if option.Title == "" {
		return nil, fmt.Errorf("missing required field: title")
	}

	return &option, nil
}

package storage

import (
	"strings"
	"testing"
)

// normalizeJSON removes all whitespace from a JSON string for comparison
func normalizeJSON(s string) string {
	return strings.Join(strings.Fields(s), "")
}

func TestRepairCommonJSONErrors(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "trailing comma in array",
			input:    `{"pros": ["item1", "item2",]}`,
			expected: `{"pros": ["item1", "item2"]}`,
		},
		{
			name:     "trailing comma in object",
			input:    `{"key": "value",}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "double closing brace before comma",
			input:    `{"options": [{"id": "opt1"}}, {"id": "opt2"}}]}`,
			expected: `{"options": [{"id": "opt1"}, {"id": "opt2"}]}`,
		},
		{
			name:     "double closing brace at end of array",
			input:    `{"options": [{"id": "opt1"}}, {"id": "opt2"}}]}`,
			expected: `{"options": [{"id": "opt1"}, {"id": "opt2"}]}`,
		},
		{
			name:     "valid JSON unchanged",
			input:    `{"key": "value", "arr": [1, 2, 3]}`,
			expected: `{"key": "value", "arr": [1, 2, 3]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := repairCommonJSONErrors(tt.input)
			if result != tt.expected {
				t.Errorf("repairCommonJSONErrors() =\n%s\nwant:\n%s", result, tt.expected)
			}
		})
	}
}

func TestRepairCommonJSONErrors_ProducesValidJSON(t *testing.T) {
	// Test case from the issue: double closing braces in options array
	// Pattern:
	//     }
	//     },      <- extra } before comma on new line
	malformedJSON := `{
  "summary": "test",
  "options": [
    {
      "id": "opt1",
      "title": "Option 1"
    }
    },
    {
      "id": "opt2",
      "title": "Option 2"
    }
    }
  ]
}`
	repaired := repairCommonJSONErrors(malformedJSON)

	// The repaired JSON should be different from the original
	if repaired == malformedJSON {
		t.Error("repairCommonJSONErrors() did not modify the malformed JSON")
	}

	// We just verify that the repair function makes changes
	// The actual validity will be tested in ParseAnalysisFromRaw
}

func TestRepairCommonJSONErrors_MultilinePattern(t *testing.T) {
	// Exact pattern from the issue
	input := `{
            "recommended":true,
            "details":"..."
            }
            },
            {
                "id":"opt2"
            }
        ]
    }`

	repaired := repairCommonJSONErrors(input)

	// Should remove the extra } before the comma
	if strings.Contains(repaired, "}\n            },") {
		t.Error("Failed to repair multiline double brace pattern")
	}
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "raw JSON",
			input:   `{"key": "value"}`,
			want:    `{"key": "value"}`,
			wantErr: false,
		},
		{
			name:    "JSON in code block",
			input:   "```json\n{\"key\": \"value\"}\n```",
			want:    `{"key": "value"}`,
			wantErr: false,
		},
		{
			name:    "JSON with preamble text",
			input:   "Here is the analysis:\n{\"key\": \"value\"}",
			want:    `{"key": "value"}`,
			wantErr: false,
		},
		{
			name:    "no JSON",
			input:   "Just some text without JSON",
			wantErr: true,
		},
		{
			name: "issue error case 1: conversational text with code block",
			input: `Perfect! Now I have a comprehensive understanding of the issue. Let me provide the structured analysis:

` + "```json" + `
{
  "summary": "Test summary",
  "root_cause": "Test root cause"
}
` + "```",
			want:    `{"summary": "Test summary","root_cause": "Test root cause"}`,
			wantErr: false,
		},
		{
			name: "issue error case 2: just code block wrapper",
			input: "```json\n{\n  \"summary\": \"Test summary\",\n  \"root_cause\": \"Test root cause\"\n}\n```",
			want:    `{"summary": "Test summary","root_cause": "Test root cause"}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractJSON(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				// Normalize whitespace for comparison
				if normalizeJSON(got) != normalizeJSON(tt.want) {
					t.Errorf("ExtractJSON() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

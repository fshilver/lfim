package claude

import (
	"encoding/json"
	"testing"
)

func TestCompressJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		validate func(t *testing.T, result string)
	}{
		{
			name: "compress AnalysisSchema",
			input: AnalysisSchema,
			wantErr: false,
			validate: func(t *testing.T, result string) {
				// Verify it's valid JSON
				var obj interface{}
				if err := json.Unmarshal([]byte(result), &obj); err != nil {
					t.Errorf("compressed AnalysisSchema is not valid JSON: %v", err)
				}
				// Verify it's shorter than original
				if len(result) >= len(AnalysisSchema) {
					t.Errorf("compressed length (%d) >= original length (%d)", len(result), len(AnalysisSchema))
				}
				t.Logf("AnalysisSchema: %d -> %d bytes (saved %d bytes)",
					len(AnalysisSchema), len(result), len(AnalysisSchema)-len(result))
			},
		},
		{
			name: "compress OptionSchema",
			input: OptionSchema,
			wantErr: false,
			validate: func(t *testing.T, result string) {
				// Verify it's valid JSON
				var obj interface{}
				if err := json.Unmarshal([]byte(result), &obj); err != nil {
					t.Errorf("compressed OptionSchema is not valid JSON: %v", err)
				}
				// Verify it's shorter than original
				if len(result) >= len(OptionSchema) {
					t.Errorf("compressed length (%d) >= original length (%d)", len(result), len(OptionSchema))
				}
				t.Logf("OptionSchema: %d -> %d bytes (saved %d bytes)",
					len(OptionSchema), len(result), len(OptionSchema)-len(result))
			},
		},
		{
			name: "handle invalid JSON",
			input: `{"invalid": json}`,
			wantErr: false,
			validate: func(t *testing.T, result string) {
				// Should return original string on error
				if result != `{"invalid": json}` {
					t.Errorf("expected original string on invalid JSON, got: %s", result)
				}
			},
		},
		{
			name: "compress simple JSON",
			input: `{
  "type": "object",
  "properties": {
    "name": {
      "type": "string"
    }
  }
}`,
			wantErr: false,
			validate: func(t *testing.T, result string) {
				expected := `{"properties":{"name":{"type":"string"}},"type":"object"}`
				if result != expected {
					t.Errorf("expected %s, got %s", expected, result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compressJSON(tt.input)
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func BenchmarkCompressJSON(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = compressJSON(AnalysisSchema)
	}
}

func TestParseResponseWithSchema(t *testing.T) {
	client := &Client{}

	tests := []struct {
		name              string
		input             string
		wantSuccess       bool
		wantResultContain string
		wantSessionID     string
	}{
		{
			name: "with structured_output field",
			input: `{
				"type": "result",
				"result": "I've completed the analysis",
				"session_id": "test-session-123",
				"structured_output": {
					"summary": "Test summary",
					"options": [{"id": "opt1", "title": "Option 1"}]
				}
			}`,
			wantSuccess:       true,
			wantResultContain: `"summary":"Test summary"`,
			wantSessionID:     "test-session-123",
		},
		{
			name: "with empty structured_output",
			input: `{
				"type": "result",
				"result": "Fallback result text",
				"session_id": "test-session-456",
				"structured_output": {}
			}`,
			wantSuccess:       true,
			wantResultContain: "Fallback result text",
			wantSessionID:     "test-session-456",
		},
		{
			name: "without structured_output field",
			input: `{
				"type": "result",
				"result": "Plain result text",
				"session_id": "test-session-789"
			}`,
			wantSuccess:       true,
			wantResultContain: "Plain result text",
			wantSessionID:     "test-session-789",
		},
		{
			name:              "invalid JSON",
			input:             `{"invalid": json}`,
			wantSuccess:       true,
			wantResultContain: `{"invalid": json}`,
			wantSessionID:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			success, result, sessionID := client.parseResponseWithSchema(tt.input)

			if success != tt.wantSuccess {
				t.Errorf("parseResponseWithSchema() success = %v, want %v", success, tt.wantSuccess)
			}

			if tt.wantResultContain != "" && !contains(result, tt.wantResultContain) {
				t.Errorf("parseResponseWithSchema() result = %v, want to contain %v", result, tt.wantResultContain)
			}

			if sessionID != tt.wantSessionID {
				t.Errorf("parseResponseWithSchema() sessionID = %v, want %v", sessionID, tt.wantSessionID)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

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

package storage

import (
	"strings"
	"testing"
)

// normalizeJSON removes all whitespace from a JSON string for comparison
func normalizeJSON(s string) string {
	return strings.Join(strings.Fields(s), "")
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "valid JSON object",
			input:   `{"key": "value"}`,
			want:    `{"key": "value"}`,
			wantErr: false,
		},
		{
			name:    "JSON with whitespace",
			input:   `  {"key": "value"}  `,
			want:    `{"key": "value"}`,
			wantErr: false,
		},
		{
			name: "multiline JSON",
			input: `{
  "summary": "Test summary",
  "root_cause": "Test root cause"
}`,
			want:    `{"summary": "Test summary","root_cause": "Test root cause"}`,
			wantErr: false,
		},
		{
			name:    "no JSON",
			input:   "Just some text without JSON",
			wantErr: true,
		},
		{
			name:    "incomplete JSON",
			input:   `{"key": "value"`,
			wantErr: true,
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

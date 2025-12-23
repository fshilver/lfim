package storage

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var frontmatterRegex = regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---\s*\n?(.*)`)

// ParseFrontmatter extracts YAML frontmatter and body from markdown content
func ParseFrontmatter(content string) (map[string]interface{}, string, error) {
	matches := frontmatterRegex.FindStringSubmatch(content)
	if matches == nil {
		return nil, strings.TrimSpace(content), nil
	}

	var fm map[string]interface{}
	if err := yaml.Unmarshal([]byte(matches[1]), &fm); err != nil {
		return nil, "", fmt.Errorf("parsing frontmatter: %w", err)
	}

	body := strings.TrimSpace(matches[2])
	return fm, body, nil
}

// CreateFrontmatter creates markdown content with YAML frontmatter
func CreateFrontmatter(data map[string]interface{}, body string) (string, error) {
	yamlBytes, err := yaml.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("marshaling frontmatter: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.Write(yamlBytes)
	sb.WriteString("---\n")
	if body != "" {
		sb.WriteString("\n")
		sb.WriteString(body)
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// GetString safely extracts a string from a map
// For "id" key, it formats integers with 4-digit zero padding
func GetString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case string:
			return val
		case int:
			if key == "id" {
				return fmt.Sprintf("%04d", val)
			}
			return fmt.Sprintf("%d", val)
		case int64:
			if key == "id" {
				return fmt.Sprintf("%04d", val)
			}
			return fmt.Sprintf("%d", val)
		case float64:
			if key == "id" {
				return fmt.Sprintf("%04d", int(val))
			}
			return fmt.Sprintf("%.0f", val)
		}
	}
	return ""
}

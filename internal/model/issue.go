package model

import (
	"time"
)

// IssueStatus represents the lifecycle status of an issue
type IssueStatus string

const (
	StatusOpen        IssueStatus = "open"
	StatusAnalyzed    IssueStatus = "analyzed"
	StatusPlanned     IssueStatus = "planned"
	StatusImplemented IssueStatus = "implemented"
	StatusClosed      IssueStatus = "closed"
	StatusInvalid     IssueStatus = "invalid"
)

// IsActive returns true if the status is not closed or invalid
func (s IssueStatus) IsActive() bool {
	return s != StatusClosed && s != StatusInvalid
}

// IsClosed returns true if the status is closed or invalid
func (s IssueStatus) IsClosed() bool {
	return s == StatusClosed || s == StatusInvalid
}

// IssueType represents the category of an issue
type IssueType string

const (
	TypeFeature  IssueType = "feature"
	TypeBug      IssueType = "bug"
	TypeRefactor IssueType = "refactor"
)

// Icon returns the display emoji for this issue type
func (t IssueType) Icon() string {
	switch t {
	case TypeFeature:
		return "💡"
	case TypeBug:
		return "💥"
	case TypeRefactor:
		return "🔧"
	default:
		return "❓"
	}
}

// Issue represents a single issue
type Issue struct {
	ID            string      `yaml:"id"`
	Title         string      `yaml:"title"`
	Type          IssueType   `yaml:"type"`
	Status        IssueStatus `yaml:"status"`
	Created       time.Time   `yaml:"created"`
	Content       string      `yaml:"-"` // Not stored in index.yaml
	DiscardReason string      `yaml:"discard_reason,omitempty"`
}

// ToIndexEntry returns a map for index.yaml serialization
func (i *Issue) ToIndexEntry() map[string]interface{} {
	return map[string]interface{}{
		"id":      i.ID,
		"title":   i.Title,
		"type":    string(i.Type),
		"status":  string(i.Status),
		"created": i.Created.Format("2006-01-02"),
	}
}

// ToFrontmatter returns a map for brief.md frontmatter
func (i *Issue) ToFrontmatter() map[string]interface{} {
	fm := map[string]interface{}{
		"title":  i.Title,
		"type":   string(i.Type),
		"status": string(i.Status),
		"date":   i.Created.Format("2006-01-02"),
	}
	if i.DiscardReason != "" {
		fm["discard_reason"] = i.DiscardReason
	}
	return fm
}

// StatusIcon returns the display icon for this issue's status
func (i *Issue) StatusIcon() string {
	switch i.Status {
	case StatusOpen:
		return "○"
	case StatusAnalyzed:
		return "◐"
	case StatusPlanned:
		return "●"
	case StatusImplemented:
		return "◉"
	case StatusClosed:
		return "✓"
	case StatusInvalid:
		return "✗"
	default:
		return "?"
	}
}

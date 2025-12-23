package model

import (
	"fmt"
	"sort"
)

// IssueIndex represents the collection of all issues
type IssueIndex struct {
	Issues []*Issue
}

// NewIssueIndex creates a new empty issue index
func NewIssueIndex() *IssueIndex {
	return &IssueIndex{
		Issues: make([]*Issue, 0),
	}
}

// GetNextID generates the next sequential issue ID (4-digit zero-padded)
func (idx *IssueIndex) GetNextID() string {
	if len(idx.Issues) == 0 {
		return "0001"
	}

	maxID := 0
	for _, issue := range idx.Issues {
		var id int
		fmt.Sscanf(issue.ID, "%d", &id)
		if id > maxID {
			maxID = id
		}
	}
	return fmt.Sprintf("%04d", maxID+1)
}

// AddIssue adds an issue to the index
func (idx *IssueIndex) AddIssue(issue *Issue) {
	idx.Issues = append(idx.Issues, issue)
}

// GetIssue finds an issue by ID
func (idx *IssueIndex) GetIssue(id string) *Issue {
	for _, issue := range idx.Issues {
		if issue.ID == id {
			return issue
		}
	}
	return nil
}

// UpdateIssue updates an existing issue in the index
func (idx *IssueIndex) UpdateIssue(updated *Issue) {
	for i, issue := range idx.Issues {
		if issue.ID == updated.ID {
			idx.Issues[i] = updated
			return
		}
	}
}

// GetActiveIssues returns issues that are not closed or invalid
func (idx *IssueIndex) GetActiveIssues() []*Issue {
	return idx.FilterByStatus(StatusOpen, StatusAnalyzed, StatusPlanned)
}

// GetClosedIssues returns issues that are closed or invalid
func (idx *IssueIndex) GetClosedIssues() []*Issue {
	return idx.FilterByStatus(StatusClosed, StatusInvalid)
}

// FilterByStatus returns issues matching the given statuses
func (idx *IssueIndex) FilterByStatus(statuses ...IssueStatus) []*Issue {
	statusSet := make(map[IssueStatus]bool)
	for _, s := range statuses {
		statusSet[s] = true
	}

	var filtered []*Issue
	for _, issue := range idx.Issues {
		if statusSet[issue.Status] {
			filtered = append(filtered, issue)
		}
	}
	return filtered
}

// SortByCreated sorts issues by creation date (newest first)
func (idx *IssueIndex) SortByCreated() {
	sort.Slice(idx.Issues, func(i, j int) bool {
		return idx.Issues[i].Created.After(idx.Issues[j].Created)
	})
}

// SortByID sorts issues by ID (ascending)
func (idx *IssueIndex) SortByID() {
	sort.Slice(idx.Issues, func(i, j int) bool {
		return idx.Issues[i].ID < idx.Issues[j].ID
	})
}

// ToYAML returns a map for YAML serialization
func (idx *IssueIndex) ToYAML() map[string]interface{} {
	issues := make([]map[string]interface{}, 0, len(idx.Issues))
	for _, issue := range idx.Issues {
		issues = append(issues, issue.ToIndexEntry())
	}
	return map[string]interface{}{
		"issues": issues,
	}
}

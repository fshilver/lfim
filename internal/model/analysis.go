package model

import (
	"encoding/json"
	"fmt"
)

// AnalysisOption represents a single implementation option
type AnalysisOption struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Pros        []string `json:"pros"`
	Cons        []string `json:"cons"`
	Recommended bool     `json:"recommended"`
	Details     string   `json:"details,omitempty"`
}

// Analysis represents the structured analysis result
type Analysis struct {
	Summary          string           `json:"summary"`
	RootCause        string           `json:"root_cause"`
	Options          []AnalysisOption `json:"options"`
	RiskAssessment   string           `json:"risk_assessment"`
	SelectedOptionID string           `json:"selected_option_id,omitempty"`
}

// GetRecommendedOption returns the recommended option, or nil if none
func (a *Analysis) GetRecommendedOption() *AnalysisOption {
	for i := range a.Options {
		if a.Options[i].Recommended {
			return &a.Options[i]
		}
	}
	return nil
}

// GetSelectedOption returns the selected option, or the recommended one if not selected
func (a *Analysis) GetSelectedOption() *AnalysisOption {
	// First try to find explicitly selected option
	if a.SelectedOptionID != "" {
		for i := range a.Options {
			if a.Options[i].ID == a.SelectedOptionID {
				return &a.Options[i]
			}
		}
	}
	// Fall back to recommended option
	return a.GetRecommendedOption()
}

// GetOptionByID returns an option by its ID
func (a *Analysis) GetOptionByID(id string) *AnalysisOption {
	for i := range a.Options {
		if a.Options[i].ID == id {
			return &a.Options[i]
		}
	}
	return nil
}

// SetSelectedOption sets the selected option ID
func (a *Analysis) SetSelectedOption(optionID string) error {
	// Verify option exists
	if a.GetOptionByID(optionID) == nil {
		return fmt.Errorf("option with ID %q not found", optionID)
	}
	a.SelectedOptionID = optionID
	return nil
}

// AddOption adds a new option to the analysis
func (a *Analysis) AddOption(option AnalysisOption) {
	a.Options = append(a.Options, option)
}

// GetRecommendedIndex returns the index of the recommended option, or 0 if none
func (a *Analysis) GetRecommendedIndex() int {
	for i := range a.Options {
		if a.Options[i].Recommended {
			return i
		}
	}
	return 0
}

// GetSelectedIndex returns the index of the selected option
func (a *Analysis) GetSelectedIndex() int {
	if a.SelectedOptionID != "" {
		for i := range a.Options {
			if a.Options[i].ID == a.SelectedOptionID {
				return i
			}
		}
	}
	return a.GetRecommendedIndex()
}

// ToJSON serializes the analysis to JSON
func (a *Analysis) ToJSON() ([]byte, error) {
	return json.MarshalIndent(a, "", "  ")
}

// ParseAnalysis parses JSON data into an Analysis struct
func ParseAnalysis(data []byte) (*Analysis, error) {
	var analysis Analysis
	if err := json.Unmarshal(data, &analysis); err != nil {
		return nil, fmt.Errorf("failed to parse analysis JSON: %w", err)
	}
	return &analysis, nil
}

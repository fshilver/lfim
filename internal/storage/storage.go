package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/lunit-heesungyang/issue-manager/internal/model"
	"gopkg.in/yaml.v3"
)

// Storage handles reading and writing issue files
type Storage struct {
	ProjectRoot string
	IssuesDir   string
}

// New creates a new Storage instance
func New(projectRoot string) *Storage {
	if projectRoot == "" {
		projectRoot, _ = os.Getwd()
	}
	return &Storage{
		ProjectRoot: projectRoot,
		IssuesDir:   filepath.Join(projectRoot, "issues"),
	}
}

// EnsureIssuesDir creates the issues directory if it doesn't exist
func (s *Storage) EnsureIssuesDir() error {
	return os.MkdirAll(s.IssuesDir, 0755)
}

// Path helpers
func (s *Storage) IndexPath() string {
	return filepath.Join(s.IssuesDir, "index.yaml")
}

func (s *Storage) IssueDir(issueID string) string {
	return filepath.Join(s.IssuesDir, issueID)
}

func (s *Storage) BriefPath(issueID string) string {
	return filepath.Join(s.IssueDir(issueID), "brief.md")
}

func (s *Storage) AnalysisPath(issueID string) string {
	return filepath.Join(s.IssueDir(issueID), "analysis.md")
}

func (s *Storage) PlanPath(issueID string) string {
	return filepath.Join(s.IssueDir(issueID), "plan.md")
}

func (s *Storage) SessionPath(issueID string) string {
	return filepath.Join(s.IssueDir(issueID), ".session")
}

func (s *Storage) AnalysisVersionPath(issueID string, version int) string {
	return filepath.Join(s.IssueDir(issueID), fmt.Sprintf("analysis_v%d.md", version))
}

func (s *Storage) VersionTrackerPath(issueID string) string {
	return filepath.Join(s.IssueDir(issueID), ".analysis_version")
}

// LoadIndex loads the issue index from index.yaml
func (s *Storage) LoadIndex() (*model.IssueIndex, error) {
	data, err := os.ReadFile(s.IndexPath())
	if os.IsNotExist(err) {
		return model.NewIssueIndex(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading index: %w", err)
	}

	var raw struct {
		Issues []map[string]interface{} `yaml:"issues"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing index: %w", err)
	}

	idx := model.NewIssueIndex()
	for _, item := range raw.Issues {
		issue, err := s.issueFromMap(item)
		if err != nil {
			return nil, err
		}
		idx.AddIssue(issue)
	}
	return idx, nil
}

// SaveIndex saves the issue index to index.yaml
func (s *Storage) SaveIndex(idx *model.IssueIndex) error {
	if err := s.EnsureIssuesDir(); err != nil {
		return err
	}

	data, err := yaml.Marshal(idx.ToYAML())
	if err != nil {
		return fmt.Errorf("marshaling index: %w", err)
	}

	return os.WriteFile(s.IndexPath(), data, 0644)
}

// LoadBrief loads an issue from its brief.md file
func (s *Storage) LoadBrief(issueID string) (*model.Issue, error) {
	data, err := os.ReadFile(s.BriefPath(issueID))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading brief: %w", err)
	}

	fm, body, err := ParseFrontmatter(string(data))
	if err != nil {
		return nil, err
	}

	issue := &model.Issue{
		ID:      issueID,
		Content: body,
	}

	issue.Title = GetString(fm, "title")
	issue.Type = model.IssueType(GetString(fm, "type"))
	issue.Status = model.IssueStatus(GetString(fm, "status"))
	issue.DiscardReason = GetString(fm, "discard_reason")

	if dateStr := GetString(fm, "date"); dateStr != "" {
		issue.Created, _ = time.Parse("2006-01-02", dateStr)
	}

	return issue, nil
}

// SaveBrief saves an issue to its brief.md file
func (s *Storage) SaveBrief(issue *model.Issue) error {
	if err := os.MkdirAll(s.IssueDir(issue.ID), 0755); err != nil {
		return fmt.Errorf("creating issue dir: %w", err)
	}

	content, err := CreateFrontmatter(issue.ToFrontmatter(), issue.Content)
	if err != nil {
		return err
	}

	return os.WriteFile(s.BriefPath(issue.ID), []byte(content), 0644)
}

// CreateIssue creates a new issue and saves it
func (s *Storage) CreateIssue(title string, issueType model.IssueType, content string) (*model.Issue, error) {
	idx, err := s.LoadIndex()
	if err != nil {
		return nil, err
	}

	issue := &model.Issue{
		ID:      idx.GetNextID(),
		Title:   title,
		Type:    issueType,
		Status:  model.StatusOpen,
		Created: time.Now(),
		Content: content,
	}

	if err := s.SaveBrief(issue); err != nil {
		return nil, err
	}

	idx.AddIssue(issue)
	if err := s.SaveIndex(idx); err != nil {
		return nil, err
	}

	s.gitAdd(s.IndexPath(), s.BriefPath(issue.ID))

	return issue, nil
}

// UpdateIssueStatus updates issue status in both brief.md and index.yaml
func (s *Storage) UpdateIssueStatus(issueID string, status model.IssueStatus, reason string) error {
	// Update brief.md
	issue, err := s.LoadBrief(issueID)
	if err != nil {
		return err
	}
	if issue == nil {
		return fmt.Errorf("issue not found: %s", issueID)
	}

	issue.Status = status
	if reason != "" {
		issue.DiscardReason = reason
	}

	if err := s.SaveBrief(issue); err != nil {
		return err
	}

	// Update index.yaml
	idx, err := s.LoadIndex()
	if err != nil {
		return err
	}

	if idxIssue := idx.GetIssue(issueID); idxIssue != nil {
		idxIssue.Status = status
		idx.UpdateIssue(idxIssue)
		if err := s.SaveIndex(idx); err != nil {
			return err
		}
	}

	// Git add
	s.gitAdd(s.IndexPath(), s.BriefPath(issueID))

	return nil
}

// SyncBriefToIndex syncs title and type from brief.md to index.yaml
func (s *Storage) SyncBriefToIndex(issueID string) error {
	// Load brief.md to get current frontmatter values
	brief, err := s.LoadBrief(issueID)
	if err != nil {
		return err
	}
	if brief == nil {
		return nil // Brief doesn't exist, nothing to sync
	}

	// Load current index
	idx, err := s.LoadIndex()
	if err != nil {
		return err
	}

	// Find and update the issue entry
	if idxIssue := idx.GetIssue(issueID); idxIssue != nil {
		// Only update if values differ
		changed := false
		if idxIssue.Title != brief.Title {
			idxIssue.Title = brief.Title
			changed = true
		}
		if idxIssue.Type != brief.Type {
			idxIssue.Type = brief.Type
			changed = true
		}

		if changed {
			idx.UpdateIssue(idxIssue)
			if err := s.SaveIndex(idx); err != nil {
				return err
			}
			s.gitAdd(s.IndexPath())
		}
	}
	return nil
}

// AnalysisExists checks if analysis.md exists for an issue
func (s *Storage) AnalysisExists(issueID string) bool {
	_, err := os.Stat(s.AnalysisPath(issueID))
	return err == nil
}

// PlanExists checks if plan.md exists for an issue
func (s *Storage) PlanExists(issueID string) bool {
	_, err := os.Stat(s.PlanPath(issueID))
	return err == nil
}

// SaveAnalysis saves analysis.md for an issue
func (s *Storage) SaveAnalysis(issueID, content string) error {
	path := s.AnalysisPath(issueID)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return err
	}
	s.gitAdd(path, s.IndexPath())
	return nil
}

// SavePlan saves plan.md for an issue
func (s *Storage) SavePlan(issueID, content string) error {
	path := s.PlanPath(issueID)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return err
	}
	s.gitAdd(path, s.IndexPath())
	return nil
}

// LoadAnalysis loads analysis.md content for an issue
func (s *Storage) LoadAnalysis(issueID string) (string, error) {
	data, err := os.ReadFile(s.AnalysisPath(issueID))
	if os.IsNotExist(err) {
		return "", nil
	}
	return string(data), err
}

// LoadPlan loads plan.md content for an issue
func (s *Storage) LoadPlan(issueID string) (string, error) {
	data, err := os.ReadFile(s.PlanPath(issueID))
	if os.IsNotExist(err) {
		return "", nil
	}
	return string(data), err
}

// Session management
func (s *Storage) SaveSessionID(issueID, sessionID string) error {
	return os.WriteFile(s.SessionPath(issueID), []byte(sessionID), 0644)
}

func (s *Storage) LoadSessionID(issueID string) (string, error) {
	data, err := os.ReadFile(s.SessionPath(issueID))
	if os.IsNotExist(err) {
		return "", nil
	}
	return string(data), err
}

func (s *Storage) ClearSessionID(issueID string) error {
	err := os.Remove(s.SessionPath(issueID))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Version management
func (s *Storage) GetAnalysisVersion(issueID string) int {
	data, err := os.ReadFile(s.VersionTrackerPath(issueID))
	if err != nil {
		if s.AnalysisExists(issueID) {
			return 1
		}
		return 0
	}
	v, _ := strconv.Atoi(string(data))
	return v
}

func (s *Storage) SaveAnalysisVersioned(issueID, content string, version int) error {
	// Save versioned file
	versionPath := s.AnalysisVersionPath(issueID, version)
	if err := os.WriteFile(versionPath, []byte(content), 0644); err != nil {
		return err
	}

	// Update main analysis.md
	if err := s.SaveAnalysis(issueID, content); err != nil {
		return err
	}

	// Update version tracker
	trackerPath := s.VersionTrackerPath(issueID)
	if err := os.WriteFile(trackerPath, []byte(strconv.Itoa(version)), 0644); err != nil {
		return err
	}

	s.gitAdd(versionPath, s.AnalysisPath(issueID), trackerPath)
	return nil
}

func (s *Storage) LoadAnalysisVersion(issueID string, version int) (string, error) {
	data, err := os.ReadFile(s.AnalysisVersionPath(issueID, version))
	if os.IsNotExist(err) {
		return "", nil
	}
	return string(data), err
}

// Helper to convert map to Issue
func (s *Storage) issueFromMap(m map[string]interface{}) (*model.Issue, error) {
	issue := &model.Issue{}

	issue.ID = GetString(m, "id")
	issue.Title = GetString(m, "title")
	issue.Type = model.IssueType(GetString(m, "type"))
	issue.Status = model.IssueStatus(GetString(m, "status"))

	if dateStr := GetString(m, "created"); dateStr != "" {
		issue.Created, _ = time.Parse("2006-01-02", dateStr)
	}

	return issue, nil
}

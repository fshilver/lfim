package storage

import (
	"os"
	"os/exec"
	"strings"
)

// StageIssueFiles stages all issue files (brief, analysis, plan, index) for git commit.
// Called before implement to stage confirmed files.
func (s *Storage) StageIssueFiles(issueID string) {
	s.gitAdd(
		s.BriefPath(issueID),
		s.AnalysisPath(issueID),
		s.PlanPath(issueID),
		s.IndexPath(),
	)
}

// gitAdd stages files to git. Silently fails if not a git repo.
func (s *Storage) gitAdd(paths ...string) {
	var existing []string
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			existing = append(existing, p)
		}
	}

	if len(existing) == 0 {
		return
	}

	args := append([]string{"add"}, existing...)
	cmd := exec.Command("git", args...)
	cmd.Dir = s.ProjectRoot
	_ = cmd.Run() // Ignore errors
}

// HasStagedChanges checks if there are staged changes to commit
func (s *Storage) HasStagedChanges() bool {
	cmd := exec.Command("git", "diff", "--cached", "--stat")
	cmd.Dir = s.ProjectRoot
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) != ""
}

// GitCommit executes git commit with given message
func (s *Storage) GitCommit(message string) (bool, string) {
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = s.ProjectRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, string(output)
	}
	return true, string(output)
}

// IsGitRepo checks if the project root is a git repository
func (s *Storage) IsGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = s.ProjectRoot
	return cmd.Run() == nil
}

// GitStatus returns the current git status
func (s *Storage) GitStatus() string {
	cmd := exec.Command("git", "status", "--short")
	cmd.Dir = s.ProjectRoot
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(output)
}

// GetGitDiff returns the git diff for the current branch compared to HEAD~1
// This captures changes made during implementation
func (s *Storage) GetGitDiff() string {
	// First try to get diff of staged changes
	cmd := exec.Command("git", "diff", "--cached")
	cmd.Dir = s.ProjectRoot
	output, err := cmd.Output()
	if err == nil && strings.TrimSpace(string(output)) != "" {
		return string(output)
	}

	// If no staged changes, get diff of all changes
	cmd = exec.Command("git", "diff")
	cmd.Dir = s.ProjectRoot
	output, err = cmd.Output()
	if err == nil && strings.TrimSpace(string(output)) != "" {
		return string(output)
	}

	// If still no diff, try to get last commit diff
	cmd = exec.Command("git", "diff", "HEAD~1", "HEAD")
	cmd.Dir = s.ProjectRoot
	output, err = cmd.Output()
	if err != nil {
		return ""
	}
	return string(output)
}

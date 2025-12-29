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
		s.AnalysisJSONPath(issueID),
		s.PlanPath(issueID),
		s.IndexPath(),
	)
}

// gitAdd stages files to git. Silently fails if not a git repo.
// Errors are intentionally ignored to allow the application to work
// in non-git environments without blocking normal operation.
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
	_ = cmd.Run() // Silently ignore errors (git may not be available)
}

// HasStagedChanges checks if there are staged changes to commit.
// If git command fails, returns false (allowing action to proceed).
func (s *Storage) HasStagedChanges() bool {
	cmd := exec.Command("git", "diff", "--cached", "--stat")
	cmd.Dir = s.ProjectRoot
	output, err := cmd.Output()
	if err != nil {
		// If git command fails, allow action (don't block on errors)
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

// runGitDiff executes git diff with given arguments and returns non-empty output
func (s *Storage) runGitDiff(args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = s.ProjectRoot
	output, err := cmd.Output()
	if err == nil && strings.TrimSpace(string(output)) != "" {
		return string(output)
	}
	return ""
}

// GetGitDiff returns the git diff for the current branch compared to HEAD~1
// This captures changes made during implementation
func (s *Storage) GetGitDiff() string {
	// First try to get diff of staged changes
	if diff := s.runGitDiff("diff", "--cached"); diff != "" {
		return diff
	}

	// If no staged changes, get diff of all changes
	if diff := s.runGitDiff("diff"); diff != "" {
		return diff
	}

	// If still no diff, try to get last commit diff
	return s.runGitDiff("diff", "HEAD~1", "HEAD")
}

// HasUncommittedChanges checks if there are any uncommitted changes in the repository.
// Returns true if there are staged, unstaged, or untracked files.
// If git command fails, returns false (allowing action to proceed).
func (s *Storage) HasUncommittedChanges() bool {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = s.ProjectRoot
	output, err := cmd.Output()
	if err != nil {
		// If git command fails, allow action (don't block on errors)
		return false
	}
	// --porcelain returns empty output if working directory is clean
	return strings.TrimSpace(string(output)) != ""
}

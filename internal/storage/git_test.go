package storage

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestHasUncommittedChanges(t *testing.T) {
	// Create a temporary directory for test git repo
	tmpDir, err := os.MkdirTemp("", "lfim-git-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Skipf("Git not available or failed to init: %v", err)
	}

	// Configure git for test commits
	exec.Command("git", "config", "user.email", "test@example.com").Dir = tmpDir
	exec.Command("git", "config", "user.name", "Test User").Dir = tmpDir

	storage := &Storage{ProjectRoot: tmpDir}

	t.Run("clean repository", func(t *testing.T) {
		// Initially, the repo should be empty
		// Git status --porcelain returns empty for clean working directory
		// but a newly initialized repo without any commits is technically "clean"
		// Let's create an initial commit first
		testFile := filepath.Join(tmpDir, "test.txt")
		if err := os.WriteFile(testFile, []byte("initial"), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}
		
		cmd := exec.Command("git", "add", "test.txt")
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to add file: %v", err)
		}
		
		cmd = exec.Command("git", "commit", "-m", "Initial commit")
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to commit: %v", err)
		}

		// Now the repo should be clean
		if storage.HasUncommittedChanges() {
			t.Error("Expected no uncommitted changes in clean repo")
		}
	})

	t.Run("unstaged changes", func(t *testing.T) {
		// Modify a file
		testFile := filepath.Join(tmpDir, "test.txt")
		if err := os.WriteFile(testFile, []byte("modified"), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		if !storage.HasUncommittedChanges() {
			t.Error("Expected uncommitted changes with unstaged file")
		}

		// Clean up
		cmd := exec.Command("git", "restore", "test.txt")
		cmd.Dir = tmpDir
		cmd.Run()
	})

	t.Run("staged changes", func(t *testing.T) {
		// Modify and stage a file
		testFile := filepath.Join(tmpDir, "test.txt")
		if err := os.WriteFile(testFile, []byte("staged"), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		cmd := exec.Command("git", "add", "test.txt")
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to stage file: %v", err)
		}

		if !storage.HasUncommittedChanges() {
			t.Error("Expected uncommitted changes with staged file")
		}

		// Clean up
		cmd = exec.Command("git", "restore", "--staged", "test.txt")
		cmd.Dir = tmpDir
		cmd.Run()
		cmd = exec.Command("git", "restore", "test.txt")
		cmd.Dir = tmpDir
		cmd.Run()
	})

	t.Run("untracked files", func(t *testing.T) {
		// Create a new untracked file
		newFile := filepath.Join(tmpDir, "untracked.txt")
		if err := os.WriteFile(newFile, []byte("untracked"), 0644); err != nil {
			t.Fatalf("Failed to write untracked file: %v", err)
		}

		if !storage.HasUncommittedChanges() {
			t.Error("Expected uncommitted changes with untracked file")
		}

		// Clean up
		os.Remove(newFile)
	})

	t.Run("non-git directory", func(t *testing.T) {
		// Create a non-git directory
		nonGitDir, err := os.MkdirTemp("", "lfim-nongit-test-*")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(nonGitDir)

		storageNonGit := &Storage{ProjectRoot: nonGitDir}

		// Should return false (allow action) for non-git repos
		if storageNonGit.HasUncommittedChanges() {
			t.Error("Expected false for non-git directory (graceful degradation)")
		}
	})
}

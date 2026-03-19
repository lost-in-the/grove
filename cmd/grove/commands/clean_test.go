package commands

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCleanCandidateDaysUnknown(t *testing.T) {
	c := CleanCandidate{DaysUnknown: true}
	if c.DaysSince != 0 {
		t.Errorf("DaysSince should be zero-value when DaysUnknown is set, got %d", c.DaysSince)
	}
	if !c.DaysUnknown {
		t.Error("DaysUnknown should be true")
	}
}

func TestWorktreeLastModified_GitFile(t *testing.T) {
	// Create a temporary directory simulating a linked worktree
	dir := t.TempDir()

	// Write a fake .git file (linked worktrees have a plain .git file, not a directory)
	gitFile := filepath.Join(dir, ".git")
	if err := os.WriteFile(gitFile, []byte("gitdir: ../.git/worktrees/test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	before := time.Now().Add(-time.Second)
	result, err := worktreeLastModified(dir, time.Now())
	after := time.Now().Add(time.Second)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.Before(before) || result.After(after) {
		t.Errorf("mtime %v not in expected range [%v, %v]", result, before, after)
	}
}

func TestWorktreeLastModified_NoGitFile(t *testing.T) {
	// A directory with no .git file and no git history — should return an error
	dir := t.TempDir()

	_, err := worktreeLastModified(dir, time.Now())
	if err == nil {
		t.Error("expected error for directory without .git file or git history")
	}
}

func TestWorktreeLastModified_GitLog(t *testing.T) {
	// Use the grove repo itself (has git history) — git log should succeed
	dir := findRepoRoot(t)
	if dir == "" {
		t.Skip("could not find git repo root")
	}

	result, err := worktreeLastModified(dir, time.Now())
	if err != nil {
		t.Fatalf("expected no error for repo with git history, got: %v", err)
	}
	if result.IsZero() {
		t.Error("expected non-zero time from git log")
	}
	// Sanity check: commit time should be in the past
	if result.After(time.Now()) {
		t.Errorf("git log timestamp %v is in the future", result)
	}
}

// findRepoRoot walks up from the current working directory to find a .git directory.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if info, err := os.Stat(filepath.Join(dir, ".git")); err == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

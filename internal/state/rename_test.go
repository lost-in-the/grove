package state

import (
	"strings"
	"testing"
	"time"
)

func TestRenameWorktree(t *testing.T) {
	t.Run("rename moves entry preserving all fields", func(t *testing.T) {
		mgr := setupTestManager(t)

		now := time.Now()
		ws := &WorktreeState{
			Path:           "/path/to/old",
			Branch:         "feature-auth",
			Root:           false,
			DockerProject:  "my-docker",
			CreatedAt:      now,
			LastAccessedAt: now,
			Environment:    true,
			Mirror:         "origin/main",
		}
		if err := mgr.AddWorktree("old-name", ws); err != nil {
			t.Fatalf("AddWorktree() error = %v", err)
		}

		if err := mgr.RenameWorktree("old-name", "new-name"); err != nil {
			t.Fatalf("RenameWorktree() error = %v", err)
		}

		// Old name should not exist
		got, err := mgr.GetWorktree("old-name")
		if err != nil {
			t.Fatalf("GetWorktree(old) error = %v", err)
		}
		if got != nil {
			t.Error("old-name should no longer exist")
		}

		// New name should exist with all fields preserved
		got, err = mgr.GetWorktree("new-name")
		if err != nil {
			t.Fatalf("GetWorktree(new) error = %v", err)
		}
		if got == nil {
			t.Fatal("new-name should exist")
		}
		if got.Path != ws.Path {
			t.Errorf("Path = %q, want %q", got.Path, ws.Path)
		}
		if got.Branch != ws.Branch {
			t.Errorf("Branch = %q, want %q", got.Branch, ws.Branch)
		}
		if got.DockerProject != ws.DockerProject {
			t.Errorf("DockerProject = %q, want %q", got.DockerProject, ws.DockerProject)
		}
		if !got.Environment {
			t.Error("Environment should be true")
		}
		if got.Mirror != ws.Mirror {
			t.Errorf("Mirror = %q, want %q", got.Mirror, ws.Mirror)
		}
	})

	t.Run("rename updates last_worktree if it was the old name", func(t *testing.T) {
		mgr := setupTestManager(t)

		ws := &WorktreeState{Path: "/path", Branch: "main"}
		if err := mgr.AddWorktree("current", ws); err != nil {
			t.Fatalf("AddWorktree() error = %v", err)
		}
		if err := mgr.SetLastWorktree("current"); err != nil {
			t.Fatalf("SetLastWorktree() error = %v", err)
		}

		if err := mgr.RenameWorktree("current", "renamed"); err != nil {
			t.Fatalf("RenameWorktree() error = %v", err)
		}

		last, err := mgr.GetLastWorktree()
		if err != nil {
			t.Fatalf("GetLastWorktree() error = %v", err)
		}
		if last != "renamed" {
			t.Errorf("GetLastWorktree() = %q, want %q", last, "renamed")
		}
	})

	t.Run("rename nonexistent name returns error", func(t *testing.T) {
		mgr := setupTestManager(t)

		err := mgr.RenameWorktree("nonexistent", "new-name")
		if err == nil {
			t.Error("RenameWorktree() should return error for nonexistent name")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("error should contain 'not found', got: %v", err)
		}
	})

	t.Run("rename to existing name returns error", func(t *testing.T) {
		mgr := setupTestManager(t)

		_ = mgr.AddWorktree("first", &WorktreeState{Path: "/p1", Branch: "b1"})
		_ = mgr.AddWorktree("second", &WorktreeState{Path: "/p2", Branch: "b2"})

		err := mgr.RenameWorktree("first", "second")
		if err == nil {
			t.Error("RenameWorktree() should return error when target name exists")
		}
		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("error should contain 'already exists', got: %v", err)
		}
	})

	t.Run("rename with empty old name returns error", func(t *testing.T) {
		mgr := setupTestManager(t)
		err := mgr.RenameWorktree("", "new")
		if err == nil {
			t.Error("RenameWorktree(\"\", ...) should return error")
		}
	})

	t.Run("rename with empty new name returns error", func(t *testing.T) {
		mgr := setupTestManager(t)
		_ = mgr.AddWorktree("old", &WorktreeState{Path: "/p", Branch: "b"})
		err := mgr.RenameWorktree("old", "")
		if err == nil {
			t.Error("RenameWorktree(..., \"\") should return error")
		}
	})

	t.Run("rename persists across manager instances", func(t *testing.T) {
		stateDir := t.TempDir()

		mgr1, err := NewManager(stateDir)
		if err != nil {
			t.Fatal(err)
		}

		_ = mgr1.AddWorktree("before", &WorktreeState{Path: "/p", Branch: "b"})

		if err := mgr1.RenameWorktree("before", "after"); err != nil {
			t.Fatalf("RenameWorktree() error = %v", err)
		}

		// Load with a new manager
		mgr2, err := NewManager(stateDir)
		if err != nil {
			t.Fatal(err)
		}

		got, _ := mgr2.GetWorktree("before")
		if got != nil {
			t.Error("old name should not exist after reload")
		}

		got, _ = mgr2.GetWorktree("after")
		if got == nil {
			t.Error("new name should exist after reload")
		}
	})
}

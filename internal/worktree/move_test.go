package worktree

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMove(t *testing.T) {
	repoDir, cleanup := setupTestRepo(t)
	defer cleanup()

	mgr, err := NewManager(repoDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	t.Run("move renames worktree directory", func(t *testing.T) {
		// Create a worktree first
		if err := mgr.Create("move-test", "move-test"); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		oldFullName := mgr.FullName("move-test")
		newFullName := mgr.FullName("moved")
		oldPath := filepath.Join(filepath.Dir(repoDir), oldFullName)
		newPath := filepath.Join(filepath.Dir(repoDir), newFullName)

		// Verify old path exists
		if _, err := os.Stat(oldPath); err != nil {
			t.Fatalf("old worktree path should exist: %v", err)
		}

		// Move it
		if err := mgr.Move("move-test", "moved"); err != nil {
			t.Fatalf("Move() error = %v", err)
		}

		// Old path should be gone
		if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
			t.Error("old worktree path should not exist after move")
		}

		// New path should exist
		if _, err := os.Stat(newPath); err != nil {
			t.Errorf("new worktree path should exist: %v", err)
		}

		// New worktree should be findable
		wt, err := mgr.Find("moved")
		if err != nil {
			t.Fatalf("Find() error = %v", err)
		}
		if wt == nil {
			t.Error("moved worktree should be findable")
		}
	})

	t.Run("move with empty old name returns error", func(t *testing.T) {
		err := mgr.Move("", "new")
		if err == nil {
			t.Error("Move(\"\", ...) should return error")
		}
	})

	t.Run("move with empty new name returns error", func(t *testing.T) {
		err := mgr.Move("old", "")
		if err == nil {
			t.Error("Move(..., \"\") should return error")
		}
	})
}

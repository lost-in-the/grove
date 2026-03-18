package worktree

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/lost-in-the/grove/internal/cmdexec"
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

	t.Run("move works for worktree in non-standard location", func(t *testing.T) {
		// Simulate a worktree created outside the standard sibling directory
		// (e.g. by Claude Code's EnterWorktree in .claude/worktrees/)
		subDir := filepath.Join(repoDir, ".claude", "worktrees")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}

		wtPath := filepath.Join(subDir, "agent-test")
		output, err := cmdexec.CombinedOutput(context.TODO(), "git",
			[]string{"worktree", "add", "-b", "worktree-agent-test", wtPath},
			repoDir, cmdexec.GitLocal)
		if err != nil {
			t.Fatalf("git worktree add error = %v: %s", err, output)
		}

		if err := mgr.Move("agent-test", "agent-renamed"); err != nil {
			t.Fatalf("Move() error = %v", err)
		}

		newPath := filepath.Join(subDir, "agent-renamed")
		if _, err := os.Stat(newPath); err != nil {
			t.Errorf("renamed worktree should exist at %s: %v", newPath, err)
		}

		// Old path should be gone
		if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
			t.Error("old worktree path should not exist after move")
		}

		wt, err := mgr.Find("agent-renamed")
		if err != nil {
			t.Fatalf("Find() error = %v", err)
		}
		if wt == nil {
			t.Error("renamed worktree should be findable")
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

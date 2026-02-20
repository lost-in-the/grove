package state

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestNewManager(t *testing.T) {
	tests := []struct {
		name      string
		groveDir  string
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "valid grove directory",
			groveDir: filepath.Join(t.TempDir(), ".grove"),
			wantErr:  false,
		},
		{
			name:      "empty grove directory returns error",
			groveDir:  "",
			wantErr:   true,
			errSubstr: "grove directory path is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr, err := NewManager(tt.groveDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewManager() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errSubstr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("NewManager() error = %v, want error containing %q", err, tt.errSubstr)
				}
				return
			}
			if !tt.wantErr && mgr == nil {
				t.Error("NewManager() returned nil manager without error")
			}
			if !tt.wantErr && mgr.groveDir == "" {
				t.Error("NewManager() returned manager with empty groveDir")
			}
		})
	}
}

func TestManagerPersistence(t *testing.T) {
	t.Run("state persists across manager instances", func(t *testing.T) {
		stateDir := t.TempDir()

		// Create first manager and add worktrees
		mgr1, err := NewManager(stateDir)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}

		if err := mgr1.AddWorktree("feature-auth", &WorktreeState{Path: "/path/1", Branch: "main"}); err != nil {
			t.Fatalf("AddWorktree() error = %v", err)
		}
		if err := mgr1.AddWorktree("bugfix-123", &WorktreeState{Path: "/path/2", Branch: "dev"}); err != nil {
			t.Fatalf("AddWorktree() error = %v", err)
		}

		// Create second manager with same state dir
		mgr2, err := NewManager(stateDir)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}

		// Verify state is loaded
		worktrees := mgr2.ListWorktrees()
		if len(worktrees) != 2 {
			t.Errorf("ListWorktrees() returned %d worktrees, want 2", len(worktrees))
		}

		// Verify individual worktree
		ws, err := mgr2.GetWorktree("feature-auth")
		if err != nil {
			t.Fatalf("GetWorktree() error = %v", err)
		}
		if ws == nil {
			t.Error("feature-auth should exist")
		}
	})

	t.Run("state file format is valid JSON", func(t *testing.T) {
		stateDir := t.TempDir()
		mgr, err := NewManager(stateDir)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}

		if err := mgr.AddWorktree("feature-auth", &WorktreeState{Path: "/path", Branch: "main"}); err != nil {
			t.Fatalf("AddWorktree() error = %v", err)
		}

		// Verify state file exists and is valid JSON
		stateFile := filepath.Join(stateDir, "state.json")
		if _, err := os.Stat(stateFile); err != nil {
			t.Errorf("state file does not exist: %v", err)
		}

		// Try to read it with a new manager (validates JSON)
		mgr2, err := NewManager(stateDir)
		if err != nil {
			t.Fatalf("NewManager() failed to load state: %v", err)
		}

		worktrees := mgr2.ListWorktrees()
		if len(worktrees) != 1 {
			t.Errorf("loaded state has %d worktrees, want 1", len(worktrees))
		}
	})
}

func TestConcurrentOperations(t *testing.T) {
	t.Run("concurrent worktree operations", func(t *testing.T) {
		mgr := setupTestManager(t)

		// Launch concurrent AddWorktree operations
		done := make(chan error, 3)
		go func() { done <- mgr.AddWorktree("worktree-1", &WorktreeState{Path: "/p1", Branch: "b1"}) }()
		go func() { done <- mgr.AddWorktree("worktree-2", &WorktreeState{Path: "/p2", Branch: "b2"}) }()
		go func() { done <- mgr.AddWorktree("worktree-3", &WorktreeState{Path: "/p3", Branch: "b3"}) }()

		// Wait for all to complete
		for i := 0; i < 3; i++ {
			if err := <-done; err != nil {
				t.Errorf("concurrent AddWorktree() error = %v", err)
			}
		}

		// Verify all worktrees exist
		worktrees := mgr.ListWorktrees()
		if len(worktrees) != 3 {
			t.Errorf("ListWorktrees() = %d worktrees, want 3", len(worktrees))
		}
	})
}

// --- V2 State Method Tests ---

func TestManagerProject(t *testing.T) {
	t.Run("set and get project name", func(t *testing.T) {
		mgr := setupTestManager(t)

		// Initially empty
		if name := mgr.GetProject(); name != "" {
			t.Errorf("GetProject() = %q, want empty", name)
		}

		// Set project
		if err := mgr.SetProject("grove-cli"); err != nil {
			t.Fatalf("SetProject() error = %v", err)
		}

		// Get project
		if name := mgr.GetProject(); name != "grove-cli" {
			t.Errorf("GetProject() = %q, want %q", name, "grove-cli")
		}
	})

	t.Run("empty project name returns error", func(t *testing.T) {
		mgr := setupTestManager(t)
		err := mgr.SetProject("")
		if err == nil {
			t.Error("SetProject(\"\") should return error")
		}
	})
}

func TestManagerLastWorktree(t *testing.T) {
	t.Run("set and get last worktree", func(t *testing.T) {
		mgr := setupTestManager(t)

		// Initially empty
		last, err := mgr.GetLastWorktree()
		if err != nil {
			t.Fatalf("GetLastWorktree() error = %v", err)
		}
		if last != "" {
			t.Errorf("GetLastWorktree() = %q, want empty", last)
		}

		// Set last worktree
		if err := mgr.SetLastWorktree("testing"); err != nil {
			t.Fatalf("SetLastWorktree() error = %v", err)
		}

		// Get last worktree
		last, err = mgr.GetLastWorktree()
		if err != nil {
			t.Fatalf("GetLastWorktree() error = %v", err)
		}
		if last != "testing" {
			t.Errorf("GetLastWorktree() = %q, want %q", last, "testing")
		}
	})

	t.Run("empty worktree name returns error", func(t *testing.T) {
		mgr := setupTestManager(t)
		err := mgr.SetLastWorktree("")
		if err == nil {
			t.Error("SetLastWorktree(\"\") should return error")
		}
	})
}

func TestManagerWorktreeOperations(t *testing.T) {
	t.Run("add and get worktree", func(t *testing.T) {
		mgr := setupTestManager(t)

		ws := &WorktreeState{
			Path:   "/path/to/worktree",
			Branch: "feature-auth",
			Root:   false,
		}

		if err := mgr.AddWorktree("testing", ws); err != nil {
			t.Fatalf("AddWorktree() error = %v", err)
		}

		got, err := mgr.GetWorktree("testing")
		if err != nil {
			t.Fatalf("GetWorktree() error = %v", err)
		}
		if got == nil {
			t.Fatal("GetWorktree() returned nil")
		}
		if got.Path != ws.Path {
			t.Errorf("GetWorktree().Path = %q, want %q", got.Path, ws.Path)
		}
		if got.Branch != ws.Branch {
			t.Errorf("GetWorktree().Branch = %q, want %q", got.Branch, ws.Branch)
		}
	})

	t.Run("get non-existent worktree returns nil", func(t *testing.T) {
		mgr := setupTestManager(t)
		got, err := mgr.GetWorktree("nonexistent")
		if err != nil {
			t.Fatalf("GetWorktree() error = %v", err)
		}
		if got != nil {
			t.Errorf("GetWorktree() = %v, want nil", got)
		}
	})

	t.Run("remove worktree", func(t *testing.T) {
		mgr := setupTestManager(t)

		ws := &WorktreeState{Path: "/path", Branch: "main"}
		if err := mgr.AddWorktree("testing", ws); err != nil {
			t.Fatalf("AddWorktree() error = %v", err)
		}

		if err := mgr.RemoveWorktree("testing"); err != nil {
			t.Fatalf("RemoveWorktree() error = %v", err)
		}

		got, _ := mgr.GetWorktree("testing")
		if got != nil {
			t.Error("worktree should be removed")
		}
	})

	t.Run("list worktrees returns sorted names", func(t *testing.T) {
		mgr := setupTestManager(t)

		_ = mgr.AddWorktree("zebra", &WorktreeState{Path: "/z"})
		_ = mgr.AddWorktree("alpha", &WorktreeState{Path: "/a"})
		_ = mgr.AddWorktree("middle", &WorktreeState{Path: "/m"})

		names := mgr.ListWorktrees()
		expected := []string{"alpha", "middle", "zebra"}
		if !slices.Equal(names, expected) {
			t.Errorf("ListWorktrees() = %v, want %v", names, expected)
		}
	})
}

func TestManagerTouchWorktree(t *testing.T) {
	t.Run("touch updates last_accessed_at", func(t *testing.T) {
		mgr := setupTestManager(t)

		ws := &WorktreeState{Path: "/path", Branch: "main"}
		if err := mgr.AddWorktree("testing", ws); err != nil {
			t.Fatalf("AddWorktree() error = %v", err)
		}

		if err := mgr.TouchWorktree("testing"); err != nil {
			t.Fatalf("TouchWorktree() error = %v", err)
		}

		got, _ := mgr.GetWorktree("testing")
		if got.LastAccessedAt.IsZero() {
			t.Error("LastAccessedAt should be set after touch")
		}
	})

	t.Run("touch non-existent worktree returns error", func(t *testing.T) {
		mgr := setupTestManager(t)
		err := mgr.TouchWorktree("nonexistent")
		if err == nil {
			t.Error("TouchWorktree() on non-existent should return error")
		}
	})
}

func TestManagerIsEnvironment(t *testing.T) {
	t.Run("returns true for environment worktree", func(t *testing.T) {
		mgr := setupTestManager(t)

		ws := &WorktreeState{
			Path:        "/path",
			Branch:      "main",
			Environment: true,
			Mirror:      "origin/main",
		}
		if err := mgr.AddWorktree("production", ws); err != nil {
			t.Fatalf("AddWorktree() error = %v", err)
		}

		isEnv, err := mgr.IsEnvironment("production")
		if err != nil {
			t.Fatalf("IsEnvironment() error = %v", err)
		}
		if !isEnv {
			t.Error("IsEnvironment() = false, want true")
		}
	})

	t.Run("returns false for regular worktree", func(t *testing.T) {
		mgr := setupTestManager(t)

		ws := &WorktreeState{Path: "/path", Branch: "feature"}
		if err := mgr.AddWorktree("testing", ws); err != nil {
			t.Fatalf("AddWorktree() error = %v", err)
		}

		isEnv, err := mgr.IsEnvironment("testing")
		if err != nil {
			t.Fatalf("IsEnvironment() error = %v", err)
		}
		if isEnv {
			t.Error("IsEnvironment() = true, want false")
		}
	})

	t.Run("returns false for non-existent worktree", func(t *testing.T) {
		mgr := setupTestManager(t)
		isEnv, err := mgr.IsEnvironment("nonexistent")
		if err != nil {
			t.Fatalf("IsEnvironment() error = %v", err)
		}
		if isEnv {
			t.Error("IsEnvironment() = true, want false for non-existent")
		}
	})
}

func TestManagerGetState(t *testing.T) {
	t.Run("returns copy of state", func(t *testing.T) {
		mgr := setupTestManager(t)

		if err := mgr.SetProject("test-project"); err != nil {
			t.Fatalf("SetProject() error = %v", err)
		}
		if err := mgr.AddWorktree("testing", &WorktreeState{Path: "/path"}); err != nil {
			t.Fatalf("AddWorktree() error = %v", err)
		}

		state := mgr.GetState()
		if state.Project != "test-project" {
			t.Errorf("GetState().Project = %q, want %q", state.Project, "test-project")
		}
		if len(state.Worktrees) != 1 {
			t.Errorf("GetState().Worktrees has %d entries, want 1", len(state.Worktrees))
		}
	})
}

func TestV2StatePersistence(t *testing.T) {
	t.Run("V2 state persists across manager instances", func(t *testing.T) {
		stateDir := t.TempDir()

		// Create first manager and add V2 state
		mgr1, err := NewManager(stateDir)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}

		if err := mgr1.SetProject("my-project"); err != nil {
			t.Fatalf("SetProject() error = %v", err)
		}
		if err := mgr1.SetLastWorktree("testing"); err != nil {
			t.Fatalf("SetLastWorktree() error = %v", err)
		}
		ws := &WorktreeState{
			Path:        "/path/to/testing",
			Branch:      "feature-auth",
			Environment: true,
			Mirror:      "origin/main",
		}
		if err := mgr1.AddWorktree("testing", ws); err != nil {
			t.Fatalf("AddWorktree() error = %v", err)
		}

		// Create second manager and verify persistence
		mgr2, err := NewManager(stateDir)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}

		if project := mgr2.GetProject(); project != "my-project" {
			t.Errorf("GetProject() = %q, want %q", project, "my-project")
		}

		last, _ := mgr2.GetLastWorktree()
		if last != "testing" {
			t.Errorf("GetLastWorktree() = %q, want %q", last, "testing")
		}

		got, _ := mgr2.GetWorktree("testing")
		if got == nil {
			t.Fatal("GetWorktree() returned nil")
		}
		if got.Branch != "feature-auth" {
			t.Errorf("GetWorktree().Branch = %q, want %q", got.Branch, "feature-auth")
		}
		if !got.Environment {
			t.Error("GetWorktree().Environment should be true")
		}
		if got.Mirror != "origin/main" {
			t.Errorf("GetWorktree().Mirror = %q, want %q", got.Mirror, "origin/main")
		}
	})
}

// Helper functions

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	stateDir := t.TempDir()
	mgr, err := NewManager(stateDir)
	if err != nil {
		t.Fatalf("failed to create test manager: %v", err)
	}
	return mgr
}

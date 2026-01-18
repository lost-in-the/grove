package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewManager(t *testing.T) {
	tests := []struct {
		name      string
		stateDir  string
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "valid state directory",
			stateDir: filepath.Join(t.TempDir(), "state"),
			wantErr:  false,
		},
		{
			name:     "empty state directory uses default",
			stateDir: "",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr, err := NewManager(tt.stateDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewManager() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && mgr == nil {
				t.Error("NewManager() returned nil manager without error")
			}
			if !tt.wantErr && mgr.stateDir == "" {
				t.Error("NewManager() returned manager with empty stateDir")
			}
		})
	}
}

func TestManagerFreeze(t *testing.T) {
	tests := []struct {
		name      string
		worktree  string
		setup     func(*Manager) error
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "freeze new worktree",
			worktree: "feature-auth",
			wantErr:  false,
		},
		{
			name:     "freeze already frozen worktree is idempotent",
			worktree: "feature-auth",
			setup: func(m *Manager) error {
				return m.Freeze("feature-auth")
			},
			wantErr: false,
		},
		{
			name:      "empty worktree name",
			worktree:  "",
			wantErr:   true,
			errSubstr: "worktree name cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := setupTestManager(t)

			if tt.setup != nil {
				if err := tt.setup(mgr); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
			}

			err := mgr.Freeze(tt.worktree)
			if (err != nil) != tt.wantErr {
				t.Errorf("Freeze() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errSubstr != "" {
				if err == nil || !contains(err.Error(), tt.errSubstr) {
					t.Errorf("Freeze() error = %v, want error containing %q", err, tt.errSubstr)
				}
			}
			if !tt.wantErr {
				// Verify it's frozen
				frozen, err := mgr.IsFrozen(tt.worktree)
				if err != nil {
					t.Errorf("IsFrozen() error = %v", err)
				}
				if !frozen {
					t.Errorf("Freeze() succeeded but worktree is not frozen")
				}
			}
		})
	}
}

func TestManagerResume(t *testing.T) {
	tests := []struct {
		name      string
		worktree  string
		setup     func(*Manager) error
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "resume frozen worktree",
			worktree: "feature-auth",
			setup: func(m *Manager) error {
				return m.Freeze("feature-auth")
			},
			wantErr: false,
		},
		{
			name:     "resume non-frozen worktree is idempotent",
			worktree: "feature-auth",
			wantErr:  false,
		},
		{
			name:      "empty worktree name",
			worktree:  "",
			wantErr:   true,
			errSubstr: "worktree name cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := setupTestManager(t)

			if tt.setup != nil {
				if err := tt.setup(mgr); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
			}

			err := mgr.Resume(tt.worktree)
			if (err != nil) != tt.wantErr {
				t.Errorf("Resume() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errSubstr != "" {
				if err == nil || !contains(err.Error(), tt.errSubstr) {
					t.Errorf("Resume() error = %v, want error containing %q", err, tt.errSubstr)
				}
			}
			if !tt.wantErr {
				// Verify it's not frozen
				frozen, err := mgr.IsFrozen(tt.worktree)
				if err != nil {
					t.Errorf("IsFrozen() error = %v", err)
				}
				if frozen {
					t.Errorf("Resume() succeeded but worktree is still frozen")
				}
			}
		})
	}
}

func TestManagerIsFrozen(t *testing.T) {
	tests := []struct {
		name       string
		worktree   string
		setup      func(*Manager) error
		wantFrozen bool
		wantErr    bool
		errSubstr  string
	}{
		{
			name:       "frozen worktree returns true",
			worktree:   "feature-auth",
			setup:      func(m *Manager) error { return m.Freeze("feature-auth") },
			wantFrozen: true,
			wantErr:    false,
		},
		{
			name:       "non-frozen worktree returns false",
			worktree:   "feature-auth",
			wantFrozen: false,
			wantErr:    false,
		},
		{
			name:      "empty worktree name",
			worktree:  "",
			wantErr:   true,
			errSubstr: "worktree name cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := setupTestManager(t)

			if tt.setup != nil {
				if err := tt.setup(mgr); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
			}

			frozen, err := mgr.IsFrozen(tt.worktree)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsFrozen() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errSubstr != "" {
				if err == nil || !contains(err.Error(), tt.errSubstr) {
					t.Errorf("IsFrozen() error = %v, want error containing %q", err, tt.errSubstr)
				}
			}
			if !tt.wantErr && frozen != tt.wantFrozen {
				t.Errorf("IsFrozen() = %v, want %v", frozen, tt.wantFrozen)
			}
		})
	}
}

func TestManagerListFrozen(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*Manager) error
		wantCount int
		wantNames []string
		wantErr   bool
	}{
		{
			name:      "no frozen worktrees",
			wantCount: 0,
			wantNames: []string{},
			wantErr:   false,
		},
		{
			name: "single frozen worktree",
			setup: func(m *Manager) error {
				return m.Freeze("feature-auth")
			},
			wantCount: 1,
			wantNames: []string{"feature-auth"},
			wantErr:   false,
		},
		{
			name: "multiple frozen worktrees",
			setup: func(m *Manager) error {
				if err := m.Freeze("feature-auth"); err != nil {
					return err
				}
				if err := m.Freeze("bugfix-123"); err != nil {
					return err
				}
				return m.Freeze("refactor-db")
			},
			wantCount: 3,
			wantNames: []string{"bugfix-123", "feature-auth", "refactor-db"}, // sorted
			wantErr:   false,
		},
		{
			name: "frozen and resumed worktrees",
			setup: func(m *Manager) error {
				if err := m.Freeze("feature-auth"); err != nil {
					return err
				}
				if err := m.Freeze("bugfix-123"); err != nil {
					return err
				}
				// Resume one
				return m.Resume("feature-auth")
			},
			wantCount: 1,
			wantNames: []string{"bugfix-123"},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := setupTestManager(t)

			if tt.setup != nil {
				if err := tt.setup(mgr); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
			}

			frozen, err := mgr.ListFrozen()
			if (err != nil) != tt.wantErr {
				t.Errorf("ListFrozen() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(frozen) != tt.wantCount {
				t.Errorf("ListFrozen() returned %d worktrees, want %d", len(frozen), tt.wantCount)
			}
			if !stringSliceEqual(frozen, tt.wantNames) {
				t.Errorf("ListFrozen() = %v, want %v", frozen, tt.wantNames)
			}
		})
	}
}

func TestManagerPersistence(t *testing.T) {
	t.Run("state persists across manager instances", func(t *testing.T) {
		stateDir := t.TempDir()

		// Create first manager and freeze worktrees
		mgr1, err := NewManager(stateDir)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}

		if err := mgr1.Freeze("feature-auth"); err != nil {
			t.Fatalf("Freeze() error = %v", err)
		}
		if err := mgr1.Freeze("bugfix-123"); err != nil {
			t.Fatalf("Freeze() error = %v", err)
		}

		// Create second manager with same state dir
		mgr2, err := NewManager(stateDir)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}

		// Verify state is loaded
		frozen, err := mgr2.ListFrozen()
		if err != nil {
			t.Fatalf("ListFrozen() error = %v", err)
		}
		if len(frozen) != 2 {
			t.Errorf("ListFrozen() returned %d worktrees, want 2", len(frozen))
		}

		// Verify individual checks
		isFrozen, err := mgr2.IsFrozen("feature-auth")
		if err != nil {
			t.Fatalf("IsFrozen() error = %v", err)
		}
		if !isFrozen {
			t.Error("feature-auth should be frozen")
		}
	})

	t.Run("state file format is valid JSON", func(t *testing.T) {
		stateDir := t.TempDir()
		mgr, err := NewManager(stateDir)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}

		if err := mgr.Freeze("feature-auth"); err != nil {
			t.Fatalf("Freeze() error = %v", err)
		}

		// Verify state file exists and is valid JSON
		stateFile := filepath.Join(stateDir, "frozen.json")
		if _, err := os.Stat(stateFile); err != nil {
			t.Errorf("state file does not exist: %v", err)
		}

		// Try to read it with a new manager (validates JSON)
		mgr2, err := NewManager(stateDir)
		if err != nil {
			t.Fatalf("NewManager() failed to load state: %v", err)
		}

		frozen, err := mgr2.ListFrozen()
		if err != nil {
			t.Fatalf("ListFrozen() error = %v", err)
		}
		if len(frozen) != 1 {
			t.Errorf("loaded state has %d worktrees, want 1", len(frozen))
		}
	})
}

func TestConcurrentOperations(t *testing.T) {
	t.Run("concurrent freeze operations", func(t *testing.T) {
		mgr := setupTestManager(t)

		// Launch concurrent freeze operations
		done := make(chan error, 3)
		go func() { done <- mgr.Freeze("worktree-1") }()
		go func() { done <- mgr.Freeze("worktree-2") }()
		go func() { done <- mgr.Freeze("worktree-3") }()

		// Wait for all to complete
		for i := 0; i < 3; i++ {
			if err := <-done; err != nil {
				t.Errorf("concurrent Freeze() error = %v", err)
			}
		}

		// Verify all are frozen
		frozen, err := mgr.ListFrozen()
		if err != nil {
			t.Fatalf("ListFrozen() error = %v", err)
		}
		if len(frozen) != 3 {
			t.Errorf("ListFrozen() = %d worktrees, want 3", len(frozen))
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

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

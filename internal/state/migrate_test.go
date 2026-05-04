package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMigrateFromLegacy(t *testing.T) {
	t.Run("migrates legacy frozen.json to V2 state", func(t *testing.T) {
		groveDir := t.TempDir()
		legacyDir := t.TempDir()
		legacyPath := filepath.Join(legacyDir, "frozen.json")

		// Create legacy state (frozen data is read but not preserved in V2)
		legacy := LegacyState{
			Frozen: map[string]any{
				"feature-auth": map[string]string{"name": "feature-auth"},
			},
		}
		data, _ := json.Marshal(legacy)
		if err := os.WriteFile(legacyPath, data, 0644); err != nil {
			t.Fatalf("failed to write legacy state: %v", err)
		}

		// Migrate
		migrated, err := MigrateFromLegacy(groveDir, legacyPath)
		if err != nil {
			t.Fatalf("MigrateFromLegacy() error = %v", err)
		}
		if !migrated {
			t.Error("MigrateFromLegacy() returned false, want true")
		}

		// Verify new state was created
		stateFile := filepath.Join(groveDir, "state.json")
		if _, err := os.Stat(stateFile); os.IsNotExist(err) {
			t.Fatal("state.json was not created")
		}

		// Load with manager and verify structure
		mgr, err := NewManager(groveDir)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}

		// Verify state has correct version
		state := mgr.GetState()
		if state.Version != CurrentVersion {
			t.Errorf("Version = %d, want %d", state.Version, CurrentVersion)
		}
	})

	t.Run("returns false when no legacy file exists", func(t *testing.T) {
		groveDir := t.TempDir()
		legacyPath := filepath.Join(t.TempDir(), "nonexistent.json")

		migrated, err := MigrateFromLegacy(groveDir, legacyPath)
		if err != nil {
			t.Fatalf("MigrateFromLegacy() error = %v", err)
		}
		if migrated {
			t.Error("MigrateFromLegacy() returned true, want false")
		}
	})
}

func TestMigrateStateVersion(t *testing.T) {
	t.Run("migrates V0 state to current version", func(t *testing.T) {
		state := &State{
			Version: 0,
		}

		if err := migrateStateVersion(state); err != nil {
			t.Fatalf("migrateStateVersion() error = %v", err)
		}

		if state.Version != CurrentVersion {
			t.Errorf("Version = %d, want %d", state.Version, CurrentVersion)
		}
		if state.Worktrees == nil {
			t.Error("Worktrees should be initialized")
		}
	})

	t.Run("current version is unchanged", func(t *testing.T) {
		state := &State{
			Version:   CurrentVersion,
			Worktrees: map[string]*WorktreeState{"test": {Path: "/test"}},
		}

		if err := migrateStateVersion(state); err != nil {
			t.Fatalf("migrateStateVersion() error = %v", err)
		}

		if state.Version != CurrentVersion {
			t.Errorf("Version changed unexpectedly")
		}
		if len(state.Worktrees) != 1 {
			t.Errorf("Worktrees was modified")
		}
	})

	t.Run("initializes nil maps", func(t *testing.T) {
		state := &State{
			Version:   0,
			Worktrees: nil,
		}

		if err := migrateStateVersion(state); err != nil {
			t.Fatalf("migrateStateVersion() error = %v", err)
		}

		if state.Worktrees == nil {
			t.Error("Worktrees should be initialized")
		}
	})

	t.Run("rejects future-version state files", func(t *testing.T) {
		// Guards against silent data corruption when a user upgrades grove
		// (writing a newer-version state.json) and then downgrades back.
		state := &State{
			Version: CurrentVersion + 1,
		}

		err := migrateStateVersion(state)
		if err == nil {
			t.Fatal("expected error for future-version state, got nil")
		}
	})

	t.Run("backfills zero-valued timestamps", func(t *testing.T) {
		// Simulates state written by an earlier grove init that created the main
		// worktree without stamping CreatedAt/LastAccessedAt.
		state := &State{
			Version: CurrentVersion,
			Worktrees: map[string]*WorktreeState{
				"main": {Path: "/test"},
			},
		}

		before := time.Now()
		if err := migrateStateVersion(state); err != nil {
			t.Fatalf("migrateStateVersion() error = %v", err)
		}
		after := time.Now()

		ws := state.Worktrees["main"]
		if ws.CreatedAt.IsZero() {
			t.Error("CreatedAt was not backfilled")
		}
		if ws.LastAccessedAt.IsZero() {
			t.Error("LastAccessedAt was not backfilled")
		}
		if ws.CreatedAt.Before(before) || ws.CreatedAt.After(after) {
			t.Errorf("CreatedAt = %v, want between %v and %v", ws.CreatedAt, before, after)
		}
	})

	t.Run("preserves non-zero timestamps", func(t *testing.T) {
		fixed := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
		state := &State{
			Version: CurrentVersion,
			Worktrees: map[string]*WorktreeState{
				"main": {
					Path:           "/test",
					CreatedAt:      fixed,
					LastAccessedAt: fixed,
				},
			},
		}

		if err := migrateStateVersion(state); err != nil {
			t.Fatalf("migrateStateVersion() error = %v", err)
		}

		ws := state.Worktrees["main"]
		if !ws.CreatedAt.Equal(fixed) {
			t.Errorf("CreatedAt = %v, want %v (should not have been overwritten)", ws.CreatedAt, fixed)
		}
		if !ws.LastAccessedAt.Equal(fixed) {
			t.Errorf("LastAccessedAt = %v, want %v (should not have been overwritten)", ws.LastAccessedAt, fixed)
		}
	})
}

func TestBackupAndRestore(t *testing.T) {
	t.Run("backup and restore state", func(t *testing.T) {
		groveDir := t.TempDir()

		// Create manager and add some state
		mgr, err := NewManager(groveDir)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}

		if err := mgr.SetProject("test-project"); err != nil {
			t.Fatalf("SetProject() error = %v", err)
		}
		if err := mgr.AddWorktree("testing", &WorktreeState{Path: "/path", Branch: "main"}); err != nil {
			t.Fatalf("AddWorktree() error = %v", err)
		}

		// Create backup
		if err := BackupState(groveDir); err != nil {
			t.Fatalf("BackupState() error = %v", err)
		}

		// Verify backup exists
		backupFile := filepath.Join(groveDir, "state.json.bak")
		if _, err := os.Stat(backupFile); os.IsNotExist(err) {
			t.Fatal("backup file was not created")
		}

		// Modify state
		if err := mgr.RemoveWorktree("testing"); err != nil {
			t.Fatalf("RemoveWorktree() error = %v", err)
		}

		// Restore from backup
		if err := RestoreStateBackup(groveDir); err != nil {
			t.Fatalf("RestoreStateBackup() error = %v", err)
		}

		// Reload manager and verify original state
		mgr2, err := NewManager(groveDir)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}

		ws, _ := mgr2.GetWorktree("testing")
		if ws == nil {
			t.Error("testing worktree should exist after restore")
		}
	})

	t.Run("backup with no state file is no-op", func(t *testing.T) {
		groveDir := t.TempDir()

		// Backup with no state file should succeed
		if err := BackupState(groveDir); err != nil {
			t.Errorf("BackupState() error = %v, want nil", err)
		}
	})

	t.Run("restore with no backup returns error", func(t *testing.T) {
		groveDir := t.TempDir()

		err := RestoreStateBackup(groveDir)
		if err == nil {
			t.Error("RestoreStateBackup() should return error when no backup exists")
		}
	})
}

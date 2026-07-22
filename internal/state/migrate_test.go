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

func TestMigrateStateVersion_RekeysMainToRoot(t *testing.T) {
	st := &State{
		Version: CurrentVersion,
		Worktrees: map[string]*WorktreeState{
			"main":    {Path: "/repo", Root: true},
			"feature": {Path: "/repo-feature"},
		},
	}
	if err := migrateStateVersion(st); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, ok := st.Worktrees["main"]; ok {
		t.Error(`legacy "main" key still present after migration`)
	}
	root, ok := st.Worktrees["root"]
	if !ok {
		t.Fatal(`main worktree not rekeyed to "root"`)
	}
	if root.Path != "/repo" || !root.Root {
		t.Errorf("root entry wrong after rekey: %+v", root)
	}
	if _, ok := st.Worktrees["feature"]; !ok {
		t.Error("non-root worktree should be untouched")
	}
}

func TestMigrateStateVersion_KeepsExistingRoot(t *testing.T) {
	// A repo already on the new scheme must not have its root clobbered, even if
	// a stray "main"-keyed entry also exists.
	st := &State{
		Version: CurrentVersion,
		Worktrees: map[string]*WorktreeState{
			"root": {Path: "/repo-new", Root: true},
			"main": {Path: "/repo-old", Root: true},
		},
	}
	if err := migrateStateVersion(st); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if st.Worktrees["root"].Path != "/repo-new" {
		t.Errorf("existing root entry was clobbered: %+v", st.Worktrees["root"])
	}
}

// writeLegacyMainState writes a pre-0.10 state.json keyed "main" and returns
// the path of the state file.
func writeLegacyMainState(t *testing.T, groveDir string) string {
	t.Helper()
	stale := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	legacy := &State{
		Version: CurrentVersion,
		Worktrees: map[string]*WorktreeState{
			"main":    {Path: "/repo", Root: true, CreatedAt: stale, LastAccessedAt: stale},
			"feature": {Path: "/repo-feature", Branch: "feature", CreatedAt: stale, LastAccessedAt: stale},
		},
	}
	data, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		t.Fatalf("marshal legacy state: %v", err)
	}
	stateFile := filepath.Join(groveDir, "state.json")
	if err := os.WriteFile(stateFile, data, 0644); err != nil {
		t.Fatalf("write legacy state: %v", err)
	}
	return stateFile
}

func readDiskState(t *testing.T, stateFile string) *State {
	t.Helper()
	data, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("read state file: %v", err)
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatalf("parse state file: %v", err)
	}
	return &st
}

func TestMigrateRekey_PersistsThroughSave(t *testing.T) {
	// The load-time main→root rekey must survive save(): save rebases onto the
	// on-disk state, so an unmigrated disk file must not resurrect "main" on
	// disk or revert the in-memory rekey (the exact failure: every subsequent
	// TouchWorktree("root") missing, forever).
	groveDir := t.TempDir()
	stateFile := writeLegacyMainState(t, groveDir)

	mgr, err := NewManager(groveDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// A save that doesn't touch any worktree entry (the first save a real
	// switch performs) must still persist the rekey.
	if err := mgr.SetLastWorktree("feature"); err != nil {
		t.Fatalf("SetLastWorktree: %v", err)
	}
	disk := readDiskState(t, stateFile)
	if _, ok := disk.Worktrees["main"]; ok {
		t.Error(`save resurrected the legacy "main" key on disk`)
	}
	if _, ok := disk.Worktrees["root"]; !ok {
		t.Error(`rekeyed "root" entry missing from disk after save`)
	}

	// The in-memory state must not have been reverted by save's merge.
	if _, err := mgr.GetWorktree("root"); err != nil {
		t.Errorf(`in-memory "root" entry lost after save: %v`, err)
	}

	// Root lookups must work end-to-end after the save.
	if err := mgr.TouchWorktree("root"); err != nil {
		t.Errorf("TouchWorktree(root) after save: %v", err)
	}
	disk = readDiskState(t, stateFile)
	root, ok := disk.Worktrees["root"]
	if !ok {
		t.Fatal(`"root" entry missing from disk after touch`)
	}
	if root.LastAccessedAt.Year() < 2026 {
		t.Errorf("root last_accessed_at not updated on disk: %v", root.LastAccessedAt)
	}
}

func TestMigrateRekey_NoDuplicateOnDirtyRoot(t *testing.T) {
	// Touching "root" before the first save must not write both "root" (the
	// replayed dirty entry) and the disk file's stale "main" (both Root:true).
	groveDir := t.TempDir()
	stateFile := writeLegacyMainState(t, groveDir)

	mgr, err := NewManager(groveDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if err := mgr.TouchWorktree("root"); err != nil {
		t.Fatalf("TouchWorktree(root): %v", err)
	}

	disk := readDiskState(t, stateFile)
	if _, ok := disk.Worktrees["main"]; ok {
		t.Error(`stale "main" duplicate written alongside "root"`)
	}
	if _, ok := disk.Worktrees["root"]; !ok {
		t.Error(`"root" entry missing from disk`)
	}
}

func TestMigrateRekey_CleansStaleDuplicateOnDisk(t *testing.T) {
	// A disk file already corrupted with both "root" and a stale Root:true
	// "main" duplicate (written by a pre-fix build) must converge to "root"
	// only. A non-root worktree that happens to be named "main" is kept.
	groveDir := t.TempDir()
	stale := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	corrupt := &State{
		Version: CurrentVersion,
		Worktrees: map[string]*WorktreeState{
			"root": {Path: "/repo", Root: true, CreatedAt: stale, LastAccessedAt: stale},
			"main": {Path: "/repo", Root: true, CreatedAt: stale, LastAccessedAt: stale},
		},
	}
	data, _ := json.MarshalIndent(corrupt, "", "  ")
	stateFile := filepath.Join(groveDir, "state.json")
	if err := os.WriteFile(stateFile, data, 0644); err != nil {
		t.Fatalf("write state: %v", err)
	}

	mgr, err := NewManager(groveDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if err := mgr.SetLastWorktree("root"); err != nil {
		t.Fatalf("SetLastWorktree: %v", err)
	}

	disk := readDiskState(t, stateFile)
	if _, ok := disk.Worktrees["main"]; ok {
		t.Error(`stale Root:true "main" duplicate survived on disk`)
	}
	if _, ok := disk.Worktrees["root"]; !ok {
		t.Error(`"root" entry missing from disk`)
	}
}

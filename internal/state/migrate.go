package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lost-in-the/grove/internal/fsutil"
)

// LegacyState represents the V0/V1 state schema (frozen.json)
// This was stored at ~/.config/grove/state/frozen.json
// Note: Frozen field is no longer used in V2 but kept for parsing old files
type LegacyState struct {
	Frozen map[string]any `json:"frozen"`
}

// MigrateFromLegacy migrates legacy global frozen.json to project-specific state.json
// This is called during grove setup or first V2 command in an existing project.
// Returns true if migration occurred, false if no legacy state found.
func MigrateFromLegacy(groveDir string, legacyPath string) (bool, error) {
	// Check if legacy file exists
	if _, err := os.Stat(legacyPath); os.IsNotExist(err) {
		return false, nil
	}

	// Read legacy state
	data, err := os.ReadFile(legacyPath)
	if err != nil {
		return false, fmt.Errorf("failed to read legacy state: %w", err)
	}

	var legacy LegacyState
	if err := json.Unmarshal(data, &legacy); err != nil {
		return false, fmt.Errorf("failed to parse legacy state: %w", err)
	}

	// Create new V2 state (frozen data from legacy is not migrated)
	newState := &State{
		Version:   CurrentVersion,
		Worktrees: make(map[string]*WorktreeState),
	}

	// Write new state
	stateFile := filepath.Join(groveDir, "state.json")
	stateData, err := json.MarshalIndent(newState, "", "  ")
	if err != nil {
		return false, fmt.Errorf("failed to marshal new state: %w", err)
	}

	if err := fsutil.AtomicWriteFile(stateFile, stateData, 0644); err != nil {
		return false, fmt.Errorf("failed to write new state: %w", err)
	}

	return true, nil
}

// migrateStateVersion handles in-place migration of state.json between versions
// This is called when loading state to ensure it's up to date.
//
// Returns an error if state.Version is newer than CurrentVersion — that
// signals the user has downgraded grove after upgrading, and silently
// parsing a future-version file as the current schema risks data loss.
func migrateStateVersion(state *State) error {
	if state.Version > CurrentVersion {
		return fmt.Errorf("state.json version %d is newer than supported version %d; "+
			"upgrade grove or restore state from .grove/state.json.bak",
			state.Version, CurrentVersion)
	}

	if state.Version != CurrentVersion {
		// Future version migrations would go here.
		// For now, we only have version 1.
		if state.Version == 0 {
			// V0 -> V1: Just set version, structure is compatible
			state.Version = CurrentVersion
		}
	}

	// Ensure maps are initialized
	if state.Worktrees == nil {
		state.Worktrees = make(map[string]*WorktreeState)
	}

	// Rekey the main worktree entry to "root". Grove <=0.9 keyed it "main", but
	// runtime lookups moved to the literal "root" key (B22); without this rename
	// every TouchWorktree("root")/GetWorktree("root") silently misses on upgraded
	// repos — freezing the root's last-access, leaving a stale "main" entry, and
	// letting a later `grove new main` overwrite it. The main entry is identified
	// by its Root flag (set since v0.5), never by key, so a coincidentally
	// "main"-named non-root worktree is left alone. Skip if "root" already exists.
	if _, ok := state.Worktrees["root"]; !ok {
		for key, ws := range state.Worktrees {
			if ws != nil && ws.Root && key != "root" {
				state.Worktrees["root"] = ws
				delete(state.Worktrees, key)
				break
			}
		}
	} else {
		// "root" already exists: drop any *other* Root:true entry. Two root
		// entries can only come from a half-applied rekey (a pre-fix build
		// that persisted the dirty "root" entry without removing "main");
		// keeping the stale one would pin it on disk forever, since this
		// branch otherwise skips it. Non-root entries — including a worktree
		// legitimately named "main" — are never touched.
		for key, ws := range state.Worktrees {
			if ws != nil && ws.Root && key != "root" {
				delete(state.Worktrees, key)
			}
		}
	}

	// Backfill zero-valued timestamps from earlier versions of grove that
	// initialized worktree state without stamping CreatedAt/LastAccessedAt
	// (e.g. v0.6.1 and earlier created the main worktree's state without
	// either field). Without this, state.json keeps "0001-01-01T00:00:00Z"
	// values until the next operation touches the worktree, and `grove trim`
	// has to rely on its filesystem-fallback path. Stamping time.Now() here
	// gives upgraders a clean trim clock from the moment of the load.
	now := time.Now()
	for _, ws := range state.Worktrees {
		if ws == nil {
			continue
		}
		if ws.CreatedAt.IsZero() {
			ws.CreatedAt = now
		}
		if ws.LastAccessedAt.IsZero() {
			ws.LastAccessedAt = now
		}
	}

	return nil
}

// BackupState creates a backup of the current state file
func BackupState(groveDir string) error {
	stateFile := filepath.Join(groveDir, "state.json")
	backupFile := filepath.Join(groveDir, "state.json.bak")

	// Check if state file exists
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		return nil // Nothing to backup
	}

	// Read current state
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return fmt.Errorf("failed to read state for backup: %w", err)
	}

	// Write backup
	if err := fsutil.AtomicWriteFile(backupFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write state backup: %w", err)
	}

	return nil
}

// RestoreStateBackup restores state from backup file
func RestoreStateBackup(groveDir string) error {
	stateFile := filepath.Join(groveDir, "state.json")
	backupFile := filepath.Join(groveDir, "state.json.bak")

	// Check if backup exists
	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		return fmt.Errorf("no backup file found")
	}

	// Read backup
	data, err := os.ReadFile(backupFile)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	// Write to state file
	if err := fsutil.AtomicWriteFile(stateFile, data, 0644); err != nil {
		return fmt.Errorf("failed to restore state: %w", err)
	}

	return nil
}

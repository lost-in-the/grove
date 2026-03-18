package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

	if err := os.WriteFile(stateFile, stateData, 0644); err != nil {
		return false, fmt.Errorf("failed to write new state: %w", err)
	}

	return true, nil
}

// migrateStateVersion handles in-place migration of state.json between versions
// This is called when loading state to ensure it's up to date.
func migrateStateVersion(state *State) {
	if state.Version == CurrentVersion {
		return // Already current
	}

	// Future version migrations would go here
	// For now, we only have version 1
	if state.Version == 0 {
		// V0 -> V1: Just set version, structure is compatible
		state.Version = CurrentVersion
	}

	// Ensure maps are initialized
	if state.Worktrees == nil {
		state.Worktrees = make(map[string]*WorktreeState)
	}
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
	if err := os.WriteFile(backupFile, data, 0644); err != nil {
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
	if err := os.WriteFile(stateFile, data, 0644); err != nil {
		return fmt.Errorf("failed to restore state: %w", err)
	}

	return nil
}

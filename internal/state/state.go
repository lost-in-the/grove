package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// State represents the persisted state
type State struct {
	Frozen map[string]*FreezeInfo `json:"frozen"`
}

// FreezeInfo contains information about a frozen worktree
type FreezeInfo struct {
	Name     string    `json:"name"`
	FrozenAt time.Time `json:"frozen_at"`
}

// Manager handles state management for Grove
type Manager struct {
	stateDir  string
	stateFile string
	mu        sync.RWMutex
	state     *State
}

// NewManager creates a new state manager
// If stateDir is empty, it uses $HOME/.config/grove/state/
func NewManager(stateDir string) (*Manager, error) {
	if stateDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		stateDir = filepath.Join(homeDir, ".config", "grove", "state")
	}

	// Ensure state directory exists
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	stateFile := filepath.Join(stateDir, "frozen.json")

	mgr := &Manager{
		stateDir:  stateDir,
		stateFile: stateFile,
		state:     &State{Frozen: make(map[string]*FreezeInfo)},
	}

	// Load existing state if it exists
	if err := mgr.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	return mgr, nil
}

// Freeze marks a worktree as frozen
// This is idempotent - freezing an already frozen worktree is a no-op
func (m *Manager) Freeze(name string) error {
	if name == "" {
		return fmt.Errorf("worktree name cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Add to frozen map (or update if already exists)
	m.state.Frozen[name] = &FreezeInfo{
		Name:     name,
		FrozenAt: time.Now(),
	}

	return m.save()
}

// Resume clears the frozen state for a worktree
// This is idempotent - resuming a non-frozen worktree is a no-op
func (m *Manager) Resume(name string) error {
	if name == "" {
		return fmt.Errorf("worktree name cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove from frozen map (no-op if not present)
	delete(m.state.Frozen, name)

	return m.save()
}

// IsFrozen checks if a worktree is frozen
func (m *Manager) IsFrozen(name string) (bool, error) {
	if name == "" {
		return false, fmt.Errorf("worktree name cannot be empty")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	_, frozen := m.state.Frozen[name]
	return frozen, nil
}

// ListFrozen returns a sorted list of all frozen worktrees
func (m *Manager) ListFrozen() ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.state.Frozen))
	for name := range m.state.Frozen {
		names = append(names, name)
	}

	// Sort for consistent output
	sort.Strings(names)

	return names, nil
}

// load reads the state from disk
func (m *Manager) load() error {
	data, err := os.ReadFile(m.stateFile)
	if err != nil {
		return err
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to parse state file: %w", err)
	}

	// Ensure frozen map is initialized
	if state.Frozen == nil {
		state.Frozen = make(map[string]*FreezeInfo)
	}

	m.state = &state
	return nil
}

// save writes the state to disk
func (m *Manager) save() error {
	data, err := json.MarshalIndent(m.state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Write atomically by writing to temp file and renaming
	tmpFile := m.stateFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	if err := os.Rename(tmpFile, m.stateFile); err != nil {
		return fmt.Errorf("failed to save state file: %w", err)
	}

	return nil
}

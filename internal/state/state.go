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

// CurrentVersion is the current state schema version
const CurrentVersion = 1

// State represents the persisted state (V2 schema)
type State struct {
	Version      int                       `json:"version"`
	Project      string                    `json:"project"`
	LastWorktree string                    `json:"last_worktree,omitempty"`
	Worktrees    map[string]*WorktreeState `json:"worktrees"`
}

// WorktreeState contains metadata for a single worktree
// NOTE: Protected/Immutable are NOT stored in state - they come from config.toml
type WorktreeState struct {
	Path           string     `json:"path"`
	Branch         string     `json:"branch"`
	Root           bool       `json:"root"`
	DockerProject  string     `json:"docker_project"`
	CreatedAt      time.Time  `json:"created_at"`
	LastAccessedAt time.Time  `json:"last_accessed_at"`
	ParentWorktree string     `json:"parent_worktree,omitempty"`
	Environment    bool       `json:"environment,omitempty"`
	Mirror         string     `json:"mirror,omitempty"`
	LastSyncedAt   *time.Time `json:"last_synced_at,omitempty"`
	AgentSlot      int        `json:"agent_slot,omitempty"` // 0 = no agent stack running
}

// Manager handles state management for Grove
type Manager struct {
	groveDir  string // Path to .grove directory
	stateFile string
	mu        sync.RWMutex
	state     *State
}

// NewManager creates a new state manager for a grove project
// groveDir should be the path to the .grove directory (e.g., /path/to/project/.grove)
func NewManager(groveDir string) (*Manager, error) {
	if groveDir == "" {
		return nil, fmt.Errorf("grove directory path is required")
	}

	// Ensure grove directory exists
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create grove directory: %w", err)
	}

	stateFile := filepath.Join(groveDir, "state.json")

	mgr := &Manager{
		groveDir:  groveDir,
		stateFile: stateFile,
		state:     newEmptyState(),
	}

	// Load existing state if it exists
	if err := mgr.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	return mgr, nil
}

// newEmptyState creates a new empty state with initialized maps
func newEmptyState() *State {
	return &State{
		Version:   CurrentVersion,
		Worktrees: make(map[string]*WorktreeState),
	}
}

// --- V2 State Methods ---

// SetProject sets the project name in state
func (m *Manager) SetProject(name string) error {
	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.state.Project = name
	return m.save()
}

// GetProject returns the project name from state
func (m *Manager) GetProject() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state.Project
}

// GetLastWorktree returns the last active worktree name
func (m *Manager) GetLastWorktree() (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state.LastWorktree, nil
}

// SetLastWorktree updates the last active worktree
func (m *Manager) SetLastWorktree(name string) error {
	if name == "" {
		return fmt.Errorf("worktree name cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.state.LastWorktree = name
	return m.save()
}

// AddWorktree adds a new worktree to state
func (m *Manager) AddWorktree(name string, ws *WorktreeState) error {
	if name == "" {
		return fmt.Errorf("worktree name cannot be empty")
	}
	if ws == nil {
		return fmt.Errorf("worktree state cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.state.Worktrees[name] = ws
	return m.save()
}

// GetWorktree returns the worktree state for a given name
func (m *Manager) GetWorktree(name string) (*WorktreeState, error) {
	if name == "" {
		return nil, fmt.Errorf("worktree name cannot be empty")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	ws, ok := m.state.Worktrees[name]
	if !ok {
		return nil, nil // Not found, not an error
	}
	return ws, nil
}

// RemoveWorktree removes a worktree from state
func (m *Manager) RemoveWorktree(name string) error {
	if name == "" {
		return fmt.Errorf("worktree name cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.state.Worktrees, name)
	return m.save()
}

// TouchWorktree updates the last_accessed_at timestamp for a worktree
func (m *Manager) TouchWorktree(name string) error {
	if name == "" {
		return fmt.Errorf("worktree name cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	ws, ok := m.state.Worktrees[name]
	if !ok {
		return fmt.Errorf("worktree %q not found in state", name)
	}

	ws.LastAccessedAt = time.Now()
	return m.save()
}

// IsEnvironment checks if a worktree is an environment worktree
func (m *Manager) IsEnvironment(name string) (bool, error) {
	if name == "" {
		return false, fmt.Errorf("worktree name cannot be empty")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	ws, ok := m.state.Worktrees[name]
	if !ok {
		return false, nil
	}
	return ws.Environment, nil
}

// ListWorktrees returns a sorted list of all worktree names in state
func (m *Manager) ListWorktrees() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.state.Worktrees))
	for name := range m.state.Worktrees {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GetState returns a copy of the current state (for debugging/inspection)
func (m *Manager) GetState() State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return *m.state
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

	// Migrate state if needed
	if err := migrateStateVersion(&state); err != nil {
		return fmt.Errorf("failed to migrate state: %w", err)
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

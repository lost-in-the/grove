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

// State represents the persisted state (schema version 1)
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
	lockFile  string // .grove/state.lock
	mu        sync.RWMutex
	state     *State

	// Mutation tracking for save()'s cross-process merge: the on-disk state
	// is authoritative for everything this process didn't explicitly touch,
	// so setters record what they changed and save() replays exactly that
	// on top of a fresh disk read.
	removedWorktrees  map[string]bool // worktree entries explicitly removed
	dirtyWorktrees    map[string]bool // worktree entries added/updated
	projectDirty      bool            // Project was set by this process
	lastWorktreeDirty bool            // LastWorktree was set/cleared by this process

	batchDepth int  // > 0 while inside Batch — setters defer their save()
	batchDirty bool // a setter ran during the current batch
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
	lockFile := filepath.Join(groveDir, "state.lock")

	mgr := &Manager{
		groveDir:         groveDir,
		stateFile:        stateFile,
		lockFile:         lockFile,
		state:            newEmptyState(),
		removedWorktrees: make(map[string]bool),
		dirtyWorktrees:   make(map[string]bool),
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
	m.projectDirty = true
	return m.saveOrDefer()
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
	m.lastWorktreeDirty = true
	return m.saveOrDefer()
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
	m.dirtyWorktrees[name] = true
	return m.saveOrDefer()
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
	delete(m.dirtyWorktrees, name)
	m.removedWorktrees[name] = true

	// Clear LastWorktree if it pointed at the removed entry, so callers
	// like `grove last` don't return a name that no longer exists.
	if m.state.LastWorktree == name {
		m.state.LastWorktree = ""
		m.lastWorktreeDirty = true
	}

	return m.saveOrDefer()
}

// RenameWorktree renames a worktree in state, moving all fields to the new key.
// Returns an error if the old name doesn't exist or the new name is already taken.
func (m *Manager) RenameWorktree(oldName, newName string) error {
	if oldName == "" {
		return fmt.Errorf("old worktree name cannot be empty")
	}
	if newName == "" {
		return fmt.Errorf("new worktree name cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	ws, ok := m.state.Worktrees[oldName]
	if !ok {
		return fmt.Errorf("worktree %q not found in state", oldName)
	}

	if _, exists := m.state.Worktrees[newName]; exists {
		return fmt.Errorf("worktree %q already exists in state", newName)
	}

	// Move entry to new key, marking the old name as explicitly removed
	// so save()'s disk-merge doesn't resurrect it.
	delete(m.state.Worktrees, oldName)
	delete(m.dirtyWorktrees, oldName)
	m.removedWorktrees[oldName] = true
	m.state.Worktrees[newName] = ws
	m.dirtyWorktrees[newName] = true

	// Update LastWorktree if it pointed to the old name
	if m.state.LastWorktree == oldName {
		m.state.LastWorktree = newName
		m.lastWorktreeDirty = true
	}

	return m.saveOrDefer()
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
	m.dirtyWorktrees[name] = true
	return m.saveOrDefer()
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

// Batch suppresses individual save() calls inside fn and flushes a single
// save when the outermost batch ends. Use when a command runs multiple state
// mutations in sequence — collapses N flock+merge+rename cycles into one.
//
// The flush runs in a defer so it still happens if fn panics (the panic
// continues to propagate after the flush). os.Exit inside fn skips deferred
// saves — keep validation-failure exits before any state mutations.
//
// Contract: only the goroutine calling Batch should mutate this Manager
// during fn. Concurrent mutations from other goroutines would also have
// their saves suppressed and may not be flushed.
func (m *Manager) Batch(fn func() error) (retErr error) {
	m.mu.Lock()
	m.batchDepth++
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.batchDepth--
		if m.batchDepth == 0 && m.batchDirty {
			m.batchDirty = false
			if saveErr := m.save(); saveErr != nil && retErr == nil {
				retErr = saveErr
			}
		}
	}()

	return fn()
}

// saveOrDefer saves immediately when no batch is active, or just marks the
// in-memory state dirty when a Batch is in progress so the outermost Batch
// can flush once. Callers must hold m.mu (matches save's contract).
func (m *Manager) saveOrDefer() error {
	if m.batchDepth > 0 {
		m.batchDirty = true
		return nil
	}
	return m.save()
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
		return err
	}

	m.state = &state
	return nil
}

// save writes the state to disk with file locking and atomic rename.
// It merges with the current on-disk state to avoid clobbering concurrent
// changes from other grove processes (e.g., a `grove rm` in another shell
// while the TUI holds a long-lived Manager): the freshly-read disk state is
// the base, and only the mutations this process explicitly made (tracked in
// removedWorktrees/dirtyWorktrees and the scalar dirty flags) are replayed
// on top. Everything this process didn't touch — including entries other
// processes removed and their LastWorktree/Project updates — stays as disk
// has it.
func (m *Manager) save() error {
	f, err := m.fileLock()
	if err != nil {
		return fmt.Errorf("failed to lock state: %w", err)
	}
	defer m.fileUnlock(f)

	// Re-read current disk state under the lock and rebase our tracked
	// mutations onto it. A missing or corrupt state file falls through to
	// writing the in-memory state as-is.
	if diskData, err := os.ReadFile(m.stateFile); err == nil {
		var diskState State
		if err := json.Unmarshal(diskData, &diskState); err == nil {
			merged := &diskState
			if merged.Worktrees == nil {
				merged.Worktrees = make(map[string]*WorktreeState)
			}
			merged.Version = m.state.Version

			for name := range m.removedWorktrees {
				delete(merged.Worktrees, name)
			}
			for name := range m.dirtyWorktrees {
				if ws, ok := m.state.Worktrees[name]; ok {
					merged.Worktrees[name] = ws
				}
			}
			if m.projectDirty {
				merged.Project = m.state.Project
			}
			if m.lastWorktreeDirty {
				merged.LastWorktree = m.state.LastWorktree
			}
			// Never persist a LastWorktree pointing at an entry removed in
			// this save (e.g. another process set it while we removed the
			// worktree).
			if m.removedWorktrees[merged.LastWorktree] {
				merged.LastWorktree = ""
			}

			m.state = merged
		}
	}

	data, err := json.MarshalIndent(m.state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	tmpFile := m.stateFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	if err := os.Rename(tmpFile, m.stateFile); err != nil {
		return fmt.Errorf("failed to save state file: %w", err)
	}

	// Disk is now authoritative — clear the mutation tracking.
	m.removedWorktrees = make(map[string]bool)
	m.dirtyWorktrees = make(map[string]bool)
	m.projectDirty = false
	m.lastWorktreeDirty = false

	return nil
}

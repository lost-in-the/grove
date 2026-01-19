package time

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/LeahArmstrong/grove-cli/internal/hooks"
)

// TimeEntry represents a single time tracking entry
type TimeEntry struct {
	Worktree  string    `json:"worktree"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Duration  int64     `json:"duration_seconds"`
}

// WeeklySummary represents time tracked in the current week
type WeeklySummary struct {
	Total       time.Duration
	ByWorktree  map[string]time.Duration
	WeekStart   time.Time
	EntryCount  int
}

// TimeTracker manages time tracking for worktrees
type TimeTracker struct {
	stateFile      string
	mu             sync.Mutex
	activeSessions map[string]time.Time
}

// timeState represents the persisted state
type timeState struct {
	Entries        []TimeEntry       `json:"entries"`
	ActiveSessions map[string]string `json:"active_sessions"` // worktree -> start time (RFC3339)
}

// globalTracker is the singleton instance used by hooks
var globalTracker *TimeTracker

// InitializePlugin initializes the time tracking plugin and registers hooks
func InitializePlugin() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	stateDir := filepath.Join(homeDir, ".config", "grove", "state")
	tracker, err := NewTimeTracker(stateDir)
	if err != nil {
		return fmt.Errorf("failed to initialize time tracker: %w", err)
	}

	globalTracker = tracker

	// Register hooks
	hooks.Register(hooks.EventPostSwitch, onPostSwitch)
	hooks.Register(hooks.EventPreFreeze, onPreFreeze)
	hooks.Register(hooks.EventPostResume, onPostResume)

	return nil
}

// onPostSwitch handles the post-switch hook
func onPostSwitch(ctx *hooks.Context) error {
	if globalTracker == nil {
		return nil // Plugin not initialized
	}

	// End session for previous worktree if exists
	if ctx.PrevWorktree != "" {
		if _, err := globalTracker.EndSession(ctx.PrevWorktree); err != nil {
			// Log but don't fail
			fmt.Fprintf(os.Stderr, "warning: failed to end time session for %s: %v\n", ctx.PrevWorktree, err)
		}
	}

	// Start session for new worktree
	if ctx.Worktree != "" {
		if err := globalTracker.StartSession(ctx.Worktree); err != nil {
			// Log but don't fail
			fmt.Fprintf(os.Stderr, "warning: failed to start time session for %s: %v\n", ctx.Worktree, err)
		}
	}

	return nil
}

// onPreFreeze handles the pre-freeze hook
func onPreFreeze(ctx *hooks.Context) error {
	if globalTracker == nil {
		return nil // Plugin not initialized
	}

	// End session for the worktree being frozen
	if ctx.Worktree != "" {
		if _, err := globalTracker.EndSession(ctx.Worktree); err != nil {
			// Log but don't fail
			fmt.Fprintf(os.Stderr, "warning: failed to end time session for %s: %v\n", ctx.Worktree, err)
		}
	}

	return nil
}

// onPostResume handles the post-resume hook
func onPostResume(ctx *hooks.Context) error {
	if globalTracker == nil {
		return nil // Plugin not initialized
	}

	// Start session for resumed worktree
	if ctx.Worktree != "" {
		if err := globalTracker.StartSession(ctx.Worktree); err != nil {
			// Log but don't fail
			fmt.Fprintf(os.Stderr, "warning: failed to start time session for %s: %v\n", ctx.Worktree, err)
		}
	}

	return nil
}

// NewTimeTracker creates a new time tracker
func NewTimeTracker(stateDir string) (*TimeTracker, error) {
	// Ensure state directory exists
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	stateFile := filepath.Join(stateDir, "time.json")
	
	tracker := &TimeTracker{
		stateFile:      stateFile,
		activeSessions: make(map[string]time.Time),
	}

	// Load existing state
	if err := tracker.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	return tracker, nil
}

// Track records a time entry for a worktree
func (t *TimeTracker) Track(worktree string, startTime, endTime time.Time) error {
	if worktree == "" {
		return fmt.Errorf("worktree name cannot be empty")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	duration := endTime.Sub(startTime)
	entry := TimeEntry{
		Worktree:  worktree,
		StartTime: startTime,
		EndTime:   endTime,
		Duration:  int64(duration.Seconds()),
	}

	return t.appendEntry(entry)
}

// StartSession starts a timing session for a worktree
func (t *TimeTracker) StartSession(worktree string) error {
	if worktree == "" {
		return fmt.Errorf("worktree name cannot be empty")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Initialize map if nil
	if t.activeSessions == nil {
		t.activeSessions = make(map[string]time.Time)
	}

	// Idempotent - if already started, just update the time
	t.activeSessions[worktree] = time.Now()
	
	return t.save()
}

// EndSession ends a timing session and records the duration
func (t *TimeTracker) EndSession(worktree string) (time.Duration, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	startTime, exists := t.activeSessions[worktree]
	if !exists {
		// Not an error - session might have already ended
		return 0, nil
	}

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	// Remove from active sessions
	delete(t.activeSessions, worktree)

	// Record the entry
	entry := TimeEntry{
		Worktree:  worktree,
		StartTime: startTime,
		EndTime:   endTime,
		Duration:  int64(duration.Seconds()),
	}

	if err := t.appendEntry(entry); err != nil {
		return 0, err
	}

	return duration, nil
}

// HasActiveSession checks if a worktree has an active timing session
func (t *TimeTracker) HasActiveSession(worktree string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	_, exists := t.activeSessions[worktree]
	return exists
}

// GetEntries returns all time entries for a worktree
func (t *TimeTracker) GetEntries(worktree string) ([]TimeEntry, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	state, err := t.loadState()
	if err != nil {
		if os.IsNotExist(err) {
			return []TimeEntry{}, nil
		}
		return nil, err
	}

	var entries []TimeEntry
	for _, entry := range state.Entries {
		if entry.Worktree == worktree {
			entries = append(entries, entry)
		}
	}

	return entries, nil
}

// GetTotal returns the total time spent in a worktree
func (t *TimeTracker) GetTotal(worktree string) (time.Duration, error) {
	entries, err := t.GetEntries(worktree)
	if err != nil {
		return 0, err
	}

	var total time.Duration
	for _, entry := range entries {
		total += time.Duration(entry.Duration) * time.Second
	}

	return total, nil
}

// GetAllWorktrees returns a list of all worktrees that have time entries
func (t *TimeTracker) GetAllWorktrees() []string {
	t.mu.Lock()
	defer t.mu.Unlock()

	state, err := t.loadState()
	if err != nil {
		return []string{}
	}

	worktrees := make(map[string]bool)
	for _, entry := range state.Entries {
		worktrees[entry.Worktree] = true
	}

	result := make([]string, 0, len(worktrees))
	for w := range worktrees {
		result = append(result, w)
	}

	sort.Strings(result)
	return result
}

// GetWeeklySummary returns time tracking data for the current week
func (t *TimeTracker) GetWeeklySummary() (*WeeklySummary, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	state, err := t.loadState()
	if err != nil {
		if os.IsNotExist(err) {
			return &WeeklySummary{
				ByWorktree: make(map[string]time.Duration),
			}, nil
		}
		return nil, err
	}

	// Calculate start of current week (Monday)
	now := time.Now()
	weekday := now.Weekday()
	daysFromMonday := int(weekday - time.Monday)
	if weekday == time.Sunday {
		daysFromMonday = 6
	}
	weekStart := now.AddDate(0, 0, -daysFromMonday).Truncate(24 * time.Hour)

	summary := &WeeklySummary{
		ByWorktree: make(map[string]time.Duration),
		WeekStart:  weekStart,
	}

	for _, entry := range state.Entries {
		// Check if entry is within current week
		if entry.StartTime.Before(weekStart) {
			continue
		}

		duration := time.Duration(entry.Duration) * time.Second
		summary.Total += duration
		summary.ByWorktree[entry.Worktree] += duration
		summary.EntryCount++
	}

	return summary, nil
}

// appendEntry adds an entry to the state file (must hold lock)
func (t *TimeTracker) appendEntry(entry TimeEntry) error {
	state, err := t.loadState()
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	state.Entries = append(state.Entries, entry)
	return t.saveState(state)
}

// load reads active sessions from state file
func (t *TimeTracker) load() error {
	state, err := t.loadState()
	if err != nil {
		return err
	}

	// Parse active sessions
	for worktree, timeStr := range state.ActiveSessions {
		startTime, err := time.Parse(time.RFC3339, timeStr)
		if err != nil {
			// Skip invalid entries
			continue
		}
		t.activeSessions[worktree] = startTime
	}

	return nil
}

// save writes current state to disk (must hold lock)
func (t *TimeTracker) save() error {
	state, err := t.loadState()
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Update active sessions
	state.ActiveSessions = make(map[string]string)
	for worktree, startTime := range t.activeSessions {
		state.ActiveSessions[worktree] = startTime.Format(time.RFC3339)
	}

	return t.saveState(state)
}

// loadState reads the state file (must hold lock)
func (t *TimeTracker) loadState() (*timeState, error) {
	data, err := os.ReadFile(t.stateFile)
	if err != nil {
		return &timeState{
			Entries:        []TimeEntry{},
			ActiveSessions: make(map[string]string),
		}, err
	}

	var state timeState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	if state.ActiveSessions == nil {
		state.ActiveSessions = make(map[string]string)
	}

	return &state, nil
}

// saveState writes state to disk (must hold lock)
func (t *TimeTracker) saveState(state *timeState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Atomic write: write to temp file, then rename
	tmpFile := t.stateFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	if err := os.Rename(tmpFile, t.stateFile); err != nil {
		return fmt.Errorf("failed to rename state file: %w", err)
	}

	return nil
}

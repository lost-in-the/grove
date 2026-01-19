package time

import (
	"path/filepath"
	"testing"
	"time"
)

func TestTimeTracker_Track(t *testing.T) {
	tests := []struct {
		name      string
		worktree  string
		startTime time.Time
		endTime   time.Time
		wantErr   bool
	}{
		{
			name:      "track basic session",
			worktree:  "test-tree",
			startTime: time.Now().Add(-1 * time.Hour),
			endTime:   time.Now(),
			wantErr:   false,
		},
		{
			name:      "track short session",
			worktree:  "short-tree",
			startTime: time.Now().Add(-5 * time.Minute),
			endTime:   time.Now(),
			wantErr:   false,
		},
		{
			name:      "empty worktree name",
			worktree:  "",
			startTime: time.Now().Add(-1 * time.Hour),
			endTime:   time.Now(),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory for state
			tmpDir := t.TempDir()
			stateFile := filepath.Join(tmpDir, "time.json")

			tracker := &TimeTracker{
				stateFile:      stateFile,
				activeSessions: make(map[string]time.Time),
			}

			err := tracker.Track(tt.worktree, tt.startTime, tt.endTime)
			if (err != nil) != tt.wantErr {
				t.Errorf("Track() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				// Verify entry was recorded
				entries, err := tracker.GetEntries(tt.worktree)
				if err != nil {
					t.Fatalf("GetEntries() error = %v", err)
				}
				if len(entries) == 0 {
					t.Errorf("expected at least one entry, got none")
				}
			}
		})
	}
}

func TestTimeTracker_GetTotal(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "time.json")

	tracker := &TimeTracker{
		stateFile:      stateFile,
		activeSessions: make(map[string]time.Time),
	}

	// Track multiple sessions
	now := time.Now()
	tracker.Track("test-tree", now.Add(-2*time.Hour), now.Add(-1*time.Hour))
	tracker.Track("test-tree", now.Add(-30*time.Minute), now)

	total, err := tracker.GetTotal("test-tree")
	if err != nil {
		t.Fatalf("GetTotal() error = %v", err)
	}

	// Should be approximately 1.5 hours (1h + 30m)
	expected := 90 * time.Minute
	if total < expected-time.Minute || total > expected+time.Minute {
		t.Errorf("GetTotal() = %v, want approximately %v", total, expected)
	}
}

func TestTimeTracker_GetAllWorktrees(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "time.json")

	tracker := &TimeTracker{
		stateFile:      stateFile,
		activeSessions: make(map[string]time.Time),
	}

	// Track sessions for multiple worktrees
	now := time.Now()
	tracker.Track("tree1", now.Add(-1*time.Hour), now)
	tracker.Track("tree2", now.Add(-30*time.Minute), now)
	tracker.Track("tree1", now.Add(-15*time.Minute), now)

	worktrees := tracker.GetAllWorktrees()
	if len(worktrees) != 2 {
		t.Errorf("GetAllWorktrees() = %d worktrees, want 2", len(worktrees))
	}

	// Verify both worktrees are present
	found := make(map[string]bool)
	for _, w := range worktrees {
		found[w] = true
	}
	if !found["tree1"] || !found["tree2"] {
		t.Errorf("GetAllWorktrees() missing expected worktrees")
	}
}

func TestTimeTracker_GetWeeklySummary(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "time.json")

	tracker := &TimeTracker{
		stateFile:      stateFile,
		activeSessions: make(map[string]time.Time),
	}

	// Track sessions this week and last week
	now := time.Now()

	// Calculate start of this week to ensure we're tracking in the right period
	weekday := now.Weekday()
	daysFromMonday := int(weekday - time.Monday)
	if weekday == time.Sunday {
		daysFromMonday = 6
	}
	weekStart := now.AddDate(0, 0, -daysFromMonday).Truncate(24 * time.Hour)

	// Create entries definitely within this week (yesterday and today)
	yesterday := now.Add(-24 * time.Hour)
	if yesterday.Before(weekStart) {
		// If yesterday was last week, use today instead
		yesterday = now
	}

	tracker.Track("tree1", yesterday, yesterday.Add(1*time.Hour))
	tracker.Track("tree2", yesterday.Add(2*time.Hour), yesterday.Add(3*time.Hour))

	// This one should not be included (last week)
	lastWeek := weekStart.Add(-48 * time.Hour)
	tracker.Track("tree1", lastWeek, lastWeek.Add(2*time.Hour))

	summary, err := tracker.GetWeeklySummary()
	if err != nil {
		t.Fatalf("GetWeeklySummary() error = %v", err)
	}

	// Should have 2 hours total this week
	expected := 2 * time.Hour
	if summary.Total < expected-time.Minute || summary.Total > expected+time.Minute {
		t.Errorf("GetWeeklySummary() total = %v, want approximately %v (got %v worktrees with %d entries)", summary.Total, expected, len(summary.ByWorktree), summary.EntryCount)
	}

	// Should have 2 worktrees
	if len(summary.ByWorktree) != 2 {
		t.Errorf("GetWeeklySummary() worktrees = %d, want 2", len(summary.ByWorktree))
	}
}

func TestTimeTracker_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "time.json")

	// Create tracker and record time
	tracker1 := &TimeTracker{
		stateFile:      stateFile,
		activeSessions: make(map[string]time.Time),
	}

	now := time.Now()
	err := tracker1.Track("test-tree", now.Add(-1*time.Hour), now)
	if err != nil {
		t.Fatalf("Track() error = %v", err)
	}

	// Create new tracker instance and verify data persists
	tracker2 := &TimeTracker{
		stateFile:      stateFile,
		activeSessions: make(map[string]time.Time),
	}

	total, err := tracker2.GetTotal("test-tree")
	if err != nil {
		t.Fatalf("GetTotal() error = %v", err)
	}

	if total < 59*time.Minute || total > 61*time.Minute {
		t.Errorf("GetTotal() = %v, want approximately 1 hour", total)
	}
}

func TestTimeTracker_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "time.json")

	tracker := &TimeTracker{
		stateFile:      stateFile,
		activeSessions: make(map[string]time.Time),
	}

	// Simulate concurrent writes
	done := make(chan bool)
	now := time.Now()

	for i := 0; i < 5; i++ {
		go func(n int) {
			tracker.Track("concurrent-tree", now.Add(-1*time.Hour), now)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 5; i++ {
		<-done
	}

	// Verify all entries were recorded
	entries, err := tracker.GetEntries("concurrent-tree")
	if err != nil {
		t.Fatalf("GetEntries() error = %v", err)
	}

	if len(entries) != 5 {
		t.Errorf("expected 5 entries, got %d", len(entries))
	}
}

func TestNewTimeTracker(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func() string
		wantErr   bool
	}{
		{
			name: "valid state directory",
			setupFunc: func() string {
				return t.TempDir()
			},
			wantErr: false,
		},
		{
			name: "creates missing directory",
			setupFunc: func() string {
				tmpDir := t.TempDir()
				return filepath.Join(tmpDir, "subdir", "time")
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateDir := tt.setupFunc()
			tracker, err := NewTimeTracker(stateDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewTimeTracker() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && tracker == nil {
				t.Errorf("NewTimeTracker() returned nil tracker")
			}
		})
	}
}

func TestTimeTracker_StartSession(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "time.json")

	tracker := &TimeTracker{
		stateFile:      stateFile,
		activeSessions: make(map[string]time.Time),
	}

	err := tracker.StartSession("test-tree")
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}

	// Verify active session exists
	if !tracker.HasActiveSession("test-tree") {
		t.Errorf("expected active session for test-tree")
	}

	// Starting another session for same tree should be idempotent
	err = tracker.StartSession("test-tree")
	if err != nil {
		t.Errorf("StartSession() second call error = %v", err)
	}
}

func TestTimeTracker_EndSession(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "time.json")

	tracker := &TimeTracker{
		stateFile:      stateFile,
		activeSessions: make(map[string]time.Time),
	}

	// Start a session
	tracker.StartSession("test-tree")

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// End the session
	duration, err := tracker.EndSession("test-tree")
	if err != nil {
		t.Fatalf("EndSession() error = %v", err)
	}

	if duration < 100*time.Millisecond {
		t.Errorf("duration = %v, want >= 100ms", duration)
	}

	// Verify session is no longer active
	if tracker.HasActiveSession("test-tree") {
		t.Errorf("expected no active session after EndSession()")
	}

	// Ending again should not error
	_, err = tracker.EndSession("test-tree")
	if err != nil {
		t.Errorf("EndSession() on inactive session error = %v", err)
	}
}

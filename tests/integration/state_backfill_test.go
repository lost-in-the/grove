//go:build integration

// Package integration_test contains end-to-end integration tests for the grove CLI.
// These tests require git to be available and may take longer than unit tests.
// Run with: go test -v -tags=integration ./tests/integration/
package integration_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lost-in-the/grove/internal/state"
)

// TestStateBackfill_ZeroTimestampsGetFilled verifies that a state.json written with
// zero-valued CreatedAt/LastAccessedAt timestamps (as produced by grove v0.6.1 and
// earlier) gets backfilled to non-zero values on load via state.NewManager.
// This covers the migrateStateVersion backfill path in internal/state/migrate.go.
func TestStateBackfill_ZeroTimestampsGetFilled(t *testing.T) {
	groveDir := filepath.Join(t.TempDir(), ".grove")
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write a state.json with zero-valued timestamps, as if written by old grove.
	zeroTime := time.Time{} // 0001-01-01T00:00:00Z
	oldState := map[string]interface{}{
		"version": 1,
		"project": "test-project",
		"worktrees": map[string]interface{}{
			"test-project-main": map[string]interface{}{
				"path":             "/tmp/test-project",
				"branch":           "main",
				"root":             true,
				"docker_project":   "",
				"created_at":       zeroTime,
				"last_accessed_at": zeroTime,
			},
			"test-project-feat": map[string]interface{}{
				"path":             "/tmp/test-project-feat",
				"branch":           "feat",
				"root":             false,
				"docker_project":   "",
				"created_at":       zeroTime,
				"last_accessed_at": zeroTime,
			},
		},
	}

	data, err := json.Marshal(oldState)
	if err != nil {
		t.Fatalf("marshal old state: %v", err)
	}
	stateFile := filepath.Join(groveDir, "state.json")
	if err := os.WriteFile(stateFile, data, 0644); err != nil {
		t.Fatalf("write state.json: %v", err)
	}

	// Record the time just before loading so we can assert stamps are >= it.
	before := time.Now().Add(-time.Second)

	// NewManager loads and migrates state automatically.
	mgr, err := state.NewManager(groveDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	st := mgr.GetState()
	if len(st.Worktrees) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(st.Worktrees))
	}

	for name, ws := range st.Worktrees {
		if ws == nil {
			t.Errorf("worktree %q has nil state", name)
			continue
		}
		if ws.CreatedAt.IsZero() {
			t.Errorf("worktree %q: CreatedAt is still zero after backfill", name)
		}
		if ws.LastAccessedAt.IsZero() {
			t.Errorf("worktree %q: LastAccessedAt is still zero after backfill", name)
		}
		if ws.CreatedAt.Before(before) {
			t.Errorf("worktree %q: CreatedAt %v predates load time %v — not freshly backfilled",
				name, ws.CreatedAt, before)
		}
		if ws.LastAccessedAt.Before(before) {
			t.Errorf("worktree %q: LastAccessedAt %v predates load time %v — not freshly backfilled",
				name, ws.LastAccessedAt, before)
		}
	}
}

// TestStateBackfill_NonZeroTimestampsUntouched verifies that worktrees with valid
// timestamps are not overwritten by the backfill migration.
func TestStateBackfill_NonZeroTimestampsUntouched(t *testing.T) {
	groveDir := filepath.Join(t.TempDir(), ".grove")
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		t.Fatal(err)
	}

	existingTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	oldState := map[string]interface{}{
		"version": 1,
		"project": "test-project",
		"worktrees": map[string]interface{}{
			"test-project-main": map[string]interface{}{
				"path":             "/tmp/test-project",
				"branch":           "main",
				"root":             true,
				"docker_project":   "",
				"created_at":       existingTime,
				"last_accessed_at": existingTime,
			},
		},
	}

	data, err := json.Marshal(oldState)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(groveDir, "state.json"), data, 0644); err != nil {
		t.Fatalf("write state.json: %v", err)
	}

	mgr, err := state.NewManager(groveDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	st := mgr.GetState()
	ws, ok := st.Worktrees["test-project-main"]
	if !ok || ws == nil {
		t.Fatal("expected test-project-main in state")
	}

	// Timestamps should be preserved — backfill only touches zero values.
	if !ws.CreatedAt.Equal(existingTime) {
		t.Errorf("CreatedAt changed: got %v, want %v", ws.CreatedAt, existingTime)
	}
	if !ws.LastAccessedAt.Equal(existingTime) {
		t.Errorf("LastAccessedAt changed: got %v, want %v", ws.LastAccessedAt, existingTime)
	}
}

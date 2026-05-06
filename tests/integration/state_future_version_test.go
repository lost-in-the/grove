//go:build integration

// Future-version state.json error guard. Covers the manual validation in PR #48
// that asks: "simulate a future-version state.json and confirm grove errors
// instead of silently parsing." End-to-end via state.NewManager so this
// verifies the production load path (migrate.go → state.go).

package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/state"
)

func TestStateLoad_RejectsFutureVersion(t *testing.T) {
	groveDir := filepath.Join(t.TempDir(), ".grove")
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write a state.json claiming a version far in the future. This simulates
	// a user who upgraded grove (which wrote a newer schema), then downgraded
	// back. Without the guard added in v0.7.0, the older grove would silently
	// parse the file as v1 and risk dropping fields it doesn't recognize.
	futureState := []byte(`{"version": 99, "worktrees": {}}`)
	if err := os.WriteFile(filepath.Join(groveDir, "state.json"), futureState, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := state.NewManager(groveDir)
	if err == nil {
		t.Fatal("expected state.NewManager to error on future-version state.json, got nil")
	}

	// The error should explicitly mention the version mismatch so the user
	// knows what's going on.
	msg := err.Error()
	if !strings.Contains(msg, "version 99") || !strings.Contains(msg, "newer than supported") {
		t.Errorf("expected error to identify the future version; got: %v", err)
	}
}

func TestStateLoad_AcceptsCurrentVersion(t *testing.T) {
	groveDir := filepath.Join(t.TempDir(), ".grove")
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Sanity check: the current version still loads without error.
	currentState := []byte(`{"version": 1, "worktrees": {}}`)
	if err := os.WriteFile(filepath.Join(groveDir, "state.json"), currentState, 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := state.NewManager(groveDir); err != nil {
		t.Fatalf("current-version state should load cleanly: %v", err)
	}
}

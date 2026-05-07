//go:build integration

// End-to-end tests for `grove context` and `grove context --json`.
//
// Scenarios:
//   - Inside a valid grove worktree → human output includes key fields
//   - Inside a valid grove worktree + --json → JSON output has correct schema
//   - Outside any grove project → exit non-zero with error message
package integration_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/testhelper"
)

// setupContextRepo creates a temp grove project with a linked worktree and
// returns (repoPath, worktreePath).
func setupContextRepo(t *testing.T) (string, string) {
	t.Helper()
	repo := testhelper.SetupRailsFixture(t)

	wtPath := filepath.Join(filepath.Dir(repo), "rails-app-ctx-test")
	testhelper.RunGit(t, repo, "worktree", "add", "-b", "ctx-test", wtPath)

	// Minimal state.json so RequireGroveContext resolves cleanly.
	groveDir := filepath.Join(repo, ".grove")
	state := []byte(`{"version": 1, "worktrees": {}}`)
	if err := os.WriteFile(filepath.Join(groveDir, "state.json"), state, 0644); err != nil {
		t.Fatalf("write state.json: %v", err)
	}

	return repo, wtPath
}

// TestContextCmd_HumanOutput verifies that `grove context` running inside a
// grove worktree produces human-readable output containing key fields.
func TestContextCmd_HumanOutput(t *testing.T) {
	_, wtPath := setupContextRepo(t)
	binary := buildGroveBinary(t)

	cmd := exec.Command(binary, "context")
	cmd.Dir = wtPath
	cmd.Env = testhelper.GitEnv()

	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			t.Fatalf("grove context failed: %v\nstderr: %s\nstdout: %s", err, ee.Stderr, out)
		}
		t.Fatalf("grove context failed: %v", err)
	}

	text := string(out)

	// Branch and display name must appear.
	if !strings.Contains(text, "ctx-test") {
		t.Errorf("expected 'ctx-test' in output, got:\n%s", text)
	}
	// Status must appear.
	if !strings.Contains(text, "clean") && !strings.Contains(text, "dirty") {
		t.Errorf("expected status (clean/dirty) in output, got:\n%s", text)
	}
	// Path must appear.
	if !strings.Contains(text, wtPath) {
		t.Errorf("expected worktree path %q in output, got:\n%s", wtPath, text)
	}
	// Recent commits section must appear.
	if !strings.Contains(text, "Recent") {
		t.Errorf("expected 'Recent' section in output, got:\n%s", text)
	}
}

// TestContextCmd_JSONOutput verifies that `grove context --json` produces
// valid JSON with the expected schema.
func TestContextCmd_JSONOutput(t *testing.T) {
	_, wtPath := setupContextRepo(t)
	binary := buildGroveBinary(t)

	cmd := exec.Command(binary, "context", "--json")
	cmd.Dir = wtPath
	cmd.Env = testhelper.GitEnv()

	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			t.Fatalf("grove context --json failed: %v\nstderr: %s\nstdout: %s", err, ee.Stderr, out)
		}
		t.Fatalf("grove context --json failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, out)
	}

	// Required top-level keys.
	for _, key := range []string{"name", "path", "branch", "commit", "status", "ahead", "behind", "stash_count", "recent_commits"} {
		if _, ok := result[key]; !ok {
			t.Errorf("expected key %q in JSON output, got keys: %v", key, jsonKeys(result))
		}
	}

	// Sanity: name should be non-empty.
	if name, _ := result["name"].(string); name == "" {
		t.Errorf("expected non-empty 'name' in JSON output")
	}

	// status must be "clean" or "dirty".
	if status, _ := result["status"].(string); status != "clean" && status != "dirty" {
		t.Errorf("expected status 'clean' or 'dirty', got %q", status)
	}

	// commit must be an object with sha and message.
	commit, ok := result["commit"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'commit' to be an object, got %T", result["commit"])
	}
	if _, ok := commit["sha"]; !ok {
		t.Error("expected 'commit.sha' in JSON output")
	}
	if _, ok := commit["message"]; !ok {
		t.Error("expected 'commit.message' in JSON output")
	}

	// recent_commits must be an array.
	if _, ok := result["recent_commits"].([]interface{}); !ok {
		t.Errorf("expected 'recent_commits' to be an array, got %T", result["recent_commits"])
	}
}

// TestContextCmd_OutsideGroveProject verifies that running `grove context`
// outside a grove project exits non-zero with a meaningful error.
func TestContextCmd_OutsideGroveProject(t *testing.T) {
	binary := buildGroveBinary(t)

	// Use a plain tempdir with no git repo.
	emptyDir := t.TempDir()

	cmd := exec.Command(binary, "context")
	cmd.Dir = emptyDir
	cmd.Env = testhelper.GitEnv()

	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit, got success\noutput: %s", out)
	}

	// Output should mention that we're not in a grove project.
	text := string(out)
	if !strings.Contains(strings.ToLower(text), "grove") {
		t.Errorf("expected grove-related error message, got: %s", text)
	}
}

// jsonKeys returns all keys from a map for error messages.
func jsonKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

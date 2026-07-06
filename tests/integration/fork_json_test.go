//go:build integration

// End-to-end coverage for `grove fork --json` keeping its stdout limited to
// a single parseable JSON object (see commit 12d6574, "fix(fork): keep
// --json output parseable").
package integration_test

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/testhelper"
)

// TestForkJSON_ValidJSONOutput verifies that `grove fork <name> --json`
// against a clean worktree produces stdout that is exactly one JSON object
// (no human-readable success/info lines mixed in before it).
func TestForkJSON_ValidJSONOutput(t *testing.T) {
	binary := buildGroveBinary(t)

	repo := testhelper.SetupRailsFixture(t)

	cmd := exec.Command(binary, "fork", "json-fork", "--json", "--no-switch")
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			t.Fatalf("grove fork --json failed: %v\nstderr: %s\nstdout: %s", err, ee.Stderr, out)
		}
		t.Fatalf("grove fork --json failed: %v\nstdout: %s", err, out)
	}

	var result struct {
		Name    string `json:"name"`
		Branch  string `json:"branch"`
		Path    string `json:"path"`
		Parent  string `json:"parent"`
		Created bool   `json:"created"`
	}

	// Decode exactly one JSON value, then confirm nothing but whitespace
	// follows — this is what would fail if a human-readable warning/success
	// line leaked onto stdout ahead of (or after) the JSON object.
	dec := json.NewDecoder(strings.NewReader(string(out)))
	if err := dec.Decode(&result); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, out)
	}
	if dec.More() {
		t.Fatalf("stdout contains extra content after the JSON object:\nstdout: %s", out)
	}

	if result.Name != "json-fork" {
		t.Errorf("result.Name = %q, want %q", result.Name, "json-fork")
	}
	if !result.Created {
		t.Error("result.Created = false, want true")
	}
	if !filepath.IsAbs(result.Path) {
		t.Errorf("result.Path %q is not absolute", result.Path)
	}
}

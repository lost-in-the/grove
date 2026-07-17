//go:build integration

package integration_test

import (
	"encoding/json"
	"errors"
	"os/exec"
	"testing"

	"github.com/lost-in-the/grove/internal/testhelper"
)

// TestSyncJSON_ErrorPathsEmitJSON: `grove sync <name> --json` must emit the
// JSON result document even when it terminates with a non-zero exit code —
// the old mid-loop os.Exit produced empty stdout, so scripted consumers got
// no parseable output on exactly the error paths they need to detect.
func TestSyncJSON_ErrorPathsEmitJSON(t *testing.T) {
	binary := buildGroveBinary(t)
	repo := testhelper.SetupRailsFixture(t)

	cmd := exec.Command(binary, "sync", "ghost", "--json")
	cmd.Dir = repo
	out, err := cmd.Output()

	// A typo'd target must still be an error (exit code preserved) ...
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("grove sync ghost --json: expected non-zero exit, got err=%v", err)
	}

	// ... but the JSON document must have been emitted first.
	var result struct {
		Synced  []any `json:"synced"`
		Skipped []struct {
			Name   string `json:"name"`
			Reason string `json:"reason"`
		} `json:"skipped"`
	}
	if jsonErr := json.Unmarshal(out, &result); jsonErr != nil {
		t.Fatalf("stdout is not valid JSON on the error path: %v\nstdout: %q", jsonErr, out)
	}
	if len(result.Skipped) != 1 || result.Skipped[0].Name != "ghost" {
		t.Errorf("expected skipped entry for 'ghost', got %+v", result.Skipped)
	}
}

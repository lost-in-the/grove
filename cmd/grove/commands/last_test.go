package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/cli"
)

func TestNoPreviousWorktree_HintAndSuccess(t *testing.T) {
	var buf bytes.Buffer
	w := cli.NewWriter(&buf, false)

	// `grove last` with nothing to switch back to is a no-op, not a failure:
	// it must return nil (exit 0) and point the user at `grove to`.
	if err := noPreviousWorktree(w, false); err != nil {
		t.Fatalf("noPreviousWorktree() should not error, got: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "No previous worktree") {
		t.Errorf("expected a 'no previous worktree' message, got: %q", out)
	}
	if !strings.Contains(out, "grove to") {
		t.Errorf("expected a 'grove to <name>' hint, got: %q", out)
	}
}

func TestNoPreviousWorktree_JSONEmitsValidObject(t *testing.T) {
	// In --json mode the no-op path must emit a parseable JSON object on stdout
	// (not a human sentence), so machine consumers never choke on this branch.
	stdout, err := captureStdout(func() error {
		return noPreviousWorktree(io.Discard, true)
	})
	if err != nil {
		t.Fatalf("noPreviousWorktree(json) should not error, got: %v", err)
	}

	var payload struct {
		SwitchTo string `json:"switch_to"`
		Message  string `json:"message"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("expected valid JSON on stdout, got %q (err: %v)", stdout, err)
	}
	if payload.SwitchTo != "" {
		t.Errorf("expected empty switch_to, got %q", payload.SwitchTo)
	}
	if !strings.Contains(payload.Message, "No previous worktree") {
		t.Errorf("expected explanatory message, got %q", payload.Message)
	}
}

// captureStdout redirects os.Stdout for the duration of fn and returns what was
// written. PrintJSON writes to os.Stdout directly, so this is the only way to
// assert its output.
func captureStdout(fn func() error) (string, error) {
	orig := os.Stdout
	r, wpipe, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = wpipe
	fnErr := fn()
	_ = wpipe.Close()
	os.Stdout = orig

	data, _ := io.ReadAll(r)
	_ = r.Close()
	return string(data), fnErr
}

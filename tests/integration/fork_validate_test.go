//go:build integration

package integration_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/testhelper"
)

// TestFork_RejectsInvalidNames wires worktree.ValidateWorktreeName into
// `grove fork`: every other creation entry point (new, open, TUI overlays)
// already enforced it, so `grove fork root` used to slip past the reserved
// name and collide with the main worktree's state/tmux keys.
func TestFork_RejectsInvalidNames(t *testing.T) {
	binary := buildGroveBinary(t)
	repo := testhelper.SetupRailsFixture(t)

	// "-dash" isn't listed: cobra's flag parsing rejects it before the
	// validator can (unknown shorthand flag) — refused either way.
	for _, name := range []string{"root", "bad/name", ".dot"} {
		t.Run(name, func(t *testing.T) {
			cmd := exec.Command(binary, "fork", name, "--no-switch")
			cmd.Dir = repo
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("grove fork %q succeeded, want rejection\noutput: %s", name, out)
			}
			if !strings.Contains(string(out), "invalid worktree name") &&
				!strings.Contains(string(out), "reserved") {
				t.Errorf("unexpected error output for %q: %s", name, out)
			}
		})
	}
}

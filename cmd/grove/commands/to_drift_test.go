package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/mux"
)

// driftMux reports a fixed pane and records what is sent to it.
type driftMux struct {
	mux.Multiplexer
	pane mux.PaneInfo
	sent []string
}

func (d *driftMux) PaneInfo(mux.Target) (*mux.PaneInfo, error) { return &d.pane, nil }

func (d *driftMux) SendCommand(_ mux.Target, command string) error {
	d.sent = append(d.sent, command)
	return nil
}

// A backend may report the canonical path of a worktree that grove reached
// through a symlink. That is the same directory, so it is not drift.
func TestHandleDirectoryDrift_SameDirectoryByAnotherSpellingIsNotDrift(t *testing.T) {
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	m := &driftMux{pane: mux.PaneInfo{CurrentPath: real, CurrentCommand: "zsh"}}

	handleDirectoryDrift(m, mux.Target{Name: "proj-feat"}, link, "reset", cli.NewStderr())

	if len(m.sent) != 0 {
		t.Errorf("sent %q to a pane that is already in the worktree", m.sent)
	}
}

func TestHandleDirectoryDrift_ResetsAPaneInAnotherDirectory(t *testing.T) {
	worktreePath := t.TempDir()
	m := &driftMux{pane: mux.PaneInfo{CurrentPath: t.TempDir(), CurrentCommand: "zsh"}}

	handleDirectoryDrift(m, mux.Target{Name: "proj-feat"}, worktreePath, "reset", cli.NewStderr())

	if len(m.sent) != 1 {
		t.Fatalf("sent %q, want one cd back to the worktree", m.sent)
	}
}

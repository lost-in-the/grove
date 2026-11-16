package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/worktree"
)

func TestCreateWorktreeCmd_RemoteBranch_UsesCreateFromBranch(t *testing.T) {
	// Build a minimal git repo so Manager.prepareWorktreePath can resolve paths,
	// then confirm the first streaming log line names the base branch.
	dir := t.TempDir()

	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "commit.gpgsign", "false"},
		{"config", "user.name", "Test"},
		{"config", "user.email", "t@t.com"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	// Create an initial commit so the repo is valid.
	f := filepath.Join(dir, "README")
	if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", "."},
		{"commit", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}

	mgr, err := worktree.NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	const baseBranch = "origin/feature-remote-only"
	cmd := createWorktreeCmd(mgr, nil, nil, dir, "remote-wt", baseBranch, "", "")
	msg := cmd()

	// The first message must be a creationLogMsg (log line before createFn runs)
	// or a creationDoneMsg (if it failed quickly). Either way the log line or
	// error context must reference the base branch, proving CreateFromBranch
	// was called rather than CreateFromExisting.
	switch m := msg.(type) {
	case creationLogMsg:
		if !strings.Contains(m.line, baseBranch) {
			t.Errorf("first log line = %q, want it to contain base branch %q", m.line, baseBranch)
		}
	case creationDoneMsg:
		// Creation failed (expected — no remote). Verify the log lines
		// that were sent named the base branch. We can infer this from
		// the source being "create" (not a silent skip).
		if m.source != "create" {
			t.Errorf("creationDoneMsg.source = %q, want %q", m.source, "create")
		}
		// The error should be about the branch fetch/create failing, not a nil
		// error that would indicate a silent no-op.
		if m.err == nil {
			t.Error("expected non-nil error from create with nonexistent remote branch")
		}
	default:
		t.Fatalf("unexpected message type %T: %v", msg, msg)
	}

	// streamingCreateCmd runs createFn (the actual `git worktree add`) in a
	// background goroutine and returns after the first log line. Drain the
	// remaining events until the done message so that goroutine's git writes
	// complete before t.TempDir() cleanup runs — otherwise an in-flight
	// worktree add races RemoveAll on .git and CI flakes with
	// "directory not empty".
	for {
		lm, ok := msg.(creationLogMsg)
		if !ok {
			break // creationDoneMsg — the creation goroutine has finished
		}
		msg = readCreationLog(lm.ch, "create")()
	}
}

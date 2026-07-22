//go:build integration

// End-to-end tests for the switch abort rollback added in the audit branch:
// the rollback must pop the exact auto-stash it created (not whatever sits at
// stash@{0}), a tmux client-switch failure must still roll back (the commit
// point sits after it), and a self-switch keeps the cd/session epilogue.
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

// setupSwitchRepo builds a repo with auto-stash switching, one extra worktree
// ("feat"), and the given hooks.toml content (empty = none). It returns the
// repo path and a run helper bound to the shared env.
func setupSwitchRepo(t *testing.T, binary, hooksToml string) (string, func(dir, name string, args ...string) []byte) {
	t.Helper()

	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(base, "proj")
	testhelper.Must(t, os.MkdirAll(repo, 0755))

	env := testhelper.GitEnv()
	run := func(dir, name string, args ...string) []byte {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out)
		}
		return out
	}

	run(repo, "git", "init", "-b", "main")
	testhelper.WriteFile(t, filepath.Join(repo, "a.txt"), "x\n")
	run(repo, "git", "add", "-A")
	run(repo, "git", "commit", "-m", "init")
	run(repo, binary, "init")

	testhelper.WriteFile(t, filepath.Join(repo, ".grove", "config.toml"),
		"project_name = \"proj\"\n\n[switch]\ndirty_handling = \"auto-stash\"\n")
	if hooksToml != "" {
		testhelper.WriteFile(t, filepath.Join(repo, ".grove", "hooks.toml"), hooksToml)
	}
	run(repo, "git", "add", ".grove")
	run(repo, "git", "commit", "-m", "grove files")

	run(repo, binary, "new", "feat", "--no-tmux")
	return repo, run
}

// TestTo_AbortRollbackPopsOwnStash: a failing required post_switch hook that
// pushes its *own* stash on top of grove's auto-stash must not confuse the
// rollback — before the SHA-targeted pop, the rollback blindly popped
// stash@{0}, restoring the hook's stash and stranding the user's changes
// while printing "Restored auto-stashed changes".
func TestTo_AbortRollbackPopsOwnStash(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	binary := buildGroveBinary(t)

	// The hook dirties the tree and stashes it (a new stash on top of the
	// auto-stash), then fails as a required action.
	hooks := "[hooks]\n[[hooks.post_switch]]\ntype = \"command\"\n" +
		"command = \"echo hook > hook.txt && git stash push --include-untracked -m 'hook stash' && exit 1\"\n" +
		"on_failure = \"fail\"\nworking_dir = \"main\"\n"
	repo, run := setupSwitchRepo(t, binary, hooks)

	testhelper.WriteFile(t, filepath.Join(repo, "a.txt"), "modified\n")

	cmd := exec.Command(binary, "to", "feat", "--no-tmux")
	cmd.Dir = repo
	cmd.Env = testhelper.GitEnv()
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("grove to succeeded despite required post_switch failure\n%s", out)
	}

	// The user's change is back in the working tree...
	data, readErr := os.ReadFile(filepath.Join(repo, "a.txt"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != "modified\n" {
		t.Errorf("user's change not restored (wrong stash popped?): a.txt = %q\noutput:\n%s", data, out)
	}
	// ...the hook's own stash is left alone, still in the list.
	stashes := string(run(repo, "git", "stash", "list"))
	if !strings.Contains(stashes, "hook stash") {
		t.Errorf("hook's stash was popped by the rollback:\n%s\noutput:\n%s", stashes, out)
	}
	if strings.Contains(stashes, "grove: auto-stash") {
		t.Errorf("grove's auto-stash still parked after rollback:\n%s\noutput:\n%s", stashes, out)
	}
}

// TestTo_TmuxSwitchFailureRollsBack: a `tmux switch-client` failure happens at
// the very end of the flow, but the switch must still abort cleanly — stash
// restored, no last_worktree recorded. Before the fix the commit point sat
// before the client switch, so the user stayed in the old session with their
// changes swallowed into the stash.
func TestTo_TmuxSwitchFailureRollsBack(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	binary := buildGroveBinary(t)
	repo, run := setupSwitchRepo(t, binary, "")

	// A tmux shim that reports sessions as existing but fails switch-client.
	shimDir := t.TempDir()
	shim := "#!/bin/sh\n" +
		"case \"$1\" in\n" +
		"  switch-client) exit 1 ;;\n" +
		"  has-session) exit 0 ;;\n" +
		"  display-message) echo proj; exit 0 ;;\n" +
		"  *) exit 0 ;;\n" +
		"esac\n"
	testhelper.WriteFile(t, filepath.Join(shimDir, "tmux"), shim)
	testhelper.Must(t, os.Chmod(filepath.Join(shimDir, "tmux"), 0755))

	testhelper.WriteFile(t, filepath.Join(repo, "a.txt"), "modified\n")

	// Snapshot the state the aborted switch must not disturb (`grove new`
	// already recorded a last_worktree during setup).
	readState := func() (string, string) {
		t.Helper()
		stateData, readErr := os.ReadFile(filepath.Join(repo, ".grove", "state.json"))
		if readErr != nil {
			t.Fatal(readErr)
		}
		var st struct {
			LastWorktree string `json:"last_worktree"`
			Worktrees    map[string]struct {
				LastAccessedAt string `json:"last_accessed_at"`
			} `json:"worktrees"`
		}
		testhelper.Must(t, json.Unmarshal(stateData, &st))
		return st.LastWorktree, st.Worktrees["feat"].LastAccessedAt
	}
	lastBefore, featTouchedBefore := readState()

	cmd := exec.Command(binary, "to", "feat")
	cmd.Dir = repo
	cmd.Env = append(testhelper.GitEnv(),
		"PATH="+shimDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"TMUX=/tmp/tmux-shim/default,1,0", // pretend we're inside tmux
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("grove to succeeded despite switch-client failure\n%s", out)
	}

	data, readErr := os.ReadFile(filepath.Join(repo, "a.txt"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != "modified\n" {
		t.Errorf("auto-stash not restored after tmux switch failure: a.txt = %q\noutput:\n%s", data, out)
	}
	stashes := run(repo, "git", "stash", "list")
	if len(strings.TrimSpace(string(stashes))) != 0 {
		t.Errorf("auto-stash still parked after tmux switch failure:\n%s", stashes)
	}

	// The aborted switch must not have touched state: last_worktree and the
	// target's access time stay exactly as they were.
	lastAfter, featTouchedAfter := readState()
	if lastAfter != lastBefore {
		t.Errorf("aborted switch changed last_worktree: %q -> %q", lastBefore, lastAfter)
	}
	if featTouchedAfter != featTouchedBefore {
		t.Errorf("aborted switch touched the target's last_accessed_at: %q -> %q", featTouchedBefore, featTouchedAfter)
	}
}

// TestTo_SelfSwitchEmitsCdFromSubdir: `grove to <current>` from a subdirectory
// must still emit the cd: directive back to the worktree root (the pre-B18
// behavior the short-circuit initially dropped).
func TestTo_SelfSwitchEmitsCdFromSubdir(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	binary := buildGroveBinary(t)
	repo, run := setupSwitchRepo(t, binary, "")

	sub := filepath.Join(repo, "sub")
	testhelper.Must(t, os.MkdirAll(sub, 0755))
	_ = run // repo already set up

	cmd := exec.Command(binary, "to", "root", "--no-tmux")
	cmd.Dir = sub
	cmd.Env = append(testhelper.GitEnv(), "GROVE_SHELL=1", "GROVE_SHELL_VERSION=9")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("self-switch failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "cd:"+repo) {
		t.Errorf("self-switch from subdirectory did not emit cd back to the root:\n%s", out)
	}
}

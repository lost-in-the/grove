//go:build integration

// End-to-end test for B6: config-file (hooks.toml) pre_switch and post_switch
// actions must actually run during `grove to`. Before the fix, only plugin
// hooks fired on switch and these documented events silently did nothing.
package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/testhelper"
)

func TestTo_FiresSwitchConfigHooks(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	binary := buildGroveBinary(t)

	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(base, "proj")
	testhelper.Must(t, os.MkdirAll(repo, 0755))

	env := testhelper.GitEnv()
	run := func(dir, name string, args ...string) {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out)
		}
	}

	run(repo, "git", "init", "-b", "main")
	testhelper.WriteFile(t, filepath.Join(repo, "a.txt"), "x\n")
	run(repo, "git", "add", "-A")
	run(repo, "git", "commit", "-m", "init")

	// `grove init` records the machine-local .grove artifacts in the shared
	// exclude (B4), so the tree stays clean through new/switch. The
	// project-level files (config.toml, hooks.toml) are deliberately NOT
	// excluded — they're meant to be committed, so commit them like a user
	// following the README would.
	run(repo, binary, "init")

	hooks := `[hooks]
[[hooks.pre_switch]]
type = "command"
command = "touch pre_switch_ran.txt"
working_dir = "new"
[[hooks.post_switch]]
type = "command"
command = "touch post_switch_ran.txt"
working_dir = "new"
`
	testhelper.WriteFile(t, filepath.Join(repo, ".grove", "hooks.toml"), hooks)
	run(repo, "git", "add", ".grove")
	run(repo, "git", "commit", "-m", "grove project files")

	run(repo, binary, "new", "feat", "--no-tmux")
	run(repo, binary, "to", "feat", "--no-tmux")

	wt := filepath.Join(base, "proj-feat")
	for _, marker := range []string{"pre_switch_ran.txt", "post_switch_ran.txt"} {
		if _, err := os.Stat(filepath.Join(wt, marker)); err != nil {
			t.Errorf("switch hook did not run: %s missing (%v)", marker, err)
		}
	}
}

// TestTo_AbortedSwitchRestoresAutoStash: when dirty_handling="auto-stash" and
// a required pre_switch hook aborts the switch, the just-made stash must be
// popped back — otherwise `grove to` silently swallows the user's uncommitted
// work into a stash they never asked for, on a switch that never happened.
// last_worktree must also stay untouched so the A↔B toggle isn't corrupted
// by the aborted attempt.
func TestTo_AbortedSwitchRestoresAutoStash(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	binary := buildGroveBinary(t)

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

	// auto-stash on switch + a required pre_switch hook that always fails.
	testhelper.WriteFile(t, filepath.Join(repo, ".grove", "config.toml"),
		"project_name = \"proj\"\n\n[switch]\ndirty_handling = \"auto-stash\"\n")
	testhelper.WriteFile(t, filepath.Join(repo, ".grove", "hooks.toml"),
		"[hooks]\n[[hooks.pre_switch]]\ntype = \"command\"\ncommand = \"exit 1\"\non_failure = \"fail\"\nworking_dir = \"main\"\n")
	run(repo, "git", "add", ".grove")
	run(repo, "git", "commit", "-m", "grove files")

	run(repo, binary, "new", "feat", "--no-tmux")

	// Dirty the main worktree, then attempt the switch that must abort.
	testhelper.WriteFile(t, filepath.Join(repo, "a.txt"), "modified\n")

	cmd := exec.Command(binary, "to", "feat", "--no-tmux")
	cmd.Dir = repo
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("grove to succeeded despite required pre_switch failure\n%s", out)
	}

	// The uncommitted change must be back in the working tree...
	data, readErr := os.ReadFile(filepath.Join(repo, "a.txt"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != "modified\n" {
		t.Errorf("uncommitted change lost after aborted switch: a.txt = %q\noutput:\n%s", data, out)
	}
	// ...and not parked in a stash the user never asked to keep.
	stashes := run(repo, "git", "stash", "list")
	if len(strings.TrimSpace(string(stashes))) != 0 {
		t.Errorf("auto-stash not restored after aborted switch:\n%s", stashes)
	}
}

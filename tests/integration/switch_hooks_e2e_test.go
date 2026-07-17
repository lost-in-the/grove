//go:build integration

// End-to-end test for B6: config-file (hooks.toml) pre_switch and post_switch
// actions must actually run during `grove to`. Before the fix, only plugin
// hooks fired on switch and these documented events silently did nothing.
package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
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
	// exclude (B4), so the tree stays clean through new/switch.
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
	run(repo, "git", "add", ".grove/hooks.toml")
	run(repo, "git", "commit", "-m", "hooks")

	run(repo, binary, "new", "feat", "--no-tmux")
	run(repo, binary, "to", "feat", "--no-tmux")

	wt := filepath.Join(base, "proj-feat")
	for _, marker := range []string{"pre_switch_ran.txt", "post_switch_ran.txt"} {
		if _, err := os.Stat(filepath.Join(wt, marker)); err != nil {
			t.Errorf("switch hook did not run: %s missing (%v)", marker, err)
		}
	}
}

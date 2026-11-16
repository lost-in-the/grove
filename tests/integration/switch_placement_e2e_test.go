//go:build integration

// End-to-end tests for placement (--here) and side effects (--no-hooks) on
// `grove to`. Every run uses [mux] backend = "off" or --here, so no real
// session is created.
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

const placementHooks = `[hooks]
[[hooks.pre_switch]]
type = "command"
command = "touch pre_switch_ran.txt"
working_dir = "new"
[[hooks.post_switch]]
type = "command"
command = "touch post_switch_ran.txt"
working_dir = "new"
`

// setupPlacementRepo creates a grove project with switch hooks, the given
// extra config, and a "feat" worktree. It returns the main checkout and the
// worktree.
func setupPlacementRepo(t *testing.T, binary, extraConfig string) (repo, wt string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo = filepath.Join(base, "proj")
	testhelper.Must(t, os.MkdirAll(repo, 0755))

	run := func(name string, args ...string) {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = repo
		cmd.Env = testhelper.GitEnv()
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out)
		}
	}

	run("git", "init", "-b", "main")
	testhelper.WriteFile(t, filepath.Join(repo, "a.txt"), "x\n")
	run("git", "add", "-A")
	run("git", "commit", "-m", "init")
	run(binary, "init")
	testhelper.WriteFile(t, filepath.Join(repo, ".grove", "config.toml"),
		"project_name = \"proj\"\n\n[mux]\nbackend = \"off\"\n"+extraConfig)
	testhelper.WriteFile(t, filepath.Join(repo, ".grove", "hooks.toml"), placementHooks)
	run("git", "add", ".grove")
	run("git", "commit", "-m", "grove project files")
	run(binary, "new", "feat", "--here", "--no-switch")

	return repo, filepath.Join(base, "proj-feat")
}

func groveIn(dir, binary string, args ...string) (stdout, stderr string, err error) {
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	cmd.Env = testhelper.GitEnv()
	var out, errOut strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err = cmd.Run()
	return out.String(), errOut.String(), err
}

func switchHooksRan(wt string) bool {
	_, preErr := os.Stat(filepath.Join(wt, "pre_switch_ran.txt"))
	_, postErr := os.Stat(filepath.Join(wt, "post_switch_ran.txt"))
	return preErr == nil || postErr == nil
}

func TestTo_HereStillRunsHooks(t *testing.T) {
	binary := buildGroveBinary(t)
	repo, wt := setupPlacementRepo(t, binary, "")

	if _, stderr, err := groveIn(repo, binary, "to", "feat", "--here"); err != nil {
		t.Fatalf("grove to --here failed: %v\n%s", err, stderr)
	}
	if !switchHooksRan(wt) {
		t.Error("--here must not skip hooks: no switch hook marker was written")
	}
}

func TestTo_NoHooksSkipsHooks(t *testing.T) {
	for _, flags := range [][]string{{"--no-hooks"}, {"--here", "--no-hooks"}, {"--peek"}} {
		t.Run(strings.Join(flags, " "), func(t *testing.T) {
			binary := buildGroveBinary(t)
			repo, wt := setupPlacementRepo(t, binary, "")

			stdout, stderr, err := groveIn(repo, binary, append([]string{"to", "feat", "--json"}, flags...)...)
			if err != nil {
				t.Fatalf("grove to %v failed: %v\n%s", flags, err, stderr)
			}
			if switchHooksRan(wt) {
				t.Errorf("grove to %v ran a switch hook", flags)
			}
			var result struct {
				SwitchTo string `json:"switch_to"`
			}
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
			}
			if result.SwitchTo != wt {
				t.Errorf("switch_to = %q, want %q", result.SwitchTo, wt)
			}
		})
	}
}

// --peek is short for --here --no-hooks, so both must pass a dirty gate that
// stops --here on its own.
func TestTo_NoHooksSkipsTheDirtyCheck(t *testing.T) {
	binary := buildGroveBinary(t)
	repo, _ := setupPlacementRepo(t, binary, "\n[switch]\ndirty_handling = \"refuse\"\n")
	testhelper.WriteFile(t, filepath.Join(repo, "a.txt"), "modified\n")

	if _, _, err := groveIn(repo, binary, "to", "feat", "--here"); err == nil {
		t.Fatal("grove to --here moved away from a dirty worktree with dirty_handling = refuse")
	}
	for _, flags := range [][]string{{"--here", "--no-hooks"}, {"--peek"}} {
		if _, stderr, err := groveIn(repo, binary, append([]string{"to", "feat"}, flags...)...); err != nil {
			t.Errorf("grove to %v was stopped by the dirty check: %v\n%s", flags, err, stderr)
		}
	}
}

func TestTo_UpConflictsWithNoHooks(t *testing.T) {
	binary := buildGroveBinary(t)
	repo, wt := setupPlacementRepo(t, binary, "")

	for _, flag := range []string{"--no-hooks", "--peek"} {
		_, stderr, err := groveIn(repo, binary, "to", "feat", "--here", "--up", flag)
		if err == nil {
			t.Fatalf("grove to --up %s succeeded, want a flag conflict", flag)
		}
		if !strings.Contains(stderr, "up") || !strings.Contains(stderr, strings.TrimPrefix(flag, "--")) {
			t.Errorf("conflict error should name both flags, got:\n%s", stderr)
		}
	}
	if switchHooksRan(wt) {
		t.Error("a rejected run still fired a switch hook")
	}
}

func TestTo_NoTmuxIsAnAliasOfHere(t *testing.T) {
	binary := buildGroveBinary(t)
	repo, wt := setupPlacementRepo(t, binary, "")

	stdout, stderr, err := groveIn(repo, binary, "to", "feat", "--no-tmux", "--json")
	if err != nil {
		t.Fatalf("grove to --no-tmux failed: %v\n%s", err, stderr)
	}
	if !strings.Contains(stderr, "--no-tmux has been deprecated, use --here") {
		t.Errorf("stderr should carry the deprecation notice, got:\n%s", stderr)
	}
	if !json.Valid([]byte(stdout)) {
		t.Errorf("the deprecation notice must stay off stdout, got:\n%s", stdout)
	}
	if !switchHooksRan(wt) {
		t.Error("--no-tmux must still run hooks, as --here does")
	}
}

func TestOpenIn_OldSpellingsStillLoad(t *testing.T) {
	binary := buildGroveBinary(t)
	repo, _ := setupPlacementRepo(t, binary, "\n[session]\nopen_in = \"current\"\n")

	if _, stderr, err := groveIn(repo, binary, "to", "feat"); err != nil {
		t.Fatalf("open_in = \"current\" stopped loading: %v\n%s", err, stderr)
	}
}

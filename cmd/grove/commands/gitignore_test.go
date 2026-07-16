package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestEnsureGroveIgnored guards B4: grove's machine-local .grove artifacts must
// be recorded in the shared git exclude (applies to all worktrees, never
// committed), so grove-created worktrees are not born dirty.
func TestEnsureGroveIgnored(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
	run("git", "init")

	if err := ensureGroveIgnored(dir); err != nil {
		t.Fatalf("ensureGroveIgnored() error = %v", err)
	}

	excludePath := filepath.Join(dir, ".git", "info", "exclude")
	data, err := os.ReadFile(excludePath)
	if err != nil {
		t.Fatalf("read exclude: %v", err)
	}
	for _, want := range groveIgnoreEntries {
		if !strings.Contains(string(data), want) {
			t.Errorf("exclude file missing %q\ngot:\n%s", want, data)
		}
	}

	// A real .grove with the machine-local files present must produce a clean
	// working tree — the whole point of the fix.
	groveDir := filepath.Join(dir, ".grove")
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"state.json", "state.lock", "config.toml"} {
		if err := os.WriteFile(filepath.Join(groveDir, f), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	out, err := exec.Command("git", "-C", dir, "status", "--porcelain").CombinedOutput()
	if err != nil {
		t.Fatalf("git status: %v\n%s", err, out)
	}
	if strings.Contains(string(out), ".grove") {
		t.Errorf(".grove artifacts still show as dirty after ensureGroveIgnored:\n%s", out)
	}

	// Idempotent: a second call must not duplicate the block.
	before, _ := os.ReadFile(excludePath)
	if err := ensureGroveIgnored(dir); err != nil {
		t.Fatalf("second ensureGroveIgnored() error = %v", err)
	}
	after, _ := os.ReadFile(excludePath)
	if string(before) != string(after) {
		t.Errorf("ensureGroveIgnored not idempotent:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

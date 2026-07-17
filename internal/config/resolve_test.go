package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitConfigFixture builds a real repo with a committed file, a .grove project
// config in the main worktree, and a linked worktree. Returns (main, linked).
func gitConfigFixture(t *testing.T) (string, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	base, _ := filepath.EvalSymlinks(t.TempDir())
	mainDir := filepath.Join(base, "main")
	if err := os.MkdirAll(mainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL=/dev/null",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run(mainDir, "init")
	if err := os.WriteFile(filepath.Join(mainDir, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(mainDir, "add", "f")
	run(mainDir, "commit", "-m", "init")

	groveDir := filepath.Join(mainDir, ".grove")
	if err := os.MkdirAll(groveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(groveDir, "config.toml"),
		[]byte("project_name = \"mainproj\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	linkedDir := filepath.Join(base, "linked")
	run(mainDir, "worktree", "add", linkedDir, "-b", "linked")
	return mainDir, linkedDir
}

// chdir switches cwd for the test and restores it on cleanup.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

// TestLoad_ResolvesToMainWorktree guards the read half of config resolution:
// no matter where in the project a command runs, Load() must read the MAIN
// worktree's .grove/config.toml — a linked worktree or subdirectory must
// never load defaults (or a stale local copy) instead.
func TestLoad_ResolvesToMainWorktree(t *testing.T) {
	mainDir, linkedDir := gitConfigFixture(t)
	subDir := filepath.Join(mainDir, "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	for name, dir := range map[string]string{
		"main root":   mainDir,
		"main subdir": subDir,
		"linked":      linkedDir,
	} {
		t.Run(name, func(t *testing.T) {
			chdir(t, dir)
			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.ProjectName != "mainproj" {
				t.Errorf("Load() from %s: ProjectName = %q, want %q (main worktree's config not resolved)",
					name, cfg.ProjectName, "mainproj")
			}
		})
	}
}

// TestSetProjectConfigValues_WritesMainWorktree guards the write half: from a
// linked worktree, `grove config set` (and the TUI settings editor) must
// update the MAIN worktree's config.toml — never materialize a private copy
// in the linked worktree (the old cwd-based path did exactly that, silently
// forking the project config).
func TestSetProjectConfigValues_WritesMainWorktree(t *testing.T) {
	mainDir, linkedDir := gitConfigFixture(t)
	chdir(t, linkedDir)

	if err := SetProjectConfigValues(map[string]string{"default_base_branch": "develop"}); err != nil {
		t.Fatalf("SetProjectConfigValues() error = %v", err)
	}

	mainConfig, err := os.ReadFile(filepath.Join(mainDir, ".grove", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mainConfig), "develop") {
		t.Errorf("main worktree config not updated:\n%s", mainConfig)
	}
	if !strings.Contains(string(mainConfig), "mainproj") {
		t.Errorf("main worktree config lost existing content:\n%s", mainConfig)
	}

	if _, err := os.Stat(filepath.Join(linkedDir, ".grove", "config.toml")); !os.IsNotExist(err) {
		t.Error("a private config.toml was created in the linked worktree — write did not anchor at main")
	}
}

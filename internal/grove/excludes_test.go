package grove

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitRun runs a git command in dir, failing the test on error.
func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// gitRepoWithWorktree creates a repo with one commit and a linked worktree,
// returning (mainDir, linkedDir).
func gitRepoWithWorktree(t *testing.T) (string, string) {
	t.Helper()
	base, _ := filepath.EvalSymlinks(t.TempDir())
	mainDir := filepath.Join(base, "main")
	if err := os.MkdirAll(mainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	gitInit(t, mainDir)
	if err := os.WriteFile(filepath.Join(mainDir, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, mainDir, "add", "f")
	gitRun(t, mainDir, "commit", "-m", "init")
	linkedDir := filepath.Join(base, "linked")
	gitRun(t, mainDir, "worktree", "add", linkedDir, "-b", "linked")
	return mainDir, linkedDir
}

// isIgnored reports whether git considers relPath ignored in dir.
func isIgnored(t *testing.T, dir, relPath string) bool {
	t.Helper()
	cmd := exec.Command("git", "check-ignore", "-q", relPath)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
	err := cmd.Run()
	if err == nil {
		return true
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false
	}
	t.Fatalf("git check-ignore %s failed: %v", relPath, err)
	return false
}

func TestGitCommonDir(t *testing.T) {
	mainDir, linkedDir := gitRepoWithWorktree(t)
	want := filepath.Join(mainDir, ".git")

	subDir := filepath.Join(mainDir, "sub", "deep")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	linkedSub := filepath.Join(linkedDir, "wsub")
	if err := os.MkdirAll(linkedSub, 0o755); err != nil {
		t.Fatal(err)
	}

	for name, dir := range map[string]string{
		"main root":     mainDir,
		"main subdir":   subDir,
		"linked root":   linkedDir,
		"linked subdir": linkedSub,
	} {
		t.Run(name, func(t *testing.T) {
			got, err := GitCommonDir(dir)
			if err != nil {
				t.Fatalf("GitCommonDir(%s) error = %v", dir, err)
			}
			if got != want {
				t.Errorf("GitCommonDir(%s) = %q, want %q", dir, got, want)
			}
		})
	}

	t.Run("non-git dir errors", func(t *testing.T) {
		if _, err := GitCommonDir(t.TempDir()); err == nil {
			t.Error("GitCommonDir on a non-repo should error")
		}
	})
}

func TestEnsureGroveExcludes(t *testing.T) {
	t.Run("machine-local files ignored, config.toml stays committable", func(t *testing.T) {
		mainDir, _ := gitRepoWithWorktree(t)
		migrated, err := EnsureGroveExcludes(mainDir)
		if err != nil {
			t.Fatalf("EnsureGroveExcludes() error = %v", err)
		}
		// A routine first-time write is not a migration — the upgrade notice
		// must not fire for fresh repos.
		if migrated {
			t.Error("migrated = true on a fresh write, want false")
		}

		groveDir := filepath.Join(mainDir, ".grove")
		if err := os.MkdirAll(groveDir, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, f := range []string{"state.json", "state.json.bak", "state.lock", "ui_prefs.json", ".envrc", "config.local.toml", "config.toml"} {
			if err := os.WriteFile(filepath.Join(groveDir, f), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}

		for _, f := range []string{"state.json", "state.json.bak", "state.lock", "ui_prefs.json", ".envrc", "config.local.toml"} {
			if !isIgnored(t, mainDir, ".grove/"+f) {
				t.Errorf(".grove/%s should be ignored", f)
			}
		}
		// The project config is meant to be committed (README: "commit this to
		// your repo") — it must never be excluded.
		if isIgnored(t, mainDir, ".grove/config.toml") {
			t.Fatal(".grove/config.toml must NOT be ignored — it is the shared project config")
		}
		// And `git add` must accept it without -f.
		gitRun(t, mainDir, "add", ".grove/config.toml")
	})

	t.Run("idempotent", func(t *testing.T) {
		mainDir, _ := gitRepoWithWorktree(t)
		if _, err := EnsureGroveExcludes(mainDir); err != nil {
			t.Fatal(err)
		}
		excludePath := filepath.Join(mainDir, ".git", "info", "exclude")
		before, err := os.ReadFile(excludePath)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := EnsureGroveExcludes(mainDir); err != nil {
			t.Fatal(err)
		}
		after, err := os.ReadFile(excludePath)
		if err != nil {
			t.Fatal(err)
		}
		if string(before) != string(after) {
			t.Errorf("not idempotent:\nbefore:\n%s\nafter:\n%s", before, after)
		}
	})

	t.Run("migrates legacy block that excluded config.toml", func(t *testing.T) {
		mainDir, _ := gitRepoWithWorktree(t)
		excludePath := filepath.Join(mainDir, ".git", "info", "exclude")
		if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
			t.Fatal(err)
		}
		// Layout an exclude file as an older grove wrote it: user content,
		// then the managed block including the config.toml entry.
		legacy := "userfile.txt\n\n" + groveExcludeHeader + "\n" +
			".grove/state.json\n.grove/state.json.bak\n.grove/state.lock\n" +
			".grove/ui_prefs.json\n.grove/.envrc\n.grove/config.local.toml\n" +
			".grove/config.toml\n"
		if err := os.WriteFile(excludePath, []byte(legacy), 0o644); err != nil {
			t.Fatal(err)
		}

		migrated, err := EnsureGroveExcludes(mainDir)
		if err != nil {
			t.Fatalf("EnsureGroveExcludes() error = %v", err)
		}
		// Removing the legacy entry IS the migration — the one-time upgrade
		// notice keys off this flag.
		if !migrated {
			t.Error("migrated = false when a legacy config.toml entry was removed, want true")
		}

		content, err := os.ReadFile(excludePath)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(content), ".grove/config.toml") {
			t.Errorf("legacy .grove/config.toml entry survived migration:\n%s", content)
		}
		if !strings.Contains(string(content), ".grove/state.json") {
			t.Errorf("state.json entry lost in migration:\n%s", content)
		}
		if !strings.Contains(string(content), "userfile.txt") {
			t.Errorf("user content lost in migration:\n%s", content)
		}
		// The second run is a no-op: the notice can never repeat.
		migrated, err = EnsureGroveExcludes(mainDir)
		if err != nil {
			t.Fatal(err)
		}
		if migrated {
			t.Error("migrated = true on the idempotent re-run, want false")
		}
	})

	t.Run("user entry outside the managed block is preserved", func(t *testing.T) {
		mainDir, _ := gitRepoWithWorktree(t)
		excludePath := filepath.Join(mainDir, ".git", "info", "exclude")
		if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
			t.Fatal(err)
		}
		// The user deliberately ignores their own config.toml OUTSIDE our
		// block — grove must not touch entries it doesn't own.
		if err := os.WriteFile(excludePath, []byte("# mine\n.grove/config.toml\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := EnsureGroveExcludes(mainDir); err != nil {
			t.Fatal(err)
		}
		content, _ := os.ReadFile(excludePath)
		if !strings.Contains(string(content), "# mine\n.grove/config.toml") {
			t.Errorf("user's own entry was removed:\n%s", content)
		}
		if !strings.Contains(string(content), ".grove/state.json") {
			t.Errorf("managed block missing:\n%s", content)
		}
	})

	t.Run("works from a linked worktree", func(t *testing.T) {
		mainDir, linkedDir := gitRepoWithWorktree(t)
		if _, err := EnsureGroveExcludes(linkedDir); err != nil {
			t.Fatalf("EnsureGroveExcludes(linked) error = %v", err)
		}
		content, err := os.ReadFile(filepath.Join(mainDir, ".git", "info", "exclude"))
		if err != nil {
			t.Fatalf("exclude not written to common dir: %v", err)
		}
		if !strings.Contains(string(content), ".grove/state.json") {
			t.Errorf("managed block missing from common exclude:\n%s", content)
		}
	})
}

func TestNeedsConfigMigrationNotice_SilentWithoutLegacySymlinks(t *testing.T) {
	mainDir, _ := gitRepoWithWorktree(t)
	groveDir := filepath.Join(mainDir, ".grove")
	if err := os.MkdirAll(groveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if NeedsConfigMigrationNotice(groveDir, mainDir) {
		t.Error("notice fired with no legacy config symlinks present")
	}
	if _, err := os.Stat(filepath.Join(groveDir, configMigrationSentinel)); err != nil {
		t.Errorf("sentinel not written after first check: %v", err)
	}
}

func TestNeedsConfigMigrationNotice_FiresOnceForLegacySymlink(t *testing.T) {
	mainDir, linkedDir := gitRepoWithWorktree(t)
	groveDir := filepath.Join(mainDir, ".grove")
	if err := os.MkdirAll(groveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(groveDir, "config.toml"), []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Plant a legacy per-worktree config.toml symlink in the linked worktree,
	// the artifact a pre-0.10 repo carries.
	linkedGrove := filepath.Join(linkedDir, ".grove")
	if err := os.MkdirAll(linkedGrove, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(groveDir, "config.toml"), filepath.Join(linkedGrove, "config.toml")); err != nil {
		t.Fatal(err)
	}

	if !NeedsConfigMigrationNotice(groveDir, mainDir) {
		t.Fatal("expected the notice to fire for a legacy config symlink")
	}
	if NeedsConfigMigrationNotice(groveDir, mainDir) {
		t.Error("notice fired a second time; the sentinel should make it one-shot")
	}
}

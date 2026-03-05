package grove

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestFindRoot(t *testing.T) {
	t.Run("finds .grove in current directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		groveDir := filepath.Join(tmpDir, ".grove")
		if err := os.MkdirAll(groveDir, 0755); err != nil {
			t.Fatalf("failed to create .grove dir: %v", err)
		}

		found, err := FindRoot(tmpDir)
		if err != nil {
			t.Fatalf("FindRoot() error = %v", err)
		}
		if found != groveDir {
			t.Errorf("FindRoot() = %q, want %q", found, groveDir)
		}
	})

	t.Run("finds .grove in parent directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		groveDir := filepath.Join(tmpDir, ".grove")
		subDir := filepath.Join(tmpDir, "sub", "nested")

		if err := os.MkdirAll(groveDir, 0755); err != nil {
			t.Fatalf("failed to create .grove dir: %v", err)
		}
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatalf("failed to create sub dir: %v", err)
		}

		found, err := FindRoot(subDir)
		if err != nil {
			t.Fatalf("FindRoot() error = %v", err)
		}
		if found != groveDir {
			t.Errorf("FindRoot() = %q, want %q", found, groveDir)
		}
	})

	t.Run("returns empty when not found", func(t *testing.T) {
		tmpDir := t.TempDir()

		found, err := FindRoot(tmpDir)
		if err != nil {
			t.Fatalf("FindRoot() error = %v", err)
		}
		if found != "" {
			t.Errorf("FindRoot() = %q, want empty", found)
		}
	})
}

func TestFindRoot_FromWorktree(t *testing.T) {
	// Create a real git repo with a worktree, verify FindRoot works from the worktree
	mainDir := t.TempDir()
	mainDir, _ = filepath.EvalSymlinks(mainDir)

	// Init git repo with an initial commit
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}

	run(mainDir, "git", "init")
	run(mainDir, "git", "commit", "--allow-empty", "-m", "init")

	// Create .grove in main worktree
	groveDir := filepath.Join(mainDir, ".grove")
	_ = os.MkdirAll(groveDir, 0755)

	// Create a sibling worktree
	wtDir := mainDir + "-wt"
	run(mainDir, "git", "worktree", "add", wtDir, "-b", "test-branch")
	defer func() { _ = os.RemoveAll(wtDir) }()

	// FindRoot from the worktree should find main's .grove
	found, err := FindRoot(wtDir)
	if err != nil {
		t.Fatalf("FindRoot() error = %v", err)
	}
	if found != groveDir {
		t.Errorf("FindRoot() = %q, want %q", found, groveDir)
	}
}

func TestFindRoot_NoGroveDir(t *testing.T) {
	// A git repo with no .grove should return empty
	mainDir := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = mainDir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}

	found, err := FindRoot(mainDir)
	if err != nil {
		t.Fatalf("FindRoot() error = %v", err)
	}
	if found != "" {
		t.Errorf("FindRoot() = %q, want empty", found)
	}
}

func TestIsInsideWorktree(t *testing.T) {
	t.Run("false when .git is directory (main repo)", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitDir := filepath.Join(tmpDir, ".git")
		if err := os.MkdirAll(gitDir, 0755); err != nil {
			t.Fatalf("failed to create .git dir: %v", err)
		}

		// Change to the temp dir
		oldWd, _ := os.Getwd()
		defer func() { _ = os.Chdir(oldWd) }()
		_ = os.Chdir(tmpDir)

		isWT, err := IsInsideWorktree()
		if err != nil {
			t.Fatalf("IsInsideWorktree() error = %v", err)
		}
		if isWT {
			t.Error("IsInsideWorktree() = true, want false for main repo")
		}
	})

	t.Run("true when .git is file (worktree)", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitFile := filepath.Join(tmpDir, ".git")
		// Worktrees have .git as a file pointing to the main repo
		if err := os.WriteFile(gitFile, []byte("gitdir: /some/path/.git/worktrees/foo"), 0644); err != nil {
			t.Fatalf("failed to create .git file: %v", err)
		}

		oldWd, _ := os.Getwd()
		defer func() { _ = os.Chdir(oldWd) }()
		_ = os.Chdir(tmpDir)

		isWT, err := IsInsideWorktree()
		if err != nil {
			t.Fatalf("IsInsideWorktree() error = %v", err)
		}
		if !isWT {
			t.Error("IsInsideWorktree() = false, want true for worktree")
		}
	})
}

func TestEnsureConfigSymlink(t *testing.T) {
	t.Run("creates symlink when main has config", func(t *testing.T) {
		mainDir := t.TempDir()
		newDir := t.TempDir()

		// Create main's config.toml
		mainGrove := filepath.Join(mainDir, ".grove")
		_ = os.MkdirAll(mainGrove, 0755)
		configContent := []byte("[plugins.docker]\nenabled = true\n")
		_ = os.WriteFile(filepath.Join(mainGrove, "config.toml"), configContent, 0644)

		err := EnsureConfigSymlink(mainDir, newDir)
		if err != nil {
			t.Fatalf("EnsureConfigSymlink() error = %v", err)
		}

		dst := filepath.Join(newDir, ".grove", "config.toml")
		info, err := os.Lstat(dst)
		if err != nil {
			t.Fatalf("symlink not created: %v", err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Error("expected symlink, got regular file")
		}

		// Verify readable and correct content
		data, err := os.ReadFile(dst)
		if err != nil {
			t.Fatalf("failed to read symlink: %v", err)
		}
		if string(data) != string(configContent) {
			t.Errorf("content = %q, want %q", data, configContent)
		}
	})

	t.Run("no-op when main has no config", func(t *testing.T) {
		mainDir := t.TempDir()
		newDir := t.TempDir()

		err := EnsureConfigSymlink(mainDir, newDir)
		if err != nil {
			t.Fatalf("EnsureConfigSymlink() error = %v", err)
		}

		dst := filepath.Join(newDir, ".grove", "config.toml")
		if _, err := os.Stat(dst); !os.IsNotExist(err) {
			t.Error("expected no file created when main has no config")
		}
	})

	t.Run("does not overwrite existing config", func(t *testing.T) {
		mainDir := t.TempDir()
		newDir := t.TempDir()

		// Create main config
		mainGrove := filepath.Join(mainDir, ".grove")
		_ = os.MkdirAll(mainGrove, 0755)
		_ = os.WriteFile(filepath.Join(mainGrove, "config.toml"), []byte("main"), 0644)

		// Create existing config in new worktree
		newGrove := filepath.Join(newDir, ".grove")
		_ = os.MkdirAll(newGrove, 0755)
		_ = os.WriteFile(filepath.Join(newGrove, "config.toml"), []byte("existing"), 0644)

		err := EnsureConfigSymlink(mainDir, newDir)
		if err != nil {
			t.Fatalf("EnsureConfigSymlink() error = %v", err)
		}

		// Should still have original content
		data, _ := os.ReadFile(filepath.Join(newGrove, "config.toml"))
		if string(data) != "existing" {
			t.Errorf("existing config was overwritten, got %q", data)
		}
	})

	t.Run("symlink target is correct", func(t *testing.T) {
		mainDir := t.TempDir()
		newDir := t.TempDir()

		mainGrove := filepath.Join(mainDir, ".grove")
		_ = os.MkdirAll(mainGrove, 0755)
		_ = os.WriteFile(filepath.Join(mainGrove, "config.toml"), []byte("test"), 0644)

		_ = EnsureConfigSymlink(mainDir, newDir)

		dst := filepath.Join(newDir, ".grove", "config.toml")
		target, err := os.Readlink(dst)
		if err != nil {
			t.Fatalf("Readlink() error = %v", err)
		}
		expected := filepath.Join(mainDir, ".grove", "config.toml")
		if target != expected {
			t.Errorf("symlink target = %q, want %q", target, expected)
		}
	})
}

func TestMustProjectRoot(t *testing.T) {
	tests := []struct {
		name     string
		groveDir string
		want     string
	}{
		{"normal path", "/home/user/project/.grove", "/home/user/project"},
		{"nested", "/a/b/c/.grove", "/a/b/c"},
		{"root-level", "/.grove", "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MustProjectRoot(tt.groveDir)
			if got != tt.want {
				t.Errorf("MustProjectRoot(%q) = %q, want %q", tt.groveDir, got, tt.want)
			}
		})
	}
}

func TestMainWorktreePath(t *testing.T) {
	// Set up a real git repo to test against
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null",
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}

	run("git", "init")
	run("git", "commit", "--allow-empty", "-m", "init")

	got, err := getMainWorktreePath(dir)
	if err != nil {
		t.Fatalf("getMainWorktreePath() error = %v", err)
	}
	if got != dir {
		t.Errorf("getMainWorktreePath() = %q, want %q", got, dir)
	}
}

func TestProjectRoot(t *testing.T) {
	t.Run("returns parent of .grove directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		// Resolve symlinks for consistent comparison (macOS /var -> /private/var)
		tmpDir, _ = filepath.EvalSymlinks(tmpDir)

		groveDir := filepath.Join(tmpDir, ".grove")
		if err := os.MkdirAll(groveDir, 0755); err != nil {
			t.Fatalf("failed to create .grove dir: %v", err)
		}

		oldWd, _ := os.Getwd()
		defer func() { _ = os.Chdir(oldWd) }()
		_ = os.Chdir(tmpDir)

		root, err := ProjectRoot()
		if err != nil {
			t.Fatalf("ProjectRoot() error = %v", err)
		}
		if root != tmpDir {
			t.Errorf("ProjectRoot() = %q, want %q", root, tmpDir)
		}
	})

	t.Run("returns empty when not in grove project", func(t *testing.T) {
		tmpDir := t.TempDir()

		oldWd, _ := os.Getwd()
		defer func() { _ = os.Chdir(oldWd) }()
		_ = os.Chdir(tmpDir)

		root, err := ProjectRoot()
		if err != nil {
			t.Fatalf("ProjectRoot() error = %v", err)
		}
		if root != "" {
			t.Errorf("ProjectRoot() = %q, want empty", root)
		}
	})
}

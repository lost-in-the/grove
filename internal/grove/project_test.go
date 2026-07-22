package grove

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitInit initializes a bare-bones git repo in dir so FindRoot treats it as
// a valid project boundary (a .grove outside a git work tree is never a
// project — see TestFindRoot_NonGitDirDoesNotWalkUp).
func gitInit(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}
}

func TestFindRoot(t *testing.T) {
	t.Run("finds .grove in current directory", func(t *testing.T) {
		// FindRoot resolves symlinks in the start dir (macOS t.TempDir lives
		// under symlinked /var), so resolve the expected path the same way.
		tmpDir, _ := filepath.EvalSymlinks(t.TempDir())
		gitInit(t, tmpDir)
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
		tmpDir, _ := filepath.EvalSymlinks(t.TempDir())
		gitInit(t, tmpDir)
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

// TestFindRoot_SecondaryWorktreeWithLocalGroveResolvesToMain guards B1: a
// grove-created secondary worktree has its OWN .grove holding only a
// config.toml symlink (EnsureConfigSymlink). FindRoot must still resolve to the
// MAIN worktree's .grove — otherwise state.json fragments per-worktree and
// commands run from inside a worktree read/write a phantom state.
func TestFindRoot_SecondaryWorktreeWithLocalGroveResolvesToMain(t *testing.T) {
	mainDir := t.TempDir()
	mainDir, _ = filepath.EvalSymlinks(mainDir)

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

	mainGrove := filepath.Join(mainDir, ".grove")
	if err := os.MkdirAll(mainGrove, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mainGrove, "config.toml"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	wtDir := mainDir + "-wt"
	run(mainDir, "git", "worktree", "add", wtDir, "-b", "test-branch")
	defer func() { _ = os.RemoveAll(wtDir) }()

	// Secondary worktree's own .grove with only a config.toml symlink, exactly
	// as grove.EnsureConfigSymlink leaves it after `grove new`.
	wtGrove := filepath.Join(wtDir, ".grove")
	if err := os.MkdirAll(wtGrove, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(mainGrove, "config.toml"), filepath.Join(wtGrove, "config.toml")); err != nil {
		t.Fatal(err)
	}

	found, err := FindRoot(wtDir)
	if err != nil {
		t.Fatalf("FindRoot() error = %v", err)
	}
	if found != mainGrove {
		t.Errorf("FindRoot(secondary worktree) = %q, want main's .grove %q", found, mainGrove)
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
		gitInit(t, tmpDir)

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

// TestIsWithinDir pins the containment boundary the main-worktree walk uses:
// the walk must stop as soon as current leaves gitRoot, even when the
// `current == gitRoot` equality never fires (EvalSymlinks divergence), and a
// sibling directory sharing a name prefix must not count as inside.
func TestIsWithinDir(t *testing.T) {
	sep := string(filepath.Separator)
	tests := []struct {
		path, dir string
		want      bool
	}{
		{"/repo", "/repo", true},
		{"/repo/sub", "/repo", true},
		{"/repo/sub/deep", "/repo", true},
		{"/repo-other", "/repo", false},
		{"/repo-other/sub", "/repo", false},
		{"/", "/repo", false},
		{"/elsewhere", "/repo", false},
		{"/repo/sub", "/", true}, // dir "/" already ends in the separator
		{"/", "/", true},
	}
	for _, tt := range tests {
		path := filepath.FromSlash(tt.path)
		dir := filepath.FromSlash(tt.dir)
		if got := isWithinDir(path, dir); got != tt.want {
			t.Errorf("isWithinDir(%q, %q) = %v, want %v (sep %q)", path, dir, got, tt.want, sep)
		}
	}
}

// TestFindRoot_SymlinkedPathDoesNotEscapeRepo: git returns the symlink-
// resolved repo root while the walk may start from a logical (symlinked)
// path. Without resolving both sides, the git-root boundary never fires and
// the walk escapes the repository, adopting an unrelated .grove above it
// (macOS: /tmp -> /private/tmp, /var -> /private/var).
func TestFindRoot_SymlinkedPathDoesNotEscapeRepo(t *testing.T) {
	base := t.TempDir()

	realDir := filepath.Join(base, "real")
	repoDir := filepath.Join(realDir, "repo")
	subDir := filepath.Join(repoDir, "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Stray .grove ABOVE the repository — must never be adopted.
	if err := os.MkdirAll(filepath.Join(realDir, ".grove"), 0755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}

	// Enter the repo through a symlink so the logical path differs from
	// git's resolved path.
	linkDir := filepath.Join(base, "link")
	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Fatal(err)
	}

	found, err := FindRoot(filepath.Join(linkDir, "repo", "sub"))
	if err != nil {
		t.Fatalf("FindRoot() error = %v", err)
	}
	if found != "" {
		t.Errorf("FindRoot() = %q, want empty — walk escaped the repo and found the stray .grove", found)
	}
}

// TestFindRoot_GitErrorIsSurfaced: a rev-parse failure that does NOT mean
// "not a git repository" (dubious ownership, unsupported repo format, broken
// config) must be returned as an error, not silently treated as "no project"
// — that would produce a false "not inside a git repository" diagnosis and
// hide git's actionable message.
func TestFindRoot_GitErrorIsSurfaced(t *testing.T) {
	dir, _ := filepath.EvalSymlinks(t.TempDir())
	gitInit(t, dir)
	if err := os.MkdirAll(filepath.Join(dir, ".grove"), 0755); err != nil {
		t.Fatal(err)
	}

	// Force a rev-parse failure that is not "not a git repository".
	cfg := filepath.Join(dir, ".git", "config")
	if err := os.WriteFile(cfg, []byte("[core]\n\trepositoryformatversion = 99\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := FindRoot(dir)
	if err == nil {
		t.Fatal("FindRoot() should surface a non-'not a git repo' git failure as an error")
	}
	if strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("error should carry git's real message, got: %v", err)
	}
}

// TestFindRoot_NonGitDirDoesNotWalkUp: outside a git work tree there is no
// git-root boundary to stop the upward walk, so the walk must not run at all.
// Otherwise a stray .grove at or above the CWD — e.g. ~/.grove, created for
// debug logs and the update-check cache — is adopted as a project root and
// bare `grove` in $HOME launches the TUI against a non-repo (#138).
func TestFindRoot_NonGitDirDoesNotWalkUp(t *testing.T) {
	base, _ := filepath.EvalSymlinks(t.TempDir())

	// Stray .grove like ~/.grove — must never be adopted outside a git repo.
	if err := os.MkdirAll(filepath.Join(base, ".grove"), 0755); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(base, "sub", "deep")
	if err := os.MkdirAll(deep, 0755); err != nil {
		t.Fatal(err)
	}

	// From a subdirectory (CWD below ~) and from base itself (CWD == ~).
	for _, dir := range []string{deep, base} {
		found, err := FindRoot(dir)
		if err != nil {
			t.Fatalf("FindRoot(%q) error = %v", dir, err)
		}
		if found != "" {
			t.Errorf("FindRoot(%q) = %q, want empty — adopted a .grove outside any git repo", dir, found)
		}
	}
}

// TestFindRoot_NestedGroveInMainWorktreeIsHonored: a .grove in a subdirectory
// of the MAIN worktree (a nested project) must win over the root's .grove when
// cwd is under it — without reintroducing the B1 fragmentation for LINKED
// worktrees (covered by TestFindRoot_SecondaryWorktreeWithLocalGroveResolvesToMain).
func TestFindRoot_NestedGroveInMainWorktreeIsHonored(t *testing.T) {
	mainDir := t.TempDir()
	mainDir, _ = filepath.EvalSymlinks(mainDir)

	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}
	run(mainDir, "git", "init")
	run(mainDir, "git", "commit", "--allow-empty", "-m", "init")

	rootGrove := filepath.Join(mainDir, ".grove")
	if err := os.MkdirAll(rootGrove, 0755); err != nil {
		t.Fatal(err)
	}

	nestedDir := filepath.Join(mainDir, "services", "api")
	nestedGrove := filepath.Join(nestedDir, ".grove")
	if err := os.MkdirAll(nestedGrove, 0755); err != nil {
		t.Fatal(err)
	}

	// From the nested subdir → the nested .grove (previously shadowed by root's).
	if found, err := FindRoot(nestedDir); err != nil {
		t.Fatalf("FindRoot(nested) error = %v", err)
	} else if found != nestedGrove {
		t.Errorf("FindRoot(nested subdir) = %q, want nested .grove %q", found, nestedGrove)
	}

	// From a subdir with no nested .grove → the root .grove.
	plainSub := filepath.Join(mainDir, "docs")
	if err := os.MkdirAll(plainSub, 0755); err != nil {
		t.Fatal(err)
	}
	if found, err := FindRoot(plainSub); err != nil {
		t.Fatalf("FindRoot(plain subdir) error = %v", err)
	} else if found != rootGrove {
		t.Errorf("FindRoot(plain subdir) = %q, want root .grove %q", found, rootGrove)
	}

	// From the main root → the root .grove.
	if found, err := FindRoot(mainDir); err != nil {
		t.Fatalf("FindRoot(root) error = %v", err)
	} else if found != rootGrove {
		t.Errorf("FindRoot(root) = %q, want root .grove %q", found, rootGrove)
	}
}

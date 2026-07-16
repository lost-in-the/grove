package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCreateFromBranch_LocalBranchNotClobberedByRemote guards B10/P1: when the
// branch already exists locally (as `grove fork` leaves it), CreateFromBranch
// must not fetch — a fast-forwardable origin/<branch> would otherwise move the
// new worktree onto the remote's commit instead of the local HEAD.
func TestCreateFromBranch_LocalBranchNotClobberedByRemote(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	base := t.TempDir()
	base, _ = filepath.EvalSymlinks(base)
	git := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}

	// A "remote" bare repo with a branch `shared`, plus one extra commit on it.
	remote := filepath.Join(base, "remote.git")
	git(base, "init", "--bare", remote)
	seed := filepath.Join(base, "seed")
	git(base, "clone", remote, seed)
	os.WriteFile(filepath.Join(seed, "a.txt"), []byte("1\n"), 0644)
	git(seed, "add", "-A")
	git(seed, "commit", "-m", "base")
	git(seed, "branch", "shared")
	git(seed, "push", "origin", "HEAD", "shared")

	// Working clone; create a LOCAL `shared` at the base commit (as fork does),
	// while origin/shared advances one commit ahead.
	work := filepath.Join(base, "work")
	git(base, "clone", remote, work)
	localHead := git(work, "rev-parse", "HEAD")
	git(work, "branch", "shared", localHead)
	os.WriteFile(filepath.Join(seed, "a.txt"), []byte("1\n2\n"), 0644)
	git(seed, "commit", "-am", "remote-ahead")
	git(seed, "push", "origin", "shared")
	git(work, "fetch", "origin")

	m := &Manager{repoRoot: work}
	if err := m.CreateFromBranch("wt-shared", "shared"); err != nil {
		t.Fatalf("CreateFromBranch: %v", err)
	}
	wt, err := m.Find("wt-shared")
	if err != nil || wt == nil {
		t.Fatalf("Find: err=%v wt=%v", err, wt)
	}
	gotHead := git(wt.Path, "rev-parse", "HEAD")
	if gotHead != localHead {
		t.Errorf("worktree HEAD = %s, want local HEAD %s (fetch clobbered the local branch)", gotHead, localHead)
	}
}

// setupTestRepo creates a temporary git repo for testing and returns cleanup function
func setupTestRepo(t *testing.T) (string, func()) {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()

	// Initialize a git repo
	initCmd := exec.Command("git", "init")
	initCmd.Dir = tmpDir
	if err := initCmd.Run(); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Configure git user locally
	configNameCmd := exec.Command("git", "config", "--local", "user.name", "Test User")
	configNameCmd.Dir = tmpDir
	if err := configNameCmd.Run(); err != nil {
		t.Fatalf("Failed to config git user.name: %v", err)
	}

	configEmailCmd := exec.Command("git", "config", "--local", "user.email", "test@example.com")
	configEmailCmd.Dir = tmpDir
	if err := configEmailCmd.Run(); err != nil {
		t.Fatalf("Failed to config git user.email: %v", err)
	}

	// Disable GPG signing for test commits
	disableSignCmd := exec.Command("git", "config", "--local", "commit.gpgsign", "false")
	disableSignCmd.Dir = tmpDir
	if err := disableSignCmd.Run(); err != nil {
		t.Fatalf("Failed to disable gpgsign: %v", err)
	}

	// Create initial commit
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	addCmd := exec.Command("git", "add", ".")
	addCmd.Dir = tmpDir
	if err := addCmd.Run(); err != nil {
		t.Fatalf("Failed to add files: %v", err)
	}

	commitCmd := exec.Command("git", "commit", "-m", "initial commit")
	commitCmd.Dir = tmpDir
	if err := commitCmd.Run(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	cleanup := func() {
		// Cleanup is handled by t.TempDir()
	}

	return tmpDir, cleanup
}

func TestParseWorktreeList(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantCount  int
		wantFirst  string
		wantBranch string
	}{
		{
			name: "single worktree",
			input: `worktree /home/user/project
HEAD 1234567
branch refs/heads/main
`,
			wantCount:  1,
			wantFirst:  "/home/user/project",
			wantBranch: "main",
		},
		{
			name: "multiple worktrees",
			input: `worktree /home/user/project
HEAD 1234567
branch refs/heads/main

worktree /home/user/project-feature
HEAD abcdef0
branch refs/heads/feature/test
`,
			wantCount:  2,
			wantFirst:  "/home/user/project",
			wantBranch: "main",
		},
		{
			name: "detached HEAD",
			input: `worktree /home/user/project
HEAD 1234567
detached
`,
			wantCount:  1,
			wantFirst:  "/home/user/project",
			wantBranch: "detached",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trees := parseWorktreeList(tt.input, "/home/user/project", "project", DefaultNamePattern)
			if len(trees) != tt.wantCount {
				t.Errorf("parseWorktreeList() got %d worktrees, want %d", len(trees), tt.wantCount)
			}
			if tt.wantCount > 0 {
				if trees[0].Path != tt.wantFirst {
					t.Errorf("First worktree path = %s, want %s", trees[0].Path, tt.wantFirst)
				}
				if trees[0].Branch != tt.wantBranch {
					t.Errorf("First worktree branch = %s, want %s", trees[0].Branch, tt.wantBranch)
				}
			}
		})
	}
}

func TestWorktreeCreate(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()

	// Initialize a git repo
	initCmd := exec.Command("git", "init")
	initCmd.Dir = tmpDir
	if err := initCmd.Run(); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Configure git user locally (required for CI environments without global config)
	configNameCmd := exec.Command("git", "config", "--local", "user.name", "Test User")
	configNameCmd.Dir = tmpDir
	if err := configNameCmd.Run(); err != nil {
		t.Fatalf("Failed to config git user.name: %v", err)
	}

	configEmailCmd := exec.Command("git", "config", "--local", "user.email", "test@example.com")
	configEmailCmd.Dir = tmpDir
	if err := configEmailCmd.Run(); err != nil {
		t.Fatalf("Failed to config git user.email: %v", err)
	}

	// Disable GPG signing for test commits
	disableSignCmd := exec.Command("git", "config", "--local", "commit.gpgsign", "false")
	disableSignCmd.Dir = tmpDir
	if err := disableSignCmd.Run(); err != nil {
		t.Fatalf("Failed to disable gpgsign: %v", err)
	}

	// Create initial commit
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	addCmd := exec.Command("git", "add", ".")
	addCmd.Dir = tmpDir
	if err := addCmd.Run(); err != nil {
		t.Fatalf("Failed to add files: %v", err)
	}

	commitCmd := exec.Command("git", "commit", "-m", "initial commit")
	commitCmd.Dir = tmpDir
	if err := commitCmd.Run(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	tests := []struct {
		name    string
		wtName  string
		branch  string
		wantErr bool
	}{
		{
			name:    "create new worktree",
			wtName:  "feature-test",
			branch:  "feature-test",
			wantErr: false,
		},
		{
			name:    "empty name",
			wtName:  "",
			branch:  "test",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Manager{
				repoRoot: tmpDir,
			}

			err := m.Create(tt.wtName, tt.branch)
			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				// Verify worktree was created
				// Create() uses FullName() which adds project prefix
				projectName := m.GetProjectName()
				fullName := projectName + "-" + tt.wtName
				wtPath := filepath.Join(tmpDir, "..", fullName)
				if _, err := os.Stat(wtPath); os.IsNotExist(err) {
					t.Errorf("Worktree directory not created: %s", wtPath)
				}
			}
		})
	}
}

func TestWorktreeList(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()

	// Initialize a git repo
	initCmd := exec.Command("git", "init")
	initCmd.Dir = tmpDir
	if err := initCmd.Run(); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Configure git user locally (required for CI environments without global config)
	configNameCmd := exec.Command("git", "config", "--local", "user.name", "Test User")
	configNameCmd.Dir = tmpDir
	if err := configNameCmd.Run(); err != nil {
		t.Fatalf("Failed to config git user.name: %v", err)
	}

	configEmailCmd := exec.Command("git", "config", "--local", "user.email", "test@example.com")
	configEmailCmd.Dir = tmpDir
	if err := configEmailCmd.Run(); err != nil {
		t.Fatalf("Failed to config git user.email: %v", err)
	}

	// Disable GPG signing for test commits
	disableSignCmd := exec.Command("git", "config", "--local", "commit.gpgsign", "false")
	disableSignCmd.Dir = tmpDir
	if err := disableSignCmd.Run(); err != nil {
		t.Fatalf("Failed to disable gpgsign: %v", err)
	}

	// Create initial commit
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	addCmd := exec.Command("git", "add", ".")
	addCmd.Dir = tmpDir
	if err := addCmd.Run(); err != nil {
		t.Fatalf("Failed to add files: %v", err)
	}

	commitCmd := exec.Command("git", "commit", "-m", "initial commit")
	commitCmd.Dir = tmpDir
	if err := commitCmd.Run(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	m := &Manager{
		repoRoot: tmpDir,
	}

	trees, err := m.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	// Should have at least the main worktree
	if len(trees) < 1 {
		t.Errorf("List() returned %d worktrees, want at least 1", len(trees))
	}
}

func TestExtractProjectNameFromRemote(t *testing.T) {
	tests := []struct {
		name        string
		remoteURL   string
		wantProject string
	}{
		{
			name:        "github https url",
			remoteURL:   "https://github.com/owner/grove-cli",
			wantProject: "grove-cli",
		},
		{
			name:        "github https url with .git",
			remoteURL:   "https://github.com/owner/grove-cli.git",
			wantProject: "grove-cli",
		},
		{
			name:        "github ssh url",
			remoteURL:   "git@github.com:owner/grove-cli.git",
			wantProject: "grove-cli",
		},
		{
			name:        "empty url",
			remoteURL:   "",
			wantProject: "",
		},
		{
			name:        "ssh url without .git",
			remoteURL:   "git@github.com:owner/grove-cli",
			wantProject: "grove-cli",
		},
		{
			name:        "nested path in ssh url",
			remoteURL:   "git@github.com:org/team/grove-cli.git",
			wantProject: "grove-cli",
		},
		{
			name:        "nested path in https url",
			remoteURL:   "https://github.com/org/team/grove-cli.git",
			wantProject: "grove-cli",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractProjectNameFromRemote(tt.remoteURL)
			if got != tt.wantProject {
				t.Errorf("extractProjectNameFromRemote() = %q, want %q", got, tt.wantProject)
			}
		})
	}
}

func TestTmuxSessionName(t *testing.T) {
	tests := []struct {
		name        string
		project     string
		worktree    string
		wantSession string
	}{
		{
			name:        "simple names",
			project:     "grove-cli",
			worktree:    "testing",
			wantSession: "grove-cli-testing",
		},
		{
			name:        "with hyphens",
			project:     "my-app",
			worktree:    "feature-auth",
			wantSession: "my-app-feature-auth",
		},
		{
			name:        "root worktree uses project name only",
			project:     "myapp",
			worktree:    "root",
			wantSession: "myapp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TmuxSessionName(tt.project, tt.worktree)
			if got != tt.wantSession {
				t.Errorf("TmuxSessionName() = %q, want %q", got, tt.wantSession)
			}
		})
	}
}

func TestNewManager(t *testing.T) {
	tmpDir, _ := setupTestRepo(t)

	// Test with explicit path
	m, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	if m == nil {
		t.Fatal("NewManager() returned nil manager")
	}

	if m.repoRoot != tmpDir {
		t.Errorf("Manager.repoRoot = %q, want %q", m.repoRoot, tmpDir)
	}
}

func TestNewManagerAutoDetect(t *testing.T) {
	tmpDir, _ := setupTestRepo(t)

	// Save and restore working directory
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()

	_ = os.Chdir(tmpDir)

	// Test with empty path (auto-detect)
	m, err := NewManager("")
	if err != nil {
		t.Fatalf("NewManager('') error = %v", err)
	}

	if m == nil {
		t.Fatal("NewManager('') returned nil manager")
	}
}

func TestNewManagerNotGitRepo(t *testing.T) {
	tmpDir := t.TempDir() // Not a git repo

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()

	_ = os.Chdir(tmpDir)

	_, err := NewManager("")
	if err == nil {
		t.Error("NewManager() expected error for non-git directory, got nil")
	}
}

func TestGetRepoRoot(t *testing.T) {
	tmpDir, _ := setupTestRepo(t)

	m := &Manager{repoRoot: tmpDir}
	got := m.GetRepoRoot()

	if got != tmpDir {
		t.Errorf("GetRepoRoot() = %q, want %q", got, tmpDir)
	}
}

func TestFind(t *testing.T) {
	tmpDir, _ := setupTestRepo(t)

	m := &Manager{repoRoot: tmpDir}

	// Create a worktree first
	err := m.Create("test-find", "test-find-branch")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	projectName := m.GetProjectName()
	fullName := projectName + "-test-find"

	// Test finding the created worktree
	wt, err := m.Find(fullName)
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}

	if wt == nil {
		t.Fatal("Find() returned nil worktree")
	}

	if !strings.HasSuffix(wt.Path, fullName) {
		t.Errorf("Found worktree path = %q, want suffix %q", wt.Path, fullName)
	}
}

func TestFindByBranchName(t *testing.T) {
	tmpDir, _ := setupTestRepo(t)

	m := &Manager{repoRoot: tmpDir}

	// The main worktree is on the "main" (or "master") branch.
	// Find("main") should locate it via branch matching.
	trees, err := m.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(trees) == 0 {
		t.Fatal("No worktrees found")
	}

	branch := trees[0].Branch
	wt, err := m.Find(branch)
	if err != nil {
		t.Fatalf("Find(%q) error = %v", branch, err)
	}
	if wt == nil {
		t.Fatalf("Find(%q) returned nil, want main worktree", branch)
	}
	if wt.Path != trees[0].Path {
		t.Errorf("Find(%q) path = %q, want %q", branch, wt.Path, trees[0].Path)
	}
}

func TestFindByShortName(t *testing.T) {
	tmpDir, _ := setupTestRepo(t)

	m := &Manager{repoRoot: tmpDir}

	// Create a worktree and verify it can be found by short name
	err := m.Create("lookup-test", "lookup-test-branch")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	wt, err := m.Find("lookup-test")
	if err != nil {
		t.Fatalf("Find('lookup-test') error = %v", err)
	}
	if wt == nil {
		t.Fatal("Find('lookup-test') returned nil")
	}
	if wt.ShortName != "lookup-test" {
		t.Errorf("ShortName = %q, want 'lookup-test'", wt.ShortName)
	}
}

// TestFindNamePrecedenceOverBranch guards B2: a worktree whose branch equals
// the query must not shadow the worktree whose short name equals the query.
func TestFindNamePrecedenceOverBranch(t *testing.T) {
	tmpDir, _ := setupTestRepo(t)
	m := &Manager{repoRoot: tmpDir}

	// Worktree "alpha" checked out on a branch literally named "zzz".
	if err := m.Create("alpha", "zzz"); err != nil {
		t.Fatalf("Create(alpha, zzz) error = %v", err)
	}
	// Worktree "zzz" on its own branch.
	if err := m.Create("zzz", "zzz-branch"); err != nil {
		t.Fatalf("Create(zzz) error = %v", err)
	}

	wt, err := m.Find("zzz")
	if err != nil {
		t.Fatalf("Find(zzz) error = %v", err)
	}
	if wt == nil {
		t.Fatal("Find(zzz) returned nil")
	}
	if wt.ShortName != "zzz" {
		t.Errorf("Find(zzz) resolved to worktree %q (branch %q); want the worktree named \"zzz\"", wt.ShortName, wt.Branch)
	}
}

func TestFindNotFound(t *testing.T) {
	tmpDir, _ := setupTestRepo(t)

	m := &Manager{repoRoot: tmpDir}

	wt, err := m.Find("nonexistent-worktree")
	// Find returns nil worktree for nonexistent, may or may not error
	if wt != nil {
		t.Error("Find() expected nil worktree for nonexistent name")
	}
	// Note: implementation may return (nil, nil) for not found
	_ = err
}

func TestRemove(t *testing.T) {
	tmpDir, _ := setupTestRepo(t)

	m := &Manager{repoRoot: tmpDir}

	// Create a worktree
	err := m.Create("test-remove", "test-remove-branch")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	projectName := m.GetProjectName()
	fullName := projectName + "-test-remove"

	// Verify it exists
	wt, err := m.Find(fullName)
	if err != nil || wt == nil {
		t.Fatalf("Find() expected worktree, got error=%v, wt=%v", err, wt)
	}

	// Remove it by name (clean worktree — plain, non-force removal succeeds)
	err = m.Remove(fullName, false)
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	// Verify it's gone - Find should return nil
	wtAfter, _ := m.Find(fullName)
	if wtAfter != nil {
		t.Error("Worktree should not exist after Remove()")
	}
}

func TestCreateFromBranch(t *testing.T) {
	tmpDir, _ := setupTestRepo(t)

	// Create a branch to use
	branchCmd := exec.Command("git", "branch", "existing-branch")
	branchCmd.Dir = tmpDir
	if err := branchCmd.Run(); err != nil {
		t.Fatalf("Failed to create branch: %v", err)
	}

	m := &Manager{repoRoot: tmpDir}

	err := m.CreateFromBranch("from-branch", "existing-branch")
	if err != nil {
		t.Fatalf("CreateFromBranch() error = %v", err)
	}

	projectName := m.GetProjectName()
	fullName := projectName + "-from-branch"

	// Verify worktree was created
	wt, err := m.Find(fullName)
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}

	if wt.Branch != "existing-branch" {
		t.Errorf("Worktree branch = %q, want 'existing-branch'", wt.Branch)
	}
}

func TestCreateFromRef(t *testing.T) {
	tmpDir, _ := setupTestRepo(t)

	// Create a second commit on a branch so we have a distinct ref
	branchCmd := exec.Command("git", "checkout", "-b", "develop")
	branchCmd.Dir = tmpDir
	if err := branchCmd.Run(); err != nil {
		t.Fatalf("Failed to create develop branch: %v", err)
	}

	testFile := filepath.Join(tmpDir, "develop.txt")
	if err := os.WriteFile(testFile, []byte("develop content"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	addCmd := exec.Command("git", "add", ".")
	addCmd.Dir = tmpDir
	if err := addCmd.Run(); err != nil {
		t.Fatalf("Failed to add: %v", err)
	}

	commitCmd := exec.Command("git", "commit", "-m", "develop commit")
	commitCmd.Dir = tmpDir
	if err := commitCmd.Run(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Get the develop commit hash for verification
	revCmd := exec.Command("git", "rev-parse", "develop")
	revCmd.Dir = tmpDir
	revOut, err := revCmd.Output()
	if err != nil {
		t.Fatalf("Failed to get develop rev: %v", err)
	}
	developSHA := strings.TrimSpace(string(revOut))

	// Switch back to main/master
	checkoutCmd := exec.Command("git", "checkout", "-")
	checkoutCmd.Dir = tmpDir
	if err := checkoutCmd.Run(); err != nil {
		t.Fatalf("Failed to checkout back: %v", err)
	}

	m := &Manager{repoRoot: tmpDir}

	t.Run("creates worktree from valid ref", func(t *testing.T) {
		err := m.CreateFromRef("from-ref", "my-feature", "develop")
		if err != nil {
			t.Fatalf("CreateFromRef() error = %v", err)
		}

		projectName := m.GetProjectName()
		fullName := projectName + "-from-ref"

		wt, err := m.Find(fullName)
		if err != nil {
			t.Fatalf("Find() error = %v", err)
		}
		if wt == nil {
			t.Fatal("Find() returned nil")
		}
		if wt.Branch != "my-feature" {
			t.Errorf("Branch = %q, want %q", wt.Branch, "my-feature")
		}

		// Verify the worktree HEAD matches the develop commit
		headCmd := exec.Command("git", "rev-parse", "HEAD")
		headCmd.Dir = wt.Path
		headOut, err := headCmd.Output()
		if err != nil {
			t.Fatalf("Failed to get HEAD: %v", err)
		}
		if strings.TrimSpace(string(headOut)) != developSHA {
			t.Errorf("HEAD = %q, want %q (develop)", strings.TrimSpace(string(headOut)), developSHA)
		}
	})

	t.Run("errors on nonexistent ref", func(t *testing.T) {
		err := m.CreateFromRef("bad-ref", "bad-branch", "nonexistent-ref-xyz")
		if err == nil {
			t.Fatal("CreateFromRef() expected error for nonexistent ref, got nil")
		}
		if !strings.Contains(err.Error(), "does not exist") {
			t.Errorf("error = %q, want it to contain 'does not exist'", err.Error())
		}
	})

	t.Run("empty fromRef creates from HEAD", func(t *testing.T) {
		err := m.CreateFromRef("head-ref", "head-branch", "")
		if err != nil {
			t.Fatalf("CreateFromRef() with empty fromRef error = %v", err)
		}

		projectName := m.GetProjectName()
		fullName := projectName + "-head-ref"

		wt, err := m.Find(fullName)
		if err != nil {
			t.Fatalf("Find() error = %v", err)
		}
		if wt == nil {
			t.Fatal("Find() returned nil")
		}
	})
}

func TestCreateFromExisting(t *testing.T) {
	tmpDir, _ := setupTestRepo(t)

	// Create an existing directory with some content
	existingDir := filepath.Join(filepath.Dir(tmpDir), "existing-dir")
	if err := os.MkdirAll(existingDir, 0755); err != nil {
		t.Fatalf("Failed to create existing dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(existingDir) }()

	m := &Manager{repoRoot: tmpDir}

	err := m.CreateFromExisting("existing-wt", existingDir)
	if err != nil {
		// CreateFromExisting may fail if the directory isn't a git repo
		// This is expected behavior - just verify it doesn't panic
		t.Logf("CreateFromExisting() error (expected): %v", err)
	}
}

func TestGetCurrent(t *testing.T) {
	tmpDir, _ := setupTestRepo(t)

	// Save and restore working directory
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()

	_ = os.Chdir(tmpDir)

	m := &Manager{repoRoot: tmpDir}

	wt, err := m.GetCurrent()
	if err != nil {
		t.Fatalf("GetCurrent() error = %v", err)
	}

	if wt == nil {
		t.Fatal("GetCurrent() returned nil")
	}

	// Should match the main repo path (handle macOS /var -> /private/var symlink)
	// Use filepath.Base to compare just the directory name
	if filepath.Base(wt.Path) != filepath.Base(tmpDir) {
		t.Errorf("GetCurrent().Path base = %q, want %q", filepath.Base(wt.Path), filepath.Base(tmpDir))
	}
}

func TestIsDirty(t *testing.T) {
	tmpDir, _ := setupTestRepo(t)

	m := &Manager{repoRoot: tmpDir}

	// Initially clean
	trees, err := m.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(trees) == 0 {
		t.Fatal("No worktrees found")
	}

	// The main worktree should be clean
	if trees[0].IsDirty {
		t.Error("Fresh repo should not be dirty")
	}

	// Make the repo dirty by modifying a file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("modified content"), 0644); err != nil {
		t.Fatalf("Failed to modify file: %v", err)
	}

	// Check again - should be dirty now
	trees, err = m.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if !trees[0].IsDirty {
		t.Error("Modified repo should be dirty")
	}
}

func TestFindMainWorktreeIsMain(t *testing.T) {
	tmpDir, _ := setupTestRepo(t)

	m := &Manager{repoRoot: tmpDir}

	// Create a linked worktree so we can test both
	err := m.Create("feature", "feature-branch")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Find via List — the main worktree should have IsMain=true
	trees, err := m.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	var mainCount, linkedCount int
	for _, tree := range trees {
		if tree.IsMain {
			mainCount++
		} else {
			linkedCount++
		}
	}

	if mainCount != 1 {
		t.Errorf("expected exactly 1 main worktree, got %d", mainCount)
	}
	if linkedCount < 1 {
		t.Errorf("expected at least 1 linked worktree, got %d", linkedCount)
	}

	// Find the linked worktree by name — should NOT be main
	projectName := m.GetProjectName()
	fullName := projectName + "-feature"
	wt, err := m.Find(fullName)
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if wt == nil {
		t.Fatal("Find() returned nil for linked worktree")
	}
	if wt.IsMain {
		t.Error("linked worktree should not have IsMain=true")
	}
}

func TestIsDirtyWithWorktree(t *testing.T) {
	tmpDir, _ := setupTestRepo(t)

	m := &Manager{repoRoot: tmpDir}

	// Create a linked worktree
	err := m.Create("dirty-test", "dirty-test-branch")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	projectName := m.GetProjectName()
	fullName := projectName + "-dirty-test"

	// Linked worktree should start clean. Find no longer populates IsDirty
	// (it skips the per-worktree status check) — verify by fetching dirty
	// state explicitly through GetDirtyFiles.
	wt, err := m.Find(fullName)
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if wt == nil {
		t.Fatal("Find() returned nil")
	}
	if files, _ := m.GetDirtyFiles(wt.Path); files != "" {
		t.Errorf("fresh worktree should not be dirty, got %q", files)
	}

	// Make the linked worktree dirty
	dirtyFile := filepath.Join(wt.Path, "dirty.txt")
	if err := os.WriteFile(dirtyFile, []byte("uncommitted"), 0644); err != nil {
		t.Fatalf("Failed to write dirty file: %v", err)
	}

	// Re-find — Find still returns the worktree, but IsDirty isn't set.
	// Use GetDirtyFiles to confirm the new file shows up.
	wt, err = m.Find(fullName)
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if wt == nil {
		t.Fatal("Find() returned nil")
	}

	dirtyFiles, err := m.GetDirtyFiles(wt.Path)
	if err != nil {
		t.Fatalf("GetDirtyFiles() error = %v", err)
	}
	if dirtyFiles == "" {
		t.Error("worktree with uncommitted file should report dirty files")
	}
	if !strings.Contains(dirtyFiles, "dirty.txt") {
		t.Errorf("GetDirtyFiles() = %q, want it to contain 'dirty.txt'", dirtyFiles)
	}
}

// TestRemove_ForceWithNonEmptyDir exercises the os.RemoveAll fallback path
// introduced for issue #24. `git worktree remove --force` refuses to delete
// a worktree directory that contains untracked subdirectories (e.g. a
// node_modules dir left by a post-create hook). Grove's Remove() falls back
// to os.RemoveAll + git worktree prune in that case.
func TestRemove_ForceWithNonEmptyDir(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir, _ := setupTestRepo(t)
	m := &Manager{repoRoot: tmpDir}

	// Create a worktree we can then pollute with an untracked directory.
	if err := m.Create("nm-test", "nm-test-branch"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	projectName := m.GetProjectName()
	fullName := projectName + "-nm-test"

	wt, err := m.Find(fullName)
	if err != nil || wt == nil {
		t.Fatalf("Find() expected worktree, got error=%v, wt=%v", err, wt)
	}

	// Plant a non-empty untracked directory inside the worktree (git's own
	// `--force` handles this fine; the os.RemoveAll fallback covers rarer cases).
	nodeModules := filepath.Join(wt.Path, "node_modules")
	if err := os.MkdirAll(filepath.Join(nodeModules, "some-pkg"), 0755); err != nil {
		t.Fatalf("MkdirAll node_modules: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nodeModules, "some-pkg", "index.js"), []byte("module.exports={}"), 0644); err != nil {
		t.Fatalf("WriteFile index.js: %v", err)
	}

	// Force removal should succeed for a non-empty untracked directory.
	if err := m.Remove(fullName, true); err != nil {
		t.Fatalf("Remove(force) with non-empty untracked dir error = %v", err)
	}

	// Verify the directory is gone.
	if _, statErr := os.Stat(wt.Path); !os.IsNotExist(statErr) {
		t.Errorf("worktree directory still exists after Remove(): %s", wt.Path)
	}

	// Verify git no longer lists it.
	wtAfter, _ := m.Find(fullName)
	if wtAfter != nil {
		t.Error("worktree should not be found after Remove()")
	}
}

// TestRemoveLockedWorktreeRefused guards B3: a git-locked worktree must never
// be force-deleted (git protects it deliberately; the os.RemoveAll fallback
// would silently defeat the lock and leave a phantom registration).
func TestRemoveLockedWorktreeRefused(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	tmpDir, _ := setupTestRepo(t)
	m := &Manager{repoRoot: tmpDir}

	if err := m.Create("locked-wt", "locked-branch"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	fullName := m.GetProjectName() + "-locked-wt"
	wt, err := m.Find(fullName)
	if err != nil || wt == nil {
		t.Fatalf("Find() expected worktree, got error=%v, wt=%v", err, wt)
	}

	// Lock it via git, exactly as a user protecting an in-progress worktree would.
	if out, lockErr := exec.Command("git", "-C", tmpDir, "worktree", "lock", wt.Path).CombinedOutput(); lockErr != nil {
		t.Fatalf("git worktree lock failed: %v: %s", lockErr, out)
	}

	// Even with force, Remove must refuse and leave the directory intact.
	if err := m.Remove(fullName, true); err == nil {
		t.Fatal("Remove(force) on a locked worktree returned nil; want refusal")
	}
	if _, statErr := os.Stat(wt.Path); os.IsNotExist(statErr) {
		t.Errorf("locked worktree directory was deleted despite the lock: %s", wt.Path)
	}
}

// TestRemoveNonForceSurfacesGitRefusal guards B3: without force, Remove must not
// escalate to os.RemoveAll — a dirty worktree removal returns an error instead
// of silently destroying the tree.
func TestRemoveNonForceSurfacesGitRefusal(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	tmpDir, _ := setupTestRepo(t)
	m := &Manager{repoRoot: tmpDir}

	if err := m.Create("dirty-wt", "dirty-branch"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	fullName := m.GetProjectName() + "-dirty-wt"
	wt, err := m.Find(fullName)
	if err != nil || wt == nil {
		t.Fatalf("Find() expected worktree, got error=%v, wt=%v", err, wt)
	}
	// Make it dirty with a tracked-file modification (git refuses plain remove).
	if err := os.WriteFile(filepath.Join(wt.Path, "README.md"), []byte("dirty change\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := m.Remove(fullName, false); err == nil {
		t.Fatal("Remove(non-force) on a dirty worktree returned nil; want git's refusal surfaced")
	}
	if _, statErr := os.Stat(wt.Path); os.IsNotExist(statErr) {
		t.Errorf("dirty worktree was destroyed by a non-force Remove: %s", wt.Path)
	}
}

func TestGetCommitInfo(t *testing.T) {
	tmpDir, _ := setupTestRepo(t)

	m := &Manager{repoRoot: tmpDir}
	trees, err := m.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(trees) == 0 {
		t.Fatal("No worktrees found")
	}

	wt := trees[0]

	// List() may or may not populate commit info depending on implementation
	// Just verify the struct fields are accessible and don't panic
	t.Logf("Worktree: Name=%q, Branch=%q, ShortCommit=%q, CommitMessage=%q",
		wt.Name, wt.Branch, wt.ShortCommit, wt.CommitMessage)

	// Branch should always be set
	if wt.Branch == "" {
		t.Error("Branch should not be empty")
	}
}

func TestRemoveByBranchName(t *testing.T) {
	// Regression: Remove used a narrower name matcher than Find, so
	// `grove rm <branch>` passed all pre-flight checks (run against Find's
	// result) and then failed with "not found" — after pre-remove hooks had
	// already fired. Remove must resolve names exactly like Find.
	tmpDir, _ := setupTestRepo(t)

	m := &Manager{repoRoot: tmpDir}

	// Slash-containing branch: the derived worktree name ("agent-slot-db")
	// differs from the branch name ("feat/agent-slot-db").
	if err := m.Create("agent-slot-db", "feat/agent-slot-db"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	wt, err := m.Find("feat/agent-slot-db")
	if err != nil || wt == nil {
		t.Fatalf("Find(branch) expected worktree, got error=%v, wt=%v", err, wt)
	}

	if err := m.Remove("feat/agent-slot-db", false); err != nil {
		t.Fatalf("Remove(branch) error = %v — Remove must accept every identifier Find accepts", err)
	}

	wtAfter, _ := m.Find("feat/agent-slot-db")
	if wtAfter != nil {
		t.Error("Worktree should not exist after Remove() by branch name")
	}
}

func TestTmuxSessionNameSanitizesTmuxUnsafeChars(t *testing.T) {
	// Regression: tmux rewrites '.' and ':' to '_' when creating a session
	// but parses them as window/pane separators in -t targets, so unsanitized
	// names never match the session tmux actually stores.
	tests := []struct {
		name        string
		project     string
		worktree    string
		wantSession string
	}{
		{
			name:        "dot in project name",
			project:     "my.app",
			worktree:    "testing",
			wantSession: "my_app-testing",
		},
		{
			name:        "dot in project name for root",
			project:     "next.js",
			worktree:    "root",
			wantSession: "next_js",
		},
		{
			name:        "colon in worktree name",
			project:     "myapp",
			worktree:    "fix:urgent",
			wantSession: "myapp-fix_urgent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TmuxSessionName(tt.project, tt.worktree)
			if got != tt.wantSession {
				t.Errorf("TmuxSessionName(%q, %q) = %q, want %q", tt.project, tt.worktree, got, tt.wantSession)
			}
		})
	}
}

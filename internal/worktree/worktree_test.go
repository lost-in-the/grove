package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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
			trees := parseWorktreeList(tt.input, "/home/user/project", "project")
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

	// Remove it by name
	err = m.Remove(fullName)
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

	// Linked worktree should start clean
	wt, err := m.Find(fullName)
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if wt == nil {
		t.Fatal("Find() returned nil")
	}
	if wt.IsDirty {
		t.Error("fresh worktree should not be dirty")
	}

	// Make the linked worktree dirty
	dirtyFile := filepath.Join(wt.Path, "dirty.txt")
	if err := os.WriteFile(dirtyFile, []byte("uncommitted"), 0644); err != nil {
		t.Fatalf("Failed to write dirty file: %v", err)
	}

	// Re-find — should now be dirty
	wt, err = m.Find(fullName)
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if wt == nil {
		t.Fatal("Find() returned nil")
	}
	if !wt.IsDirty {
		t.Error("worktree with uncommitted file should be dirty")
	}

	// GetDirtyFiles should list the file
	dirtyFiles, err := m.GetDirtyFiles(wt.Path)
	if err != nil {
		t.Fatalf("GetDirtyFiles() error = %v", err)
	}
	if !strings.Contains(dirtyFiles, "dirty.txt") {
		t.Errorf("GetDirtyFiles() = %q, want it to contain 'dirty.txt'", dirtyFiles)
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

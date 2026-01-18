package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

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
			trees := parseWorktreeList(tt.input)
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

	// Configure git user
	configNameCmd := exec.Command("git", "config", "user.name", "Test User")
	configNameCmd.Dir = tmpDir
	if err := configNameCmd.Run(); err != nil {
		t.Fatalf("Failed to config git user.name: %v", err)
	}

	configEmailCmd := exec.Command("git", "config", "user.email", "test@example.com")
	configEmailCmd.Dir = tmpDir
	if err := configEmailCmd.Run(); err != nil {
		t.Fatalf("Failed to config git user.email: %v", err)
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
				wtPath := filepath.Join(tmpDir, "..", tt.wtName)
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

	// Configure git user
	configNameCmd := exec.Command("git", "config", "user.name", "Test User")
	configNameCmd.Dir = tmpDir
	if err := configNameCmd.Run(); err != nil {
		t.Fatalf("Failed to config git user.name: %v", err)
	}

	configEmailCmd := exec.Command("git", "config", "user.email", "test@example.com")
	configEmailCmd.Dir = tmpDir
	if err := configEmailCmd.Run(); err != nil {
		t.Fatalf("Failed to config git user.email: %v", err)
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

func TestGetProjectName(t *testing.T) {
	tests := []struct {
		name         string
		remoteURL    string
		dirName      string
		wantProject  string
	}{
		{
			name:        "github https url",
			remoteURL:   "https://github.com/owner/grove-cli",
			dirName:     "grove-cli",
			wantProject: "grove-cli",
		},
		{
			name:        "github https url with .git",
			remoteURL:   "https://github.com/owner/grove-cli.git",
			dirName:     "grove-cli",
			wantProject: "grove-cli",
		},
		{
			name:        "github ssh url",
			remoteURL:   "git@github.com:owner/grove-cli.git",
			dirName:     "grove-cli",
			wantProject: "grove-cli",
		},
		{
			name:        "fallback to dir name",
			remoteURL:   "",
			dirName:     "my-project",
			wantProject: "my-project",
		},
		{
			name:        "ssh url without .git",
			remoteURL:   "git@github.com:owner/grove-cli",
			dirName:     "grove-cli",
			wantProject: "grove-cli",
		},
		{
			name:        "nested path in ssh url",
			remoteURL:   "git@github.com:org/team/grove-cli.git",
			dirName:     "grove-cli",
			wantProject: "grove-cli",
		},
		{
			name:        "nested path in https url",
			remoteURL:   "https://github.com/org/team/grove-cli.git",
			dirName:     "grove-cli",
			wantProject: "grove-cli",
		},
		{
			name:        "malformed url falls back to dir",
			remoteURL:   ":",
			dirName:     "fallback-project",
			wantProject: "fallback-project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getProjectName(tt.remoteURL, tt.dirName)
			if got != tt.wantProject {
				t.Errorf("getProjectName() = %q, want %q", got, tt.wantProject)
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &Worktree{Name: tt.worktree}
			got := TmuxSessionName(tt.project, w.Name)
			if got != tt.wantSession {
				t.Errorf("TmuxSessionName() = %q, want %q", got, tt.wantSession)
			}
		})
	}
}

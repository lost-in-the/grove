package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDetectProjectName(t *testing.T) {
	tests := []struct {
		name       string
		repoRoot   string
		remotePath string
		want       string
	}{
		{
			name:       "from git remote URL - github https",
			remotePath: "https://github.com/LeahArmstrong/grove-cli.git",
			want:       "grove-cli",
		},
		{
			name:       "from git remote URL - github ssh",
			remotePath: "git@github.com:LeahArmstrong/grove-cli.git",
			want:       "grove-cli",
		},
		{
			name:       "from git remote URL - no .git suffix",
			remotePath: "https://github.com/LeahArmstrong/grove-cli",
			want:       "grove-cli",
		},
		{
			name:       "from directory name when no remote",
			remotePath: "",
			repoRoot:   "/home/user/my-project",
			want:       "my-project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory for test
			tmpDir := t.TempDir()

			// Initialize git repo
			initCmd := exec.Command("git", "init")
			initCmd.Dir = tmpDir
			if err := initCmd.Run(); err != nil {
				t.Fatalf("Failed to init git repo: %v", err)
			}

			// Add remote if specified
			if tt.remotePath != "" {
				remoteCmd := exec.Command("git", "remote", "add", "origin", tt.remotePath)
				remoteCmd.Dir = tmpDir
				if err := remoteCmd.Run(); err != nil {
					t.Fatalf("Failed to add remote: %v", err)
				}
			}

			// Override repoRoot if specified
			repoRoot := tmpDir
			if tt.repoRoot != "" {
				// For testing fallback to directory name
				// We'll use the base name from tt.repoRoot
				repoRoot = tt.repoRoot
			}

			m := &Manager{repoRoot: repoRoot}
			got := m.detectProjectName()

			if got != tt.want {
				t.Errorf("detectProjectName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWorktreeFullName(t *testing.T) {
	tests := []struct {
		name        string
		projectName string
		shortName   string
		want        string
	}{
		{
			name:        "simple name",
			projectName: "grove-cli",
			shortName:   "testing",
			want:        "grove-cli-testing",
		},
		{
			name:        "name with hyphens",
			projectName: "grove-cli",
			shortName:   "feature-auth",
			want:        "grove-cli-feature-auth",
		},
		{
			name:        "single project name",
			projectName: "myapp",
			shortName:   "hotfix",
			want:        "myapp-hotfix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Manager{
				repoRoot:    "/fake/path",
				projectName: tt.projectName,
			}

			got := m.FullName(tt.shortName)
			if got != tt.want {
				t.Errorf("FullName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWorktreeDisplayName(t *testing.T) {
	tests := []struct {
		name        string
		projectName string
		fullPath    string
		isMain      bool
		shortName   string
		want        string
	}{
		{
			name:        "strip project prefix",
			projectName: "grove-cli",
			fullPath:    "/home/user/grove-cli-testing",
			isMain:      false,
			shortName:   "testing",
			want:        "testing",
		},
		{
			name:        "strip project prefix with hyphens",
			projectName: "grove-cli",
			fullPath:    "/home/user/grove-cli-feature-auth",
			isMain:      false,
			shortName:   "feature-auth",
			want:        "feature-auth",
		},
		{
			name:        "no prefix to strip",
			projectName: "grove-cli",
			fullPath:    "/home/user/testing",
			isMain:      false,
			shortName:   "testing",
			want:        "testing",
		},
		{
			name:        "main worktree",
			projectName: "grove-cli",
			fullPath:    "/home/user/grove-cli",
			isMain:      true,
			shortName:   "grove-cli",
			want:        "main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wt := &Worktree{
				Path:      tt.fullPath,
				IsMain:    tt.isMain,
				ShortName: tt.shortName,
			}

			got := wt.DisplayName()
			if got != tt.want {
				t.Errorf("DisplayName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectProjectNameFromConfig(t *testing.T) {
	// Test that config file project_name takes precedence
	tmpDir := t.TempDir()

	// Initialize git repo
	initCmd := exec.Command("git", "init")
	initCmd.Dir = tmpDir
	if err := initCmd.Run(); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Add remote
	remoteCmd := exec.Command("git", "remote", "add", "origin", "https://github.com/user/different-repo.git")
	remoteCmd.Dir = tmpDir
	if err := remoteCmd.Run(); err != nil {
		t.Fatalf("Failed to add remote: %v", err)
	}

	// Create .grove directory and config file
	groveDir := filepath.Join(tmpDir, ".grove")
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		t.Fatalf("Failed to create .grove dir: %v", err)
	}

	configContent := `project_name = "my-custom-project"`
	configPath := filepath.Join(groveDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	m := &Manager{repoRoot: tmpDir}
	got := m.detectProjectName()

	// Should use config value, not remote
	want := "my-custom-project"
	if got != want {
		t.Errorf("detectProjectName() with config = %q, want %q", got, want)
	}
}

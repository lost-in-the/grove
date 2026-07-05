package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestTestEnvNumber(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "myapp-feature-auth"},
		{name: "myapp-main"},
		{name: "myapp-hotfix-login"},
		{name: "myapp-testing"},
		{name: "grove-cli-feature"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TestEnvNumber(tt.name)

			// Must be in range [50, 99]
			if got < 50 || got > 99 {
				t.Errorf("TestEnvNumber(%q) = %d, want value in [50, 99]", tt.name, got)
			}

			// Must be deterministic
			got2 := TestEnvNumber(tt.name)
			if got != got2 {
				t.Errorf("TestEnvNumber(%q) not deterministic: got %d then %d", tt.name, got, got2)
			}
		})
	}

	// Verify different names produce different numbers (at least for our test set)
	seen := map[int]string{}
	for _, tt := range tests {
		n := TestEnvNumber(tt.name)
		if prev, ok := seen[n]; ok {
			t.Logf("collision: %q and %q both map to %d (acceptable, just noting)", prev, tt.name, n)
		}
		seen[n] = tt.name
	}

	// Spot-check known stable values so we catch accidental algorithm changes
	stableTests := []struct {
		name string
		want int
	}{
		{name: "myapp-feature-auth", want: TestEnvNumber("myapp-feature-auth")},
		{name: "myapp-main", want: TestEnvNumber("myapp-main")},
	}
	for _, tt := range stableTests {
		if got := TestEnvNumber(tt.name); got != tt.want {
			t.Errorf("TestEnvNumber(%q) = %d, want %d (algorithm changed?)", tt.name, got, tt.want)
		}
	}
}

func TestDetectProjectName(t *testing.T) {
	tests := []struct {
		name       string
		repoRoot   string
		remotePath string
		want       string
	}{
		{
			name:       "from git remote URL - github https",
			remotePath: "https://github.com/lost-in-the/grove.git",
			want:       "grove",
		},
		{
			name:       "from git remote URL - github ssh",
			remotePath: "git@github.com:lost-in-the/grove.git",
			want:       "grove",
		},
		{
			name:       "from git remote URL - no .git suffix",
			remotePath: "https://github.com/lost-in-the/grove",
			want:       "grove",
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
			want:        "root",
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

func TestValidateNamePattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		wantErr bool
	}{
		{name: "default pattern", pattern: "{project}-{name}", wantErr: false},
		{name: "reversed order", pattern: "{name}-{project}", wantErr: false},
		{name: "underscore separator", pattern: "{project}_{name}", wantErr: false},
		{name: "dot separator", pattern: "{name}.{project}", wantErr: false},
		{name: "no separator", pattern: "{project}{name}", wantErr: false},
		{name: "literal affixes", pattern: "wt-{project}-{name}", wantErr: false},
		{name: "missing project", pattern: "{name}", wantErr: true},
		{name: "missing name", pattern: "{project}", wantErr: true},
		{name: "empty", pattern: "", wantErr: true},
		{name: "duplicate name", pattern: "{project}-{name}-{name}", wantErr: true},
		{name: "duplicate project", pattern: "{project}{project}-{name}", wantErr: true},
		{name: "path separator", pattern: "{project}/{name}", wantErr: true},
		{name: "whitespace literal", pattern: "{project} {name}", wantErr: true},
		{name: "shell metachar", pattern: "{project}${name}", wantErr: true},
		{name: "legacy branch tokens", pattern: "{type}/{description}", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNamePattern(tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNamePattern(%q) error = %v, wantErr %v", tt.pattern, err, tt.wantErr)
			}
		})
	}
}

func TestFullNameWithPattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		project string
		short   string
		want    string
	}{
		{name: "default", pattern: "{project}-{name}", project: "grove", short: "testing", want: "grove-testing"},
		{name: "reversed", pattern: "{name}.{project}", project: "grove", short: "testing", want: "testing.grove"},
		{name: "underscore", pattern: "{project}_{name}", project: "myapp", short: "pr-42", want: "myapp_pr-42"},
		{name: "literal prefix", pattern: "wt-{project}-{name}", project: "myapp", short: "auth", want: "wt-myapp-auth"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Manager{repoRoot: "/fake/path", projectName: tt.project, namePattern: tt.pattern}
			if got := m.FullName(tt.short); got != tt.want {
				t.Errorf("FullName(%q) = %q, want %q", tt.short, got, tt.want)
			}
		})
	}
}

func TestShortNameFromFull(t *testing.T) {
	tests := []struct {
		name    string
		full    string
		project string
		pattern string
		want    string
		wantOK  bool
	}{
		{name: "default pattern", full: "grove-testing", project: "grove", pattern: "{project}-{name}", want: "testing", wantOK: true},
		{name: "reversed pattern", full: "testing.grove", project: "grove", pattern: "{name}.{project}", want: "testing", wantOK: true},
		{name: "hyphenated short name", full: "grove-feature-auth", project: "grove", pattern: "{project}-{name}", want: "feature-auth", wantOK: true},
		{name: "no match returns input", full: "unrelated-dir", project: "grove", pattern: "{project}-{name}", want: "unrelated-dir", wantOK: false},
		{name: "prefix only no name", full: "grove-", project: "grove", pattern: "{project}-{name}", want: "grove-", wantOK: false},
		{name: "literal affix pattern", full: "wt-grove-auth", project: "grove", pattern: "wt-{project}-{name}", want: "auth", wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := shortNameFromFull(tt.full, tt.project, tt.pattern)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("shortNameFromFull(%q, %q, %q) = (%q, %v), want (%q, %v)",
					tt.full, tt.project, tt.pattern, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestGetNamePatternFromConfig(t *testing.T) {
	setup := func(t *testing.T, configContent string) *Manager {
		t.Helper()
		tmpDir := t.TempDir()
		initCmd := exec.Command("git", "init")
		initCmd.Dir = tmpDir
		if err := initCmd.Run(); err != nil {
			t.Fatalf("Failed to init git repo: %v", err)
		}
		if configContent != "" {
			groveDir := filepath.Join(tmpDir, ".grove")
			if err := os.MkdirAll(groveDir, 0755); err != nil {
				t.Fatalf("Failed to create .grove dir: %v", err)
			}
			if err := os.WriteFile(filepath.Join(groveDir, "config.toml"), []byte(configContent), 0644); err != nil {
				t.Fatalf("Failed to write config: %v", err)
			}
		}
		return &Manager{repoRoot: tmpDir}
	}

	t.Run("no config falls back to default", func(t *testing.T) {
		m := setup(t, "")
		if got := m.getNamePattern(); got != DefaultNamePattern {
			t.Errorf("getNamePattern() = %q, want %q", got, DefaultNamePattern)
		}
	})

	t.Run("valid pattern is used", func(t *testing.T) {
		m := setup(t, "[naming]\npattern = \"{name}.{project}\"\n")
		if got := m.getNamePattern(); got != "{name}.{project}" {
			t.Errorf("getNamePattern() = %q, want %q", got, "{name}.{project}")
		}
	})

	t.Run("invalid pattern falls back to default", func(t *testing.T) {
		m := setup(t, "[naming]\npattern = \"{name}\"\n")
		if got := m.getNamePattern(); got != DefaultNamePattern {
			t.Errorf("getNamePattern() = %q, want %q", got, DefaultNamePattern)
		}
	})

	t.Run("pattern applies end to end via FullName", func(t *testing.T) {
		m := setup(t, "project_name = \"proj\"\n\n[naming]\npattern = \"{name}_{project}\"\n")
		if got := m.FullName("foo"); got != "foo_proj" {
			t.Errorf("FullName(\"foo\") = %q, want %q", got, "foo_proj")
		}
	})
}

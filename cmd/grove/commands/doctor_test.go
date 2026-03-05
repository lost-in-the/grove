package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckEnvFileConfig_NonDefault(t *testing.T) {
	direnvFound := func(name string) (string, error) {
		if name == "direnv" {
			return "/usr/bin/direnv", nil
		}
		return "", fmt.Errorf("not found")
	}
	miseFound := func(name string) (string, error) {
		if name == "mise" {
			return "/usr/bin/mise", nil
		}
		return "", fmt.Errorf("not found")
	}
	bothFound := func(name string) (string, error) {
		if name == "direnv" {
			return "/usr/bin/direnv", nil
		}
		if name == "mise" {
			return "/usr/bin/mise", nil
		}
		return "", fmt.Errorf("not found")
	}
	neitherFound := func(name string) (string, error) { return "", fmt.Errorf("not found") }

	tests := []struct {
		name           string
		envFileName    string
		envrcContent   string // "" means no .envrc file
		miseContent    string // "" means no .mise.toml file
		lookPath       func(string) (string, error)
		wantLoader     bool
		wantLoaderName string
		wantConfig     bool
		wantLoads      bool
		wantLoaderErr  bool
		wantConfigErr  bool
	}{
		{
			name:           "direnv installed and envrc references file",
			envFileName:    ".env.local",
			envrcContent:   "dotenv_if_exists .env.local",
			lookPath:       direnvFound,
			wantLoader:     true,
			wantLoaderName: "direnv",
			wantConfig:     true,
			wantLoads:      true,
		},
		{
			name:           "mise installed and mise.toml references file",
			envFileName:    ".env.local",
			miseContent:    "[env]\nfile = \".env.local\"",
			lookPath:       miseFound,
			wantLoader:     true,
			wantLoaderName: "mise",
			wantConfig:     true,
			wantLoads:      true,
		},
		{
			name:           "both installed, direnv preferred",
			envFileName:    ".env.local",
			envrcContent:   "dotenv_if_exists .env.local",
			lookPath:       bothFound,
			wantLoader:     true,
			wantLoaderName: "direnv",
			wantConfig:     true,
			wantLoads:      true,
		},
		{
			name:          "neither direnv nor mise installed",
			envFileName:   ".env.local",
			envrcContent:  "dotenv_if_exists .env.local",
			lookPath:      neitherFound,
			wantLoader:    false,
			wantConfig:    true,
			wantLoads:     true,
			wantLoaderErr: true,
		},
		{
			name:           "direnv installed but no config files",
			envFileName:    ".env.local",
			lookPath:       direnvFound,
			wantLoader:     true,
			wantLoaderName: "direnv",
			wantConfig:     false,
			wantConfigErr:  true,
		},
		{
			name:           "envrc exists but does not reference file",
			envFileName:    ".env.local",
			envrcContent:   "layout ruby",
			lookPath:       direnvFound,
			wantLoader:     true,
			wantLoaderName: "direnv",
			wantConfig:     true,
			wantLoads:      false,
			wantConfigErr:  true,
		},
		{
			name:           "mise installed with mise.toml not referencing file",
			envFileName:    ".env.local",
			miseContent:    "[tools]\nnode = \"20\"",
			lookPath:       miseFound,
			wantLoader:     true,
			wantLoaderName: "mise",
			wantConfig:     true,
			wantLoads:      false,
			wantConfigErr:  true,
		},
		{
			name:           "custom env file name with direnv",
			envFileName:    ".env.grove",
			envrcContent:   "dotenv_if_exists .env.grove",
			lookPath:       direnvFound,
			wantLoader:     true,
			wantLoaderName: "direnv",
			wantConfig:     true,
			wantLoads:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			if tt.envrcContent != "" {
				if err := os.WriteFile(filepath.Join(tmpDir, ".envrc"), []byte(tt.envrcContent), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if tt.miseContent != "" {
				if err := os.WriteFile(filepath.Join(tmpDir, ".mise.toml"), []byte(tt.miseContent), 0644); err != nil {
					t.Fatal(err)
				}
			}

			result := checkEnvFileConfig(tt.envFileName, tmpDir, tt.lookPath)

			if result.loaderInstalled != tt.wantLoader {
				t.Errorf("loaderInstalled = %v, want %v", result.loaderInstalled, tt.wantLoader)
			}
			if result.loaderName != tt.wantLoaderName {
				t.Errorf("loaderName = %q, want %q", result.loaderName, tt.wantLoaderName)
			}
			if result.configExists != tt.wantConfig {
				t.Errorf("configExists = %v, want %v", result.configExists, tt.wantConfig)
			}
			if result.configLoadsFile != tt.wantLoads {
				t.Errorf("configLoadsFile = %v, want %v", result.configLoadsFile, tt.wantLoads)
			}
			if (result.loaderErr != "") != tt.wantLoaderErr {
				t.Errorf("loaderErr = %q, wantErr = %v", result.loaderErr, tt.wantLoaderErr)
			}
			if (result.configErr != "") != tt.wantConfigErr {
				t.Errorf("configErr = %q, wantErr = %v", result.configErr, tt.wantConfigErr)
			}
		})
	}
}

func TestCheckEnvFileConfig_DefaultEnv(t *testing.T) {
	noopLookPath := func(name string) (string, error) { return "", nil }

	tests := []struct {
		name         string
		envrcContent string
		miseContent  string
		wantHint     bool
	}{
		{
			name:         "envrc with env.local support shows hint",
			envrcContent: "dotenv_if_exists .env.local",
			wantHint:     true,
		},
		{
			name:        "mise.toml with env.local support shows hint",
			miseContent: "[env]\nfile = \".env.local\"",
			wantHint:    true,
		},
		{
			name:         "envrc without env.local support no hint",
			envrcContent: "layout ruby",
			wantHint:     false,
		},
		{
			name:         "no config files no hint",
			envrcContent: "",
			wantHint:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			if tt.envrcContent != "" {
				if err := os.WriteFile(filepath.Join(tmpDir, ".envrc"), []byte(tt.envrcContent), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if tt.miseContent != "" {
				if err := os.WriteFile(filepath.Join(tmpDir, ".mise.toml"), []byte(tt.miseContent), 0644); err != nil {
					t.Fatal(err)
				}
			}

			result := checkEnvFileConfig(".env", tmpDir, noopLookPath)

			if result.hintAvailable != tt.wantHint {
				t.Errorf("hintAvailable = %v, want %v", result.hintAvailable, tt.wantHint)
			}
			if result.loaderInstalled {
				t.Error("loaderInstalled should be false in default mode")
			}
			if result.configExists {
				t.Error("configExists should be false in default mode")
			}
		})
	}
}

func TestCheckGroveBinary(t *testing.T) {
	tests := []struct {
		name     string
		lookPath func(string) (string, error)
		wantPass bool
		wantMsg  string
	}{
		{
			name: "binary found",
			lookPath: func(name string) (string, error) {
				return "/usr/local/bin/grove", nil
			},
			wantPass: true,
			wantMsg:  "grove",
		},
		{
			name: "binary not found",
			lookPath: func(name string) (string, error) {
				return "", fmt.Errorf("not found")
			},
			wantPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detail, err := checkGroveBinary(tt.lookPath)
			if tt.wantPass && err != nil {
				t.Errorf("expected pass, got error: %v", err)
			}
			if !tt.wantPass && err == nil {
				t.Errorf("expected fail, got pass with: %s", detail)
			}
			if tt.wantPass && !strings.Contains(detail, tt.wantMsg) {
				t.Errorf("expected detail to contain %q, got %q", tt.wantMsg, detail)
			}
		})
	}
}

// testRunGit runs a git command in the given directory, failing the test on error.
func testRunGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
		"GIT_CONFIG_GLOBAL=/dev/null",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func TestCheckConfigSymlinks(t *testing.T) {
	t.Run("all symlinks valid", func(t *testing.T) {
		mainDir := t.TempDir()
		testRunGit(t, mainDir, "init")
		testRunGit(t, mainDir, "commit", "--allow-empty", "-m", "init")

		// Create .grove/config.toml in main worktree
		groveDir := filepath.Join(mainDir, ".grove")
		if err := os.MkdirAll(groveDir, 0755); err != nil {
			t.Fatal(err)
		}
		configPath := filepath.Join(groveDir, "config.toml")
		if err := os.WriteFile(configPath, []byte("[grove]"), 0644); err != nil {
			t.Fatal(err)
		}

		// Create a secondary worktree
		wtDir := filepath.Join(t.TempDir(), "worktree")
		testRunGit(t, mainDir, "worktree", "add", wtDir, "-b", "test-branch")

		// Create .grove with valid symlink in worktree
		wtGrove := filepath.Join(wtDir, ".grove")
		if err := os.MkdirAll(wtGrove, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(configPath, filepath.Join(wtGrove, "config.toml")); err != nil {
			t.Fatal(err)
		}

		detail, err := checkConfigSymlinks(groveDir)
		if err != nil {
			t.Errorf("expected pass, got error: %v", err)
		}
		if !strings.Contains(detail, "worktrees checked") {
			t.Errorf("expected 'worktrees checked' in detail, got %q", detail)
		}
	})

	t.Run("broken symlink detected", func(t *testing.T) {
		mainDir := t.TempDir()
		testRunGit(t, mainDir, "init")
		testRunGit(t, mainDir, "commit", "--allow-empty", "-m", "init")

		// Create .grove in main (no config.toml — target will be missing)
		groveDir := filepath.Join(mainDir, ".grove")
		if err := os.MkdirAll(groveDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Create a secondary worktree
		wtDir := filepath.Join(t.TempDir(), "worktree")
		testRunGit(t, mainDir, "worktree", "add", wtDir, "-b", "test-branch")

		// Create .grove with broken symlink in worktree
		wtGrove := filepath.Join(wtDir, ".grove")
		if err := os.MkdirAll(wtGrove, 0755); err != nil {
			t.Fatal(err)
		}
		// Point to non-existent target
		if err := os.Symlink(filepath.Join(groveDir, "config.toml"), filepath.Join(wtGrove, "config.toml")); err != nil {
			t.Fatal(err)
		}

		_, err := checkConfigSymlinks(groveDir)
		if err == nil {
			t.Fatal("expected error for broken symlink, got nil")
		}
		if !strings.Contains(err.Error(), "broken symlinks") {
			t.Errorf("expected 'broken symlinks' in error, got %q", err.Error())
		}
	})

	t.Run("no worktrees besides main", func(t *testing.T) {
		mainDir := t.TempDir()
		testRunGit(t, mainDir, "init")
		testRunGit(t, mainDir, "commit", "--allow-empty", "-m", "init")

		groveDir := filepath.Join(mainDir, ".grove")
		if err := os.MkdirAll(groveDir, 0755); err != nil {
			t.Fatal(err)
		}

		detail, err := checkConfigSymlinks(groveDir)
		if err != nil {
			t.Errorf("expected pass, got error: %v", err)
		}
		if !strings.Contains(detail, "1 worktrees checked") {
			t.Errorf("expected '1 worktrees checked', got %q", detail)
		}
	})
}

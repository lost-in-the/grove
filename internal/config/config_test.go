package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	cfg := LoadDefaults()

	if cfg.Alias != "w" {
		t.Errorf("Expected default alias 'w', got '%s'", cfg.Alias)
	}
	if cfg.DefaultBranch != "main" {
		t.Errorf("Expected default branch 'main', got '%s'", cfg.DefaultBranch)
	}
	if cfg.Switch.DirtyHandling != "prompt" {
		t.Errorf("Expected default dirty handling 'prompt', got '%s'", cfg.Switch.DirtyHandling)
	}
}

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name       string
		configData string
		wantErr    bool
		validate   func(*testing.T, *Config)
	}{
		{
			name: "valid config",
			configData: `
alias = "grove"
projects_dir = "/tmp/projects"
default_base_branch = "develop"

[switch]
dirty_handling = "auto-stash"
`,
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.Alias != "grove" {
					t.Errorf("Expected alias 'grove', got '%s'", cfg.Alias)
				}
				if cfg.ProjectsDir != "/tmp/projects" {
					t.Errorf("Expected projects_dir '/tmp/projects', got '%s'", cfg.ProjectsDir)
				}
				if cfg.DefaultBranch != "develop" {
					t.Errorf("Expected default_base_branch 'develop', got '%s'", cfg.DefaultBranch)
				}
				if cfg.Switch.DirtyHandling != "auto-stash" {
					t.Errorf("Expected dirty_handling 'auto-stash', got '%s'", cfg.Switch.DirtyHandling)
				}
			},
		},
		{
			name:       "empty config uses defaults",
			configData: ``,
			wantErr:    false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.Alias != "w" {
					t.Errorf("Expected default alias 'w', got '%s'", cfg.Alias)
				}
			},
		},
		{
			name: "invalid toml",
			configData: `
alias = "grove
invalid toml here
`,
			wantErr:  true,
			validate: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp config file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.toml")
			if err := os.WriteFile(configPath, []byte(tt.configData), 0644); err != nil {
				t.Fatalf("Failed to write test config: %v", err)
			}

			cfg, err := LoadConfigFromPath(configPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfigFromPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}

func TestMergeConfigs(t *testing.T) {
	boolTrue := true
	boolFalse := false

	tests := []struct {
		name     string
		base     *Config
		override *Config
		validate func(*testing.T, *Config)
	}{
		{
			name: "override empty does nothing",
			base: &Config{
				ProjectName:   "base-project",
				Alias:         "base-alias",
				ProjectsDir:   "/base/dir",
				DefaultBranch: "main",
			},
			override: &Config{},
			validate: func(t *testing.T, cfg *Config) {
				if cfg.ProjectName != "base-project" {
					t.Errorf("Expected ProjectName 'base-project', got '%s'", cfg.ProjectName)
				}
				if cfg.Alias != "base-alias" {
					t.Errorf("Expected Alias 'base-alias', got '%s'", cfg.Alias)
				}
			},
		},
		{
			name: "override replaces values",
			base: &Config{
				ProjectName:   "base-project",
				Alias:         "base-alias",
				ProjectsDir:   "/base/dir",
				DefaultBranch: "main",
				Switch:        SwitchConfig{DirtyHandling: "prompt"},
				Naming:        NamingConfig{Pattern: "base-pattern"},
				Tmux:          TmuxConfig{Prefix: "base-prefix"},
			},
			override: &Config{
				ProjectName:   "override-project",
				Alias:         "override-alias",
				ProjectsDir:   "/override/dir",
				DefaultBranch: "develop",
				Switch:        SwitchConfig{DirtyHandling: "auto-stash"},
				Naming:        NamingConfig{Pattern: "override-pattern"},
				Tmux:          TmuxConfig{Prefix: "override-prefix"},
			},
			validate: func(t *testing.T, cfg *Config) {
				if cfg.ProjectName != "override-project" {
					t.Errorf("Expected ProjectName 'override-project', got '%s'", cfg.ProjectName)
				}
				if cfg.DefaultBranch != "develop" {
					t.Errorf("Expected DefaultBranch 'develop', got '%s'", cfg.DefaultBranch)
				}
				if cfg.Switch.DirtyHandling != "auto-stash" {
					t.Errorf("Expected DirtyHandling 'auto-stash', got '%s'", cfg.Switch.DirtyHandling)
				}
			},
		},
		{
			name: "docker plugin config merges correctly",
			base: &Config{
				Plugins: PluginsConfig{
					Docker: DockerPluginConfig{
						Enabled:   &boolTrue,
						AutoStart: &boolTrue,
						AutoStop:  &boolTrue,
					},
				},
			},
			override: &Config{
				Plugins: PluginsConfig{
					Docker: DockerPluginConfig{
						Enabled: &boolFalse,
						// AutoStart and AutoStop not set - should keep base values
					},
				},
			},
			validate: func(t *testing.T, cfg *Config) {
				if cfg.Plugins.Docker.Enabled == nil || *cfg.Plugins.Docker.Enabled != false {
					t.Errorf("Expected Enabled false")
				}
				if cfg.Plugins.Docker.AutoStart == nil || *cfg.Plugins.Docker.AutoStart != true {
					t.Errorf("Expected AutoStart to remain true from base")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeConfigs(tt.base, tt.override)
			tt.validate(t, result)
		})
	}
}

func TestGetConfigPaths(t *testing.T) {
	globalPath, projectPath, err := GetConfigPaths()
	if err != nil {
		t.Fatalf("GetConfigPaths() error = %v", err)
	}

	// Global path should contain .config/grove
	if !strings.Contains(globalPath, filepath.Join(".config", "grove", "config.toml")) {
		t.Errorf("Global path should contain .config/grove/config.toml, got '%s'", globalPath)
	}

	// Project path should contain .grove
	if !strings.Contains(projectPath, filepath.Join(".grove", "config.toml")) {
		t.Errorf("Project path should contain .grove/config.toml, got '%s'", projectPath)
	}
}

func TestLoad(t *testing.T) {
	// Save original working directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Create temp directory structure
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	groveDir := filepath.Join(projectDir, ".grove")

	if err := os.MkdirAll(groveDir, 0755); err != nil {
		t.Fatalf("Failed to create dirs: %v", err)
	}

	// Write project config
	projectConfig := `
alias = "test-alias"
default_base_branch = "develop"

[switch]
dirty_handling = "auto-stash"
`
	if err := os.WriteFile(filepath.Join(groveDir, "config.toml"), []byte(projectConfig), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Change to project directory
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Load config
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify loaded config
	if cfg.Alias != "test-alias" {
		t.Errorf("Expected alias 'test-alias', got '%s'", cfg.Alias)
	}
	if cfg.DefaultBranch != "develop" {
		t.Errorf("Expected default_base_branch 'develop', got '%s'", cfg.DefaultBranch)
	}
	if cfg.Switch.DirtyHandling != "auto-stash" {
		t.Errorf("Expected dirty_handling 'auto-stash', got '%s'", cfg.Switch.DirtyHandling)
	}
}

func TestLoadWithInvalidProjectConfig(t *testing.T) {
	// Save original working directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Create temp directory structure
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	groveDir := filepath.Join(projectDir, ".grove")

	if err := os.MkdirAll(groveDir, 0755); err != nil {
		t.Fatalf("Failed to create dirs: %v", err)
	}

	// Write invalid config
	if err := os.WriteFile(filepath.Join(groveDir, "config.toml"), []byte("invalid toml {{{"), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Change to project directory
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Load config should fail
	_, err = Load()
	if err == nil {
		t.Error("Load() expected error with invalid config, got nil")
	}
}

func TestLoadWithInvalidValidation(t *testing.T) {
	// Save original working directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Create temp directory structure
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	groveDir := filepath.Join(projectDir, ".grove")

	if err := os.MkdirAll(groveDir, 0755); err != nil {
		t.Fatalf("Failed to create dirs: %v", err)
	}

	// Write config with invalid dirty_handling
	projectConfig := `
alias = "test"
[switch]
dirty_handling = "invalid-value"
`
	if err := os.WriteFile(filepath.Join(groveDir, "config.toml"), []byte(projectConfig), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Change to project directory
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Load config should fail validation
	_, err = Load()
	if err == nil {
		t.Error("Load() expected validation error, got nil")
	}
}

func TestSetProjectConfigValues(t *testing.T) {
	// Save original working directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	tests := []struct {
		name     string
		initial  string // initial file content ("" means no file)
		updates  map[string]string
		validate func(*testing.T, string)
	}{
		{
			name: "update existing key in existing section",
			initial: `project_name = "myproject"

[switch]
dirty_handling = "prompt"
`,
			updates: map[string]string{"switch.dirty_handling": `"auto-stash"`},
			validate: func(t *testing.T, content string) {
				if !strings.Contains(content, `dirty_handling = "auto-stash"`) {
					t.Errorf("expected updated value, got:\n%s", content)
				}
				if strings.Contains(content, `"prompt"`) {
					t.Errorf("old value should be gone, got:\n%s", content)
				}
			},
		},
		{
			name: "add new key to existing section",
			initial: `[tui]
skip_branch_notice = true
`,
			updates: map[string]string{"tui.default_branch_action": `"split"`},
			validate: func(t *testing.T, content string) {
				if !strings.Contains(content, `default_branch_action = "split"`) {
					t.Errorf("expected new key, got:\n%s", content)
				}
				if !strings.Contains(content, "skip_branch_notice = true") {
					t.Errorf("existing key should be preserved, got:\n%s", content)
				}
			},
		},
		{
			name: "create new section and key",
			initial: `project_name = "myproject"
`,
			updates: map[string]string{"tui.skip_branch_notice": "true"},
			validate: func(t *testing.T, content string) {
				if !strings.Contains(content, "[tui]") {
					t.Errorf("expected [tui] section, got:\n%s", content)
				}
				if !strings.Contains(content, "skip_branch_notice = true") {
					t.Errorf("expected key, got:\n%s", content)
				}
			},
		},
		{
			name: "preserves comments",
			initial: `# Project configuration
project_name = "myproject"

[switch]
# auto-stash, prompt, refuse
dirty_handling = "prompt"
`,
			updates: map[string]string{"switch.dirty_handling": `"auto-stash"`},
			validate: func(t *testing.T, content string) {
				if !strings.Contains(content, "# Project configuration") {
					t.Errorf("top comment should be preserved, got:\n%s", content)
				}
				if !strings.Contains(content, "# auto-stash, prompt, refuse") {
					t.Errorf("section comment should be preserved, got:\n%s", content)
				}
				if !strings.Contains(content, `dirty_handling = "auto-stash"`) {
					t.Errorf("value should be updated, got:\n%s", content)
				}
			},
		},
		{
			name:    "handles missing file",
			initial: "",
			updates: map[string]string{"tui.skip_branch_notice": "true"},
			validate: func(t *testing.T, content string) {
				if !strings.Contains(content, "[tui]") {
					t.Errorf("expected [tui] section, got:\n%s", content)
				}
				if !strings.Contains(content, "skip_branch_notice = true") {
					t.Errorf("expected key, got:\n%s", content)
				}
			},
		},
		{
			name: "top-level key without section",
			initial: `project_name = "old"

[switch]
dirty_handling = "prompt"
`,
			updates: map[string]string{"project_name": `"new"`},
			validate: func(t *testing.T, content string) {
				if !strings.Contains(content, `project_name = "new"`) {
					t.Errorf("expected updated top-level key, got:\n%s", content)
				}
				if strings.Contains(content, `"old"`) {
					t.Errorf("old value should be gone, got:\n%s", content)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			projectDir := filepath.Join(tmpDir, "project")
			groveDir := filepath.Join(projectDir, ".grove")
			if err := os.MkdirAll(groveDir, 0755); err != nil {
				t.Fatalf("Failed to create dirs: %v", err)
			}

			configPath := filepath.Join(groveDir, "config.toml")
			if tt.initial != "" {
				if err := os.WriteFile(configPath, []byte(tt.initial), 0644); err != nil {
					t.Fatalf("Failed to write initial config: %v", err)
				}
			}

			if err := os.Chdir(projectDir); err != nil {
				t.Fatalf("Failed to chdir: %v", err)
			}

			if err := SetProjectConfigValues(tt.updates); err != nil {
				t.Fatalf("SetProjectConfigValues() error = %v", err)
			}

			data, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatalf("Failed to read result: %v", err)
			}
			tt.validate(t, string(data))
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid config",
			config:  LoadDefaults(),
			wantErr: false,
		},
		{
			name: "invalid dirty handling",
			config: &Config{
				Alias:         "w",
				ProjectsDir:   "/tmp",
				DefaultBranch: "main",
				Switch: SwitchConfig{
					DirtyHandling: "invalid",
				},
			},
			wantErr: true,
			errMsg:  "dirty_handling",
		},
		{
			name: "empty alias",
			config: &Config{
				Alias:         "",
				ProjectsDir:   "/tmp",
				DefaultBranch: "main",
				Switch: SwitchConfig{
					DirtyHandling: "prompt",
				},
			},
			wantErr: true,
			errMsg:  "alias",
		},
		{
			name: "empty dirty handling",
			config: &Config{
				Alias:         "w",
				ProjectsDir:   "/tmp",
				DefaultBranch: "main",
				Switch: SwitchConfig{
					DirtyHandling: "",
				},
			},
			wantErr: true,
			errMsg:  "dirty_handling",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" {
				if err == nil {
					t.Errorf("Expected error message to contain '%s'", tt.errMsg)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error message to contain '%s', got '%s'", tt.errMsg, err.Error())
				}
			}
		})
	}
}

func TestLoadExternalDockerConfig(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "shared-compose")
	if err := os.MkdirAll(composePath, 0755); err != nil {
		t.Fatalf("Failed to create compose dir: %v", err)
	}

	configData := `
[plugins.docker]
enabled = true
mode = "external"

[plugins.docker.external]
path = "` + composePath + `"
env_var = "ADMIN_DIR"
services = ["admin", "admin_sidekiq"]
copy_files = ["config/credentials/development.key"]
symlink_dirs = ["vendor/bundle"]
`
	configPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(configData), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	cfg, err := LoadConfigFromPath(configPath)
	if err != nil {
		t.Fatalf("LoadConfigFromPath() error = %v", err)
	}

	if cfg.Plugins.Docker.Mode != "external" {
		t.Errorf("Expected mode 'external', got %q", cfg.Plugins.Docker.Mode)
	}
	if cfg.Plugins.Docker.External == nil {
		t.Fatal("Expected External config to be non-nil")
	}
	ext := cfg.Plugins.Docker.External
	if ext.Path != composePath {
		t.Errorf("Expected path %q, got %q", composePath, ext.Path)
	}
	if ext.EnvVar != "ADMIN_DIR" {
		t.Errorf("Expected env_var 'ADMIN_DIR', got %q", ext.EnvVar)
	}
	if len(ext.Services) != 2 {
		t.Errorf("Expected 2 services, got %d", len(ext.Services))
	}
	if len(ext.CopyFiles) != 1 {
		t.Errorf("Expected 1 copy_files entry, got %d", len(ext.CopyFiles))
	}
	if len(ext.SymlinkDirs) != 1 {
		t.Errorf("Expected 1 symlink_dirs entry, got %d", len(ext.SymlinkDirs))
	}
}

func TestMergeConfigsExternalDocker(t *testing.T) {
	boolTrue := true

	base := &Config{
		Alias:         "w",
		DefaultBranch: "main",
		Switch:        SwitchConfig{DirtyHandling: "prompt"},
		Plugins: PluginsConfig{
			Docker: DockerPluginConfig{
				Enabled:   &boolTrue,
				AutoStart: &boolTrue,
			},
		},
	}

	override := &Config{
		Plugins: PluginsConfig{
			Docker: DockerPluginConfig{
				Mode: "external",
				External: &ExternalComposeConfig{
					Path:     "/tmp/shared-compose",
					EnvVar:   "ADMIN_DIR",
					Services: []string{"admin"},
				},
			},
		},
	}

	result := mergeConfigs(base, override)

	if result.Plugins.Docker.Mode != "external" {
		t.Errorf("Expected mode 'external', got %q", result.Plugins.Docker.Mode)
	}
	if result.Plugins.Docker.External == nil {
		t.Fatal("Expected External config to be preserved from override")
	}
	if result.Plugins.Docker.External.EnvVar != "ADMIN_DIR" {
		t.Errorf("Expected env_var 'ADMIN_DIR', got %q", result.Plugins.Docker.External.EnvVar)
	}
	// Base values should be preserved
	if result.Plugins.Docker.Enabled == nil || !*result.Plugins.Docker.Enabled {
		t.Error("Expected Enabled to remain true from base")
	}
	if result.Plugins.Docker.AutoStart == nil || !*result.Plugins.Docker.AutoStart {
		t.Error("Expected AutoStart to remain true from base")
	}
}

func TestValidateDockerPlugin(t *testing.T) {
	tmpDir := t.TempDir()

	validBase := func() *Config {
		return &Config{
			Alias:         "w",
			DefaultBranch: "main",
			Switch:        SwitchConfig{DirtyHandling: "prompt"},
		}
	}

	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr bool
		errMsg  string
	}{
		{
			name:    "local mode (empty) is valid",
			modify:  func(c *Config) {},
			wantErr: false,
		},
		{
			name:    "explicit local mode is valid",
			modify:  func(c *Config) { c.Plugins.Docker.Mode = "local" },
			wantErr: false,
		},
		{
			name:    "invalid mode",
			modify:  func(c *Config) { c.Plugins.Docker.Mode = "invalid" },
			wantErr: true,
			errMsg:  "plugins.docker.mode",
		},
		{
			name: "external mode without external config",
			modify: func(c *Config) {
				c.Plugins.Docker.Mode = "external"
			},
			wantErr: true,
			errMsg:  "plugins.docker.external is required",
		},
		{
			name: "external mode without path",
			modify: func(c *Config) {
				c.Plugins.Docker.Mode = "external"
				c.Plugins.Docker.External = &ExternalComposeConfig{
					EnvVar:   "ADMIN_DIR",
					Services: []string{"admin"},
				}
			},
			wantErr: true,
			errMsg:  "path is required",
		},
		{
			name: "external mode without env_var",
			modify: func(c *Config) {
				c.Plugins.Docker.Mode = "external"
				c.Plugins.Docker.External = &ExternalComposeConfig{
					Path:     tmpDir,
					Services: []string{"admin"},
				}
			},
			wantErr: true,
			errMsg:  "env_var is required",
		},
		{
			name: "external mode without services",
			modify: func(c *Config) {
				c.Plugins.Docker.Mode = "external"
				c.Plugins.Docker.External = &ExternalComposeConfig{
					Path:   tmpDir,
					EnvVar: "ADMIN_DIR",
				}
			},
			wantErr: true,
			errMsg:  "services is required",
		},
		{
			name: "external mode with nonexistent path",
			modify: func(c *Config) {
				c.Plugins.Docker.Mode = "external"
				c.Plugins.Docker.External = &ExternalComposeConfig{
					Path:     "/nonexistent/path",
					EnvVar:   "ADMIN_DIR",
					Services: []string{"admin"},
				}
			},
			wantErr: true,
			errMsg:  "does not exist",
		},
		{
			name: "valid external mode",
			modify: func(c *Config) {
				c.Plugins.Docker.Mode = "external"
				c.Plugins.Docker.External = &ExternalComposeConfig{
					Path:     tmpDir,
					EnvVar:   "ADMIN_DIR",
					Services: []string{"admin", "admin_sidekiq"},
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validBase()
			tt.modify(cfg)
			err := Validate(cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			}
		})
	}
}

func TestIsExternalDockerMode(t *testing.T) {
	cfg := &Config{}
	if cfg.IsExternalDockerMode() {
		t.Error("empty mode should not be external")
	}

	cfg.Plugins.Docker.Mode = "local"
	if cfg.IsExternalDockerMode() {
		t.Error("local mode should not be external")
	}

	cfg.Plugins.Docker.Mode = "external"
	if !cfg.IsExternalDockerMode() {
		t.Error("external mode should be external")
	}
}

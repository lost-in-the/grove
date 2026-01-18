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

[tmux]
prefix = "grove-"
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

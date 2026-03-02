package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDefaults_AllFields(t *testing.T) {
	cfg := LoadDefaults()
	if cfg.Alias != "w" {
		t.Errorf("Alias = %q, want %q", cfg.Alias, "w")
	}
	if cfg.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q, want %q", cfg.DefaultBranch, "main")
	}
	if cfg.Switch.DirtyHandling != "prompt" {
		t.Errorf("DirtyHandling = %q, want %q", cfg.Switch.DirtyHandling, "prompt")
	}
	if cfg.Tmux.Mode != "auto" {
		t.Errorf("Tmux.Mode = %q, want %q", cfg.Tmux.Mode, "auto")
	}
	if cfg.Plugins.Docker.Enabled == nil || !*cfg.Plugins.Docker.Enabled {
		t.Error("Docker.Enabled should default to true")
	}
	if cfg.Plugins.Docker.AutoStart == nil || !*cfg.Plugins.Docker.AutoStart {
		t.Error("Docker.AutoStart should default to true")
	}
	if cfg.Plugins.Docker.AutoStop == nil || *cfg.Plugins.Docker.AutoStop {
		t.Error("Docker.AutoStop should default to false")
	}
	if cfg.Session.PopupWidth != "80%" {
		t.Errorf("Session.PopupWidth = %q, want %q", cfg.Session.PopupWidth, "80%")
	}
}

func TestLoadConfigFromPath(t *testing.T) {
	t.Run("valid toml", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		content := `alias = "g"
default_base_branch = "develop"
`
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		cfg, err := LoadConfigFromPath(path)
		if err != nil {
			t.Fatalf("LoadConfigFromPath() error = %v", err)
		}
		if cfg.Alias != "g" {
			t.Errorf("Alias = %q, want %q", cfg.Alias, "g")
		}
		if cfg.DefaultBranch != "develop" {
			t.Errorf("DefaultBranch = %q, want %q", cfg.DefaultBranch, "develop")
		}
		// Defaults should be preserved for unset values
		if cfg.Tmux.Mode != "auto" {
			t.Errorf("Tmux.Mode = %q, want default %q", cfg.Tmux.Mode, "auto")
		}
	})

	t.Run("invalid toml", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		if err := os.WriteFile(path, []byte("not valid [[ toml"), 0644); err != nil {
			t.Fatal(err)
		}
		_, err := LoadConfigFromPath(path)
		if err == nil {
			t.Error("expected error for invalid TOML")
		}
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := LoadConfigFromPath("/nonexistent/config.toml")
		if err == nil {
			t.Error("expected error for missing file")
		}
	})
}

// TestValidate_DefaultBranch covers the empty DefaultBranch case not in config_test.go.
func TestValidate_DefaultBranch(t *testing.T) {
	cfg := &Config{
		Alias:         "w",
		DefaultBranch: "",
		Switch:        SwitchConfig{DirtyHandling: "prompt"},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for empty default_base_branch, got nil")
	}
	if !strings.Contains(err.Error(), "default_base_branch") {
		t.Errorf("expected error about default_base_branch, got %q", err.Error())
	}
}

// TestValidate_DirtyHandlingValidValues confirms each valid value passes.
func TestValidate_DirtyHandlingValidValues(t *testing.T) {
	for _, val := range []string{"auto-stash", "prompt", "refuse"} {
		t.Run(val, func(t *testing.T) {
			cfg := &Config{
				Alias:         "w",
				DefaultBranch: "main",
				Switch:        SwitchConfig{DirtyHandling: val},
			}
			if err := Validate(cfg); err != nil {
				t.Errorf("valid dirty_handling %q should pass, got error: %v", val, err)
			}
		})
	}
}

// TestValidate_TmuxMode covers tmux mode validation including valid and invalid values.
func TestValidate_TmuxMode(t *testing.T) {
	tests := []struct {
		name    string
		mode    string
		wantErr bool
	}{
		{"empty is valid", "", false},
		{"auto is valid", "auto", false},
		{"manual is valid", "manual", false},
		{"off is valid", "off", false},
		{"invalid mode errors", "always", true},
		{"typo errors", "Auto", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Alias:         "w",
				DefaultBranch: "main",
				Switch:        SwitchConfig{DirtyHandling: "prompt"},
				Tmux:          TmuxConfig{Mode: tt.mode},
			}
			err := Validate(cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && !strings.Contains(err.Error(), "tmux.mode") {
				t.Errorf("expected error about tmux.mode, got %q", err.Error())
			}
		})
	}
}

// TestValidate_OnSwitch covers on_switch validation including valid and invalid values.
func TestValidate_OnSwitch(t *testing.T) {
	tests := []struct {
		name     string
		onSwitch string
		wantErr  bool
	}{
		{"empty is valid", "", false},
		{"reset is valid", "reset", false},
		{"warn is valid", "warn", false},
		{"ignore is valid", "ignore", false},
		{"invalid value errors", "delete", true},
		{"typo errors", "Reset", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Alias:         "w",
				DefaultBranch: "main",
				Switch:        SwitchConfig{DirtyHandling: "prompt"},
				Tmux:          TmuxConfig{OnSwitch: tt.onSwitch},
			}
			err := Validate(cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && !strings.Contains(err.Error(), "tmux.on_switch") {
				t.Errorf("expected error about tmux.on_switch, got %q", err.Error())
			}
		})
	}
}

// TestValidateDockerPlugin_FileNotDir covers the path-is-a-file case missing from config_test.go.
func TestValidateDockerPlugin_FileNotDir(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "not-a-dir.txt")
	if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	cfg := &Config{
		Alias:         "w",
		DefaultBranch: "main",
		Switch:        SwitchConfig{DirtyHandling: "prompt"},
		Plugins: PluginsConfig{
			Docker: DockerPluginConfig{
				Mode: "external",
				External: &ExternalComposeConfig{
					Path:     filePath,
					EnvVar:   "APP_DIR",
					Services: []string{"app"},
				},
			},
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error when path is a file, got nil")
	}
	if !strings.Contains(err.Error(), "is not a directory") {
		t.Errorf("expected error about not being a directory, got %q", err.Error())
	}
}

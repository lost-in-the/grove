package docker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LeahArmstrong/grove-cli/internal/config"
	"github.com/LeahArmstrong/grove-cli/internal/hooks"
)

func TestNew(t *testing.T) {
	plugin := New()
	if plugin == nil {
		t.Fatal("New() returned nil")
	}
	if !plugin.enabled {
		t.Error("Plugin should be enabled by default")
	}
}

func TestPlugin_Name(t *testing.T) {
	plugin := New()
	if plugin.Name() != "docker" {
		t.Errorf("Name() = %s, want %s", plugin.Name(), "docker")
	}
}

func TestPlugin_Init(t *testing.T) {
	plugin := New()
	cfg := &config.Config{
		ProjectsDir: "/tmp/projects",
	}

	err := plugin.Init(cfg)
	// Error is acceptable if docker/docker-compose is not installed
	if err != nil && plugin.enabled {
		t.Error("Plugin should be disabled if docker is not available")
	}

	if plugin.cfg != cfg {
		t.Error("Config not set correctly")
	}
}

func TestPlugin_Enabled(t *testing.T) {
	plugin := New()
	plugin.enabled = true
	if !plugin.Enabled() {
		t.Error("Enabled() should return true")
	}

	plugin.enabled = false
	if plugin.Enabled() {
		t.Error("Enabled() should return false")
	}
}

func TestPlugin_RegisterHooks(t *testing.T) {
	plugin := New()
	cfg := &config.Config{}
	_ = plugin.Init(cfg)

	registry := hooks.NewRegistry()
	err := plugin.RegisterHooks(registry)
	if err != nil {
		t.Errorf("RegisterHooks() error = %v", err)
	}

	// Test that hooks were registered by firing them
	ctx := &hooks.Context{
		Worktree:     "test-worktree",
		PrevWorktree: "prev-worktree",
		Config:       cfg,
	}

	// These should not error even if docker-compose doesn't exist
	// because the plugin checks for docker-compose files first
	_ = registry.Fire(hooks.EventPostSwitch, ctx)
	_ = registry.Fire(hooks.EventPreSwitch, ctx)
}

func TestPlugin_HasDockerCompose(t *testing.T) {
	plugin := New()

	// Create temp directory for testing
	tmpDir, err := os.MkdirTemp("", "grove-docker-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{
			name:     "docker-compose.yml exists",
			filename: "docker-compose.yml",
			want:     true,
		},
		{
			name:     "docker-compose.yaml exists",
			filename: "docker-compose.yaml",
			want:     true,
		},
		{
			name:     "compose.yml exists",
			filename: "compose.yml",
			want:     true,
		},
		{
			name:     "compose.yaml exists",
			filename: "compose.yaml",
			want:     true,
		},
		{
			name:     "no compose file",
			filename: "",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := filepath.Join(tmpDir, tt.name)
			if err := os.MkdirAll(testDir, 0755); err != nil {
				t.Fatal(err)
			}

			if tt.filename != "" {
				file := filepath.Join(testDir, tt.filename)
				if err := os.WriteFile(file, []byte("version: '3'\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}

			got := plugin.hasDockerCompose(testDir)
			if got != tt.want {
				t.Errorf("hasDockerCompose() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlugin_GetWorktreePath(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.Config
		worktree    string
		wantContain string
	}{
		{
			name: "absolute path",
			config: &config.Config{
				ProjectsDir: "/tmp/projects",
			},
			worktree:    "/absolute/path/to/worktree",
			wantContain: "/absolute/path/to/worktree",
		},
		{
			name: "relative with projects dir",
			config: &config.Config{
				ProjectsDir: "/tmp/projects",
			},
			worktree:    "my-worktree",
			wantContain: "my-worktree",
		},
		{
			name: "empty worktree",
			config: &config.Config{
				ProjectsDir: "/tmp/projects",
			},
			worktree:    "",
			wantContain: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := New()
			plugin.cfg = tt.config

			got := plugin.getWorktreePath(tt.worktree)
			if tt.wantContain != "" && got == "" {
				t.Errorf("getWorktreePath() returned empty string")
			}
			if tt.wantContain == "" && got != "" {
				t.Errorf("getWorktreePath() = %v, want empty", got)
			}
		})
	}
}

func TestPlugin_GetAutoStart(t *testing.T) {
	plugin := New()
	// Should default to true
	if !plugin.getAutoStart() {
		t.Error("getAutoStart() should default to true")
	}
}

func TestPlugin_GetAutoStop(t *testing.T) {
	plugin := New()
	// Should default to false
	if plugin.getAutoStop() {
		t.Error("getAutoStop() should default to false")
	}
}

func TestPlugin_ComposeCommand(t *testing.T) {
	plugin := New()

	tests := []struct {
		name         string
		worktreePath string
		args         []string
	}{
		{
			name:         "up command",
			worktreePath: "/tmp/test",
			args:         []string{"up", "-d"},
		},
		{
			name:         "down command",
			worktreePath: "/tmp/test",
			args:         []string{"down"},
		},
		{
			name:         "logs command",
			worktreePath: "/tmp/test",
			args:         []string{"logs", "-f", "web"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := plugin.composeCommand(tt.worktreePath, tt.args...)
			if cmd == nil {
				t.Fatal("composeCommand() returned nil")
			}
			if cmd.Dir != tt.worktreePath {
				t.Errorf("Command Dir = %s, want %s", cmd.Dir, tt.worktreePath)
			}
		})
	}
}

func TestPlugin_OnPostSwitch(t *testing.T) {
	plugin := New()
	cfg := &config.Config{
		ProjectsDir: "/tmp/projects",
	}
	_ = plugin.Init(cfg)

	ctx := &hooks.Context{
		Worktree: "test-worktree",
		Config:   cfg,
	}

	// Should not error if docker-compose file doesn't exist
	err := plugin.onPostSwitch(ctx)
	if err != nil {
		t.Errorf("onPostSwitch() error = %v, want nil", err)
	}
}

func TestPlugin_OnPreSwitch(t *testing.T) {
	plugin := New()
	cfg := &config.Config{
		ProjectsDir: "/tmp/projects",
	}
	_ = plugin.Init(cfg)

	ctx := &hooks.Context{
		PrevWorktree: "prev-worktree",
		Config:       cfg,
	}

	// Should not error if docker-compose file doesn't exist
	err := plugin.onPreSwitch(ctx)
	if err != nil {
		t.Errorf("onPreSwitch() error = %v, want nil", err)
	}
}

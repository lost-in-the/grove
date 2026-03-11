package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/hooks"
)

func newTestAgentConfig(t *testing.T) *config.Config {
	t.Helper()
	tmpDir := t.TempDir()
	enabled := true
	return &config.Config{
		ProjectName: "myapp",
		Plugins: config.PluginsConfig{
			Docker: config.DockerPluginConfig{
				Mode: "external",
				External: &config.ExternalComposeConfig{
					Path:     tmpDir,
					EnvVar:   "APP_DIR",
					Services: []string{"app"},
					Agent: &config.AgentStackConfig{
						Enabled:      &enabled,
						MaxSlots:     3,
						Services:     []string{"app", "app_worker"},
						TemplatePath: "agent-stacks/template.yml",
						URLPattern:   "https://agent-{slot}--app.example.com",
					},
				},
			},
		},
	}
}

func TestAgentExternalStrategy_OnPreSwitch(t *testing.T) {
	cfg := newTestAgentConfig(t)
	s := newAgentExternalStrategy(cfg)

	ctx := &hooks.Context{Worktree: "test"}
	if err := s.OnPreSwitch(ctx); err != nil {
		t.Errorf("OnPreSwitch() error = %v, want nil", err)
	}
}

func TestAgentExternalStrategy_OnPostSwitch(t *testing.T) {
	cfg := newTestAgentConfig(t)
	s := newAgentExternalStrategy(cfg)

	ctx := &hooks.Context{Worktree: "test"}
	if err := s.OnPostSwitch(ctx); err != nil {
		t.Errorf("OnPostSwitch() error = %v, want nil", err)
	}
}

func TestAgentExternalStrategy_OnPostCreate_NoPaths(t *testing.T) {
	cfg := newTestAgentConfig(t)
	s := newAgentExternalStrategy(cfg)

	ctx := &hooks.Context{Worktree: "test"}
	if err := s.OnPostCreate(ctx); err != nil {
		t.Errorf("OnPostCreate() with no paths error = %v, want nil", err)
	}
}

func TestAgentExternalStrategy_OnPostCreate_CopiesFiles(t *testing.T) {
	tmpDir := t.TempDir()

	mainPath := filepath.Join(tmpDir, "main")
	_ = os.MkdirAll(filepath.Join(mainPath, "config"), 0755)
	_ = os.WriteFile(filepath.Join(mainPath, "config", "secret.key"), []byte("secret"), 0600)

	newPath := filepath.Join(tmpDir, "worktree")
	_ = os.MkdirAll(newPath, 0755)

	enabled := true
	cfg := &config.Config{
		ProjectName: "myapp",
		Plugins: config.PluginsConfig{
			Docker: config.DockerPluginConfig{
				Mode: "external",
				External: &config.ExternalComposeConfig{
					Path:      tmpDir,
					EnvVar:    "APP_DIR",
					Services:  []string{"app"},
					CopyFiles: []string{"config/secret.key"},
					Agent: &config.AgentStackConfig{
						Enabled:      &enabled,
						MaxSlots:     3,
						Services:     []string{"app"},
						TemplatePath: "agent-stacks/template.yml",
					},
				},
			},
		},
	}

	s := newAgentExternalStrategy(cfg)
	ctx := &hooks.Context{
		Worktree:     "worktree",
		WorktreePath: newPath,
		MainPath:     mainPath,
	}

	if err := s.OnPostCreate(ctx); err != nil {
		t.Fatalf("OnPostCreate() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(newPath, "config", "secret.key"))
	if err != nil {
		t.Fatalf("Failed to read copied file: %v", err)
	}
	if string(data) != "secret" {
		t.Errorf("Expected 'secret', got %q", string(data))
	}
}

func TestAgentExternalStrategy_ComposeProjectName(t *testing.T) {
	cfg := newTestAgentConfig(t)
	s := newAgentExternalStrategy(cfg)

	tests := []struct {
		name string
		slot int
		want string
	}{
		{"slot 1", 1, "myapp-agent-1"},
		{"slot 5", 5, "myapp-agent-5"},
		{"slot 0 ephemeral", 0, "myapp-agent-ephemeral"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.composeProjectName(tt.slot)
			if got != tt.want {
				t.Errorf("composeProjectName(%d) = %q, want %q", tt.slot, got, tt.want)
			}
		})
	}
}

func TestAgentExternalStrategy_ComposeProjectName_DefaultProject(t *testing.T) {
	cfg := newTestAgentConfig(t)
	cfg.ProjectName = ""
	s := newAgentExternalStrategy(cfg)

	got := s.composeProjectName(2)
	want := "grove-agent-2"
	if got != want {
		t.Errorf("composeProjectName(2) with empty project = %q, want %q", got, want)
	}
}

func TestAgentExternalStrategy_ResolveTemplatePath(t *testing.T) {
	cfg := newTestAgentConfig(t)
	s := newAgentExternalStrategy(cfg)

	// Relative path should be joined with compose path
	got := s.resolveTemplatePath()
	if !strings.HasSuffix(got, "agent-stacks/template.yml") {
		t.Errorf("resolveTemplatePath() = %q, want suffix 'agent-stacks/template.yml'", got)
	}

	// Absolute path should be returned as-is
	s.agent.TemplatePath = "/abs/path/template.yml"
	got = s.resolveTemplatePath()
	if got != "/abs/path/template.yml" {
		t.Errorf("resolveTemplatePath() = %q, want '/abs/path/template.yml'", got)
	}
}

func TestAgentExternalStrategy_AgentEnv(t *testing.T) {
	cfg := newTestAgentConfig(t)
	s := newAgentExternalStrategy(cfg)

	tests := []struct {
		name     string
		path     string
		slot     int
		wantLen  int
		wantSlot string
	}{
		{"with slot", "/tmp/wt", 2, 2, "AGENT_SLOT=2"},
		{"without slot", "/tmp/wt", 0, 1, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := s.agentEnv(tt.path, tt.slot)
			if len(env) != tt.wantLen {
				t.Errorf("agentEnv() returned %d vars, want %d", len(env), tt.wantLen)
			}
			if env[0] != "APP_DIR=/tmp/wt" {
				t.Errorf("agentEnv()[0] = %q, want APP_DIR=/tmp/wt", env[0])
			}
			if tt.wantSlot != "" && len(env) > 1 && env[1] != tt.wantSlot {
				t.Errorf("agentEnv()[1] = %q, want %q", env[1], tt.wantSlot)
			}
		})
	}
}

func TestAgentComposeCommand(t *testing.T) {
	cmd := agentComposeCommand("/tmp/compose", "/tmp/compose/template.yml", "myapp-agent-1", []string{"APP_DIR=/app"}, "up", "-d", "app")

	if cmd.Dir != "/tmp/compose" {
		t.Errorf("cmd.Dir = %q, want /tmp/compose", cmd.Dir)
	}

	wantArgs := []string{"docker", "compose", "-f", "/tmp/compose/template.yml", "-p", "myapp-agent-1", "up", "-d", "app"}
	if len(cmd.Args) != len(wantArgs) {
		t.Fatalf("cmd.Args length = %d, want %d: %v", len(cmd.Args), len(wantArgs), cmd.Args)
	}
	for i, want := range wantArgs {
		if cmd.Args[i] != want {
			t.Errorf("cmd.Args[%d] = %q, want %q", i, cmd.Args[i], want)
		}
	}
}

func TestAgentComposeCommand_WithEnv(t *testing.T) {
	cmd := agentComposeCommand("/tmp", "/tmp/template.yml", "test-agent-1", []string{"FOO=bar"}, "up")
	if len(cmd.Env) == 0 {
		t.Error("Expected env vars to be set")
	}
}

func TestAgentComposeCommand_WithoutEnv(t *testing.T) {
	cmd := agentComposeCommand("/tmp", "/tmp/template.yml", "test-agent-1", nil, "up")
	if len(cmd.Env) != 0 {
		t.Errorf("Expected no env override, got %d vars", len(cmd.Env))
	}
}

func TestResolveComposePath(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"absolute path unchanged", "/tmp/shared-infra"},
		{"tilde path resolved", "~/shared-infra"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveComposePath(tt.path)
			if strings.HasPrefix(got, "~") {
				t.Errorf("resolveComposePath(%q) = %q, should not start with ~", tt.path, got)
			}
			if tt.path == "/tmp/shared-infra" && got != "/tmp/shared-infra" {
				t.Errorf("resolveComposePath(%q) = %q, want %q", tt.path, got, tt.path)
			}
		})
	}
}

func TestSetupWorktreeFiles(t *testing.T) {
	tmpDir := t.TempDir()

	mainPath := filepath.Join(tmpDir, "main")
	_ = os.MkdirAll(filepath.Join(mainPath, "config"), 0755)
	_ = os.WriteFile(filepath.Join(mainPath, "config", "secret.key"), []byte("secret"), 0600)
	_ = os.MkdirAll(filepath.Join(mainPath, "vendor", "bundle"), 0755)

	newPath := filepath.Join(tmpDir, "worktree")
	_ = os.MkdirAll(newPath, 0755)

	ext := &config.ExternalComposeConfig{
		CopyFiles:   []string{"config/secret.key"},
		SymlinkDirs: []string{"vendor/bundle"},
	}

	err := setupWorktreeFiles(ext, newPath, mainPath)
	if err != nil {
		t.Fatalf("setupWorktreeFiles() error = %v", err)
	}

	// Verify copied file
	data, err := os.ReadFile(filepath.Join(newPath, "config", "secret.key"))
	if err != nil {
		t.Fatalf("Failed to read copied file: %v", err)
	}
	if string(data) != "secret" {
		t.Errorf("Expected 'secret', got %q", string(data))
	}

	// Verify symlink
	info, err := os.Lstat(filepath.Join(newPath, "vendor", "bundle"))
	if err != nil {
		t.Fatalf("Failed to stat symlink: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("Expected vendor/bundle to be a symlink")
	}
}

func TestSetupWorktreeFiles_MissingSource(t *testing.T) {
	tmpDir := t.TempDir()

	mainPath := filepath.Join(tmpDir, "main")
	_ = os.MkdirAll(mainPath, 0755)

	newPath := filepath.Join(tmpDir, "worktree")
	_ = os.MkdirAll(newPath, 0755)

	ext := &config.ExternalComposeConfig{
		CopyFiles: []string{"nonexistent.key"},
	}

	err := setupWorktreeFiles(ext, newPath, mainPath)
	if err == nil {
		t.Error("Expected error for missing source file, got nil")
	}
}

func TestIsAgentMode(t *testing.T) {
	enabled := true
	disabled := false

	tests := []struct {
		name   string
		envVal string
		cfg    *config.Config
		want   bool
	}{
		{
			name:   "env not set",
			envVal: "",
			cfg: &config.Config{
				Plugins: config.PluginsConfig{
					Docker: config.DockerPluginConfig{
						Mode: "external",
						External: &config.ExternalComposeConfig{
							Agent: &config.AgentStackConfig{Enabled: &enabled},
						},
					},
				},
			},
			want: false,
		},
		{
			name:   "env set and agent enabled",
			envVal: "1",
			cfg: &config.Config{
				Plugins: config.PluginsConfig{
					Docker: config.DockerPluginConfig{
						Mode: "external",
						External: &config.ExternalComposeConfig{
							Agent: &config.AgentStackConfig{Enabled: &enabled},
						},
					},
				},
			},
			want: true,
		},
		{
			name:   "env set but agent disabled",
			envVal: "1",
			cfg: &config.Config{
				Plugins: config.PluginsConfig{
					Docker: config.DockerPluginConfig{
						Mode: "external",
						External: &config.ExternalComposeConfig{
							Agent: &config.AgentStackConfig{Enabled: &disabled},
						},
					},
				},
			},
			want: false,
		},
		{
			name:   "env set but no agent config",
			envVal: "1",
			cfg: &config.Config{
				Plugins: config.PluginsConfig{
					Docker: config.DockerPluginConfig{
						Mode:     "external",
						External: &config.ExternalComposeConfig{},
					},
				},
			},
			want: false,
		},
		{
			name:   "env set but no external config",
			envVal: "1",
			cfg: &config.Config{
				Plugins: config.PluginsConfig{
					Docker: config.DockerPluginConfig{
						Mode: "external",
					},
				},
			},
			want: false,
		},
		{
			name:   "env set but agent enabled is nil",
			envVal: "1",
			cfg: &config.Config{
				Plugins: config.PluginsConfig{
					Docker: config.DockerPluginConfig{
						Mode: "external",
						External: &config.ExternalComposeConfig{
							Agent: &config.AgentStackConfig{},
						},
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVal != "" {
				t.Setenv("GROVE_AGENT_MODE", tt.envVal)
			} else {
				t.Setenv("GROVE_AGENT_MODE", "")
			}
			got := isAgentMode(tt.cfg)
			if got != tt.want {
				t.Errorf("isAgentMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlugin_InitAgentExternalStrategy(t *testing.T) {
	t.Setenv("GROVE_AGENT_MODE", "1")

	tmpDir := t.TempDir()
	enabled := true

	plugin := New()
	cfg := &config.Config{
		ProjectsDir: "/tmp/projects",
		Plugins: config.PluginsConfig{
			Docker: config.DockerPluginConfig{
				Mode: "external",
				External: &config.ExternalComposeConfig{
					Path:     tmpDir,
					EnvVar:   "APP_DIR",
					Services: []string{"app"},
					Agent: &config.AgentStackConfig{
						Enabled:      &enabled,
						MaxSlots:     3,
						Services:     []string{"app"},
						TemplatePath: "agent-stacks/template.yml",
					},
				},
			},
		},
	}

	_ = plugin.Init(cfg)

	if plugin.strategy == nil && plugin.enabled {
		t.Error("Strategy should be set when plugin is enabled")
	}

	if plugin.enabled {
		if _, ok := plugin.strategy.(*agentExternalStrategy); !ok {
			t.Errorf("Agent mode should use agentExternalStrategy, got %T", plugin.strategy)
		}
	}
}

func TestPlugin_InitExternalStrategy_NotAgentMode(t *testing.T) {
	t.Setenv("GROVE_AGENT_MODE", "")

	tmpDir := t.TempDir()
	enabled := true

	plugin := New()
	cfg := &config.Config{
		ProjectsDir: "/tmp/projects",
		Plugins: config.PluginsConfig{
			Docker: config.DockerPluginConfig{
				Mode: "external",
				External: &config.ExternalComposeConfig{
					Path:     tmpDir,
					EnvVar:   "APP_DIR",
					Services: []string{"app"},
					Agent: &config.AgentStackConfig{
						Enabled:      &enabled,
						MaxSlots:     3,
						Services:     []string{"app"},
						TemplatePath: "agent-stacks/template.yml",
					},
				},
			},
		},
	}

	_ = plugin.Init(cfg)

	if plugin.enabled {
		if _, ok := plugin.strategy.(*externalStrategy); !ok {
			t.Errorf("Without GROVE_AGENT_MODE, should use externalStrategy, got %T", plugin.strategy)
		}
	}
}

func TestPlugin_SetIsolated(t *testing.T) {
	t.Setenv("GROVE_AGENT_MODE", "")

	tmpDir := t.TempDir()
	enabled := true

	plugin := New()
	cfg := &config.Config{
		ProjectsDir: "/tmp/projects",
		Plugins: config.PluginsConfig{
			Docker: config.DockerPluginConfig{
				Mode: "external",
				External: &config.ExternalComposeConfig{
					Path:     tmpDir,
					EnvVar:   "APP_DIR",
					Services: []string{"app"},
					Agent: &config.AgentStackConfig{
						Enabled:      &enabled,
						MaxSlots:     3,
						Services:     []string{"app"},
						TemplatePath: "agent-stacks/template.yml",
					},
				},
			},
		},
	}

	plugin.SetIsolated(true)
	_ = plugin.Init(cfg)

	if plugin.enabled {
		if !plugin.IsIsolated() {
			t.Errorf("SetIsolated(true) should use agentExternalStrategy, got %T", plugin.strategy)
		}
	}
}

func TestPlugin_IsIsolated_False(t *testing.T) {
	t.Setenv("GROVE_AGENT_MODE", "")

	plugin := New()
	cfg := &config.Config{
		ProjectsDir: "/tmp/projects",
		Plugins: config.PluginsConfig{
			Docker: config.DockerPluginConfig{
				Mode: "external",
				External: &config.ExternalComposeConfig{
					Path:     t.TempDir(),
					EnvVar:   "APP_DIR",
					Services: []string{"app"},
				},
			},
		},
	}
	_ = plugin.Init(cfg)

	if plugin.enabled && plugin.IsIsolated() {
		t.Error("IsIsolated() should be false without agent mode")
	}
}

func TestHasActiveAgentSlot(t *testing.T) {
	tmpDir := t.TempDir()
	enabled := true

	cfg := &config.Config{
		Plugins: config.PluginsConfig{
			Docker: config.DockerPluginConfig{
				Mode: "external",
				External: &config.ExternalComposeConfig{
					Path:     tmpDir,
					EnvVar:   "APP_DIR",
					Services: []string{"app"},
					Agent: &config.AgentStackConfig{
						Enabled:      &enabled,
						MaxSlots:     3,
						Services:     []string{"app"},
						TemplatePath: "agent-stacks/template.yml",
					},
				},
			},
		},
	}

	worktreePath := filepath.Join(tmpDir, "myapp-feature")

	// No slot allocated yet
	if HasActiveAgentSlot(cfg, worktreePath) {
		t.Error("HasActiveAgentSlot should be false before allocation")
	}

	// Allocate a slot
	slotsFile := filepath.Join(tmpDir, "agent-stacks", ".slots.json")
	_ = os.MkdirAll(filepath.Dir(slotsFile), 0755)
	sm := NewSlotManager(slotsFile, 3)
	_, err := sm.Allocate("myapp-feature")
	if err != nil {
		t.Fatalf("Failed to allocate slot: %v", err)
	}

	// Now should detect the active slot
	if !HasActiveAgentSlot(cfg, worktreePath) {
		t.Error("HasActiveAgentSlot should be true after allocation")
	}
}

func TestHasActiveAgentSlot_NilConfig(t *testing.T) {
	if HasActiveAgentSlot(nil, "/tmp/wt") {
		t.Error("HasActiveAgentSlot should be false for nil config")
	}
}

func TestHasActiveAgentSlot_NoAgentConfig(t *testing.T) {
	cfg := &config.Config{
		Plugins: config.PluginsConfig{
			Docker: config.DockerPluginConfig{
				Mode: "external",
				External: &config.ExternalComposeConfig{
					Path:     t.TempDir(),
					EnvVar:   "APP_DIR",
					Services: []string{"app"},
				},
			},
		},
	}

	if HasActiveAgentSlot(cfg, "/tmp/wt") {
		t.Error("HasActiveAgentSlot should be false without agent config")
	}
}

func TestSetupWorktreeFiles_SymlinkFiles(t *testing.T) {
	tmpDir := t.TempDir()

	mainPath := filepath.Join(tmpDir, "main")
	_ = os.MkdirAll(filepath.Join(mainPath, "config", "credentials"), 0755)
	_ = os.WriteFile(filepath.Join(mainPath, "config", "credentials", "dev.key"), []byte("devkey"), 0600)

	newPath := filepath.Join(tmpDir, "worktree")
	_ = os.MkdirAll(newPath, 0755)

	ext := &config.ExternalComposeConfig{
		SymlinkFiles: []string{"config/credentials/dev.key"},
	}

	err := setupWorktreeFiles(ext, newPath, mainPath)
	if err != nil {
		t.Fatalf("setupWorktreeFiles() error = %v", err)
	}

	// Verify it's a symlink, not a copy
	dst := filepath.Join(newPath, "config", "credentials", "dev.key")
	info, err := os.Lstat(dst)
	if err != nil {
		t.Fatalf("Failed to stat symlinked file: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("Expected config/credentials/dev.key to be a symlink")
	}

	// Verify content is accessible through the symlink
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("Failed to read through symlink: %v", err)
	}
	if string(data) != "devkey" {
		t.Errorf("Expected 'devkey', got %q", string(data))
	}
}

func TestSetupWorktreeFiles_AllThreeTypes(t *testing.T) {
	tmpDir := t.TempDir()

	mainPath := filepath.Join(tmpDir, "main")
	_ = os.MkdirAll(filepath.Join(mainPath, "config"), 0755)
	_ = os.WriteFile(filepath.Join(mainPath, "config", "settings.yml"), []byte("settings"), 0644)
	_ = os.WriteFile(filepath.Join(mainPath, "config", "secret.key"), []byte("secret"), 0600)
	_ = os.MkdirAll(filepath.Join(mainPath, "vendor", "bundle"), 0755)

	newPath := filepath.Join(tmpDir, "worktree")
	_ = os.MkdirAll(newPath, 0755)

	ext := &config.ExternalComposeConfig{
		CopyFiles:    []string{"config/settings.yml"},
		SymlinkFiles: []string{"config/secret.key"},
		SymlinkDirs:  []string{"vendor/bundle"},
	}

	err := setupWorktreeFiles(ext, newPath, mainPath)
	if err != nil {
		t.Fatalf("setupWorktreeFiles() error = %v", err)
	}

	// Verify copied file is a regular file
	copyInfo, err := os.Lstat(filepath.Join(newPath, "config", "settings.yml"))
	if err != nil {
		t.Fatalf("Failed to stat copied file: %v", err)
	}
	if copyInfo.Mode()&os.ModeSymlink != 0 {
		t.Error("Expected config/settings.yml to be a regular file (copy), not a symlink")
	}

	// Verify symlinked file
	fileInfo, err := os.Lstat(filepath.Join(newPath, "config", "secret.key"))
	if err != nil {
		t.Fatalf("Failed to stat symlinked file: %v", err)
	}
	if fileInfo.Mode()&os.ModeSymlink == 0 {
		t.Error("Expected config/secret.key to be a symlink")
	}

	// Verify symlinked directory
	dirInfo, err := os.Lstat(filepath.Join(newPath, "vendor", "bundle"))
	if err != nil {
		t.Fatalf("Failed to stat symlinked dir: %v", err)
	}
	if dirInfo.Mode()&os.ModeSymlink == 0 {
		t.Error("Expected vendor/bundle to be a symlink")
	}
}

func TestSetupWorktreeFiles_SymlinkFiles_MissingSource(t *testing.T) {
	tmpDir := t.TempDir()

	mainPath := filepath.Join(tmpDir, "main")
	_ = os.MkdirAll(mainPath, 0755)

	newPath := filepath.Join(tmpDir, "worktree")
	_ = os.MkdirAll(newPath, 0755)

	ext := &config.ExternalComposeConfig{
		SymlinkFiles: []string{"nonexistent.key"},
	}

	err := setupWorktreeFiles(ext, newPath, mainPath)
	if err == nil {
		t.Error("Expected error for missing source file, got nil")
	}
	if !strings.Contains(err.Error(), "source not found") {
		t.Errorf("Expected 'source not found' in error, got %q", err.Error())
	}
}

func TestPlugin_RegisterHooks_IncludesPreRemove(t *testing.T) {
	plugin := New()
	cfg := &config.Config{}
	_ = plugin.Init(cfg)

	registry := hooks.NewRegistry()
	err := plugin.RegisterHooks(registry)
	if err != nil {
		t.Errorf("RegisterHooks() error = %v", err)
	}

	// Fire pre-remove — should not error even without agent mode
	ctx := &hooks.Context{
		Worktree: "test-worktree",
		Config:   cfg,
	}
	_ = registry.Fire(hooks.EventPreRemove, ctx)
}

package docker

import (
	"bytes"
	"fmt"
	"io"
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

// captureStdout replaces os.Stdout with a pipe, runs f, then restores os.Stdout
// and returns everything written to it as a string.
func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("captureStdout: os.Pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	f()

	if err := w.Close(); err != nil {
		t.Fatalf("captureStdout: close write end: %v", err)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("captureStdout: copy: %v", err)
	}
	return buf.String()
}

func TestAgentExternalStrategy_OnPostSwitch_EmptyPath(t *testing.T) {
	cfg := newTestAgentConfig(t)
	s := newAgentExternalStrategy(cfg)

	t.Setenv("GROVE_SHELL", "1")
	out := captureStdout(t, func() {
		ctx := &hooks.Context{Worktree: "test"} // WorktreePath intentionally empty
		if err := s.OnPostSwitch(ctx); err != nil {
			t.Errorf("OnPostSwitch() error = %v, want nil", err)
		}
	})
	if out != "" {
		t.Errorf("OnPostSwitch with empty WorktreePath emitted %q, want nothing", out)
	}
}

func TestAgentExternalStrategy_OnPostSwitch_NoSlot(t *testing.T) {
	cfg := newTestAgentConfig(t)
	s := newAgentExternalStrategy(cfg)

	t.Setenv("GROVE_SHELL", "1")
	out := captureStdout(t, func() {
		ctx := &hooks.Context{
			Worktree:     "myapp-feature",
			WorktreePath: "/tmp/myapp-feature",
		}
		if err := s.OnPostSwitch(ctx); err != nil {
			t.Errorf("OnPostSwitch() error = %v, want nil", err)
		}
	})
	if out != "" {
		t.Errorf("OnPostSwitch with no allocated slot emitted %q, want nothing", out)
	}
}

func TestAgentExternalStrategy_OnPostSwitch_WithSlot(t *testing.T) {
	cfg := newTestAgentConfig(t)
	s := newAgentExternalStrategy(cfg)

	// The slots file lives under <compose-path>/agent-stacks/.slots.json — ensure
	// that directory exists before calling Allocate.
	_ = os.MkdirAll(filepath.Dir(s.slots.slotsFile), 0755)

	// Pre-allocate a slot for the worktree
	wtName := "myapp-feature"
	slot, err := s.slots.Allocate(wtName)
	if err != nil {
		t.Fatalf("Allocate() error = %v", err)
	}

	t.Setenv("GROVE_SHELL", "1")
	out := captureStdout(t, func() {
		ctx := &hooks.Context{
			Worktree:     wtName,
			WorktreePath: "/tmp/" + wtName,
		}
		if err := s.OnPostSwitch(ctx); err != nil {
			t.Errorf("OnPostSwitch() error = %v, want nil", err)
		}
	})

	wantLine := fmt.Sprintf("env:COMPOSE_PROJECT_NAME=myapp-agent-%d\n", slot)
	if out != wantLine {
		t.Errorf("OnPostSwitch emitted %q, want %q", out, wantLine)
	}
}

func TestAgentExternalStrategy_Up_EmitsEnvDirective(t *testing.T) {
	cfg := newTestAgentConfig(t)
	s := newAgentExternalStrategy(cfg)

	// Create the slots directory so Allocate can write the slots file.
	_ = os.MkdirAll(filepath.Dir(s.slots.slotsFile), 0755)

	t.Setenv("GROVE_SHELL", "1")
	worktreePath := "/tmp/myapp-up-test"

	// Up will fail because docker compose isn't available/configured, but the
	// env directive is emitted before compose runs — that's what we're testing.
	out := captureStdout(t, func() {
		_ = s.Up(worktreePath, true)
	})

	if !strings.Contains(out, "env:COMPOSE_PROJECT_NAME=myapp-agent-") {
		t.Errorf("Up() stdout = %q, want env:COMPOSE_PROJECT_NAME=myapp-agent-N line", out)
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

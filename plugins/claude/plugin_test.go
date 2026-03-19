package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/hooks"
)

func boolPtr(b bool) *bool { return &b }

func TestPluginName(t *testing.T) {
	p := New()
	if p.Name() != "claude" {
		t.Errorf("expected name 'claude', got %q", p.Name())
	}
}

func TestPluginDisabledByConfig(t *testing.T) {
	p := New()
	cfg := config.LoadDefaults()
	cfg.Plugins.Claude.Enabled = boolPtr(false)

	if err := p.Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if p.Enabled() {
		t.Error("expected plugin to be disabled")
	}
}

func TestPluginEnabledByDefault(t *testing.T) {
	p := New()
	cfg := config.LoadDefaults()
	cfg.Plugins.Claude.Enabled = boolPtr(true)

	// The devcontainer CLI won't be found in test, but plugin should handle that
	_ = p.Init(cfg)
	// Plugin will be disabled because devcontainer isn't in PATH — that's expected
}

func TestDevcontainerEnabled(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.Config
		want    bool
	}{
		{
			name: "nil config",
			cfg:  nil,
			want: false,
		},
		{
			name: "devcontainer nil",
			cfg: &config.Config{
				Plugins: config.PluginsConfig{
					Claude: config.ClaudePluginConfig{},
				},
			},
			want: true, // default: enabled
		},
		{
			name: "devcontainer enabled",
			cfg: &config.Config{
				Plugins: config.PluginsConfig{
					Claude: config.ClaudePluginConfig{
						Devcontainer: &config.ClaudeDevcontainerConfig{
							Enabled: boolPtr(true),
						},
					},
				},
			},
			want: true,
		},
		{
			name: "devcontainer disabled",
			cfg: &config.Config{
				Plugins: config.PluginsConfig{
					Claude: config.ClaudePluginConfig{
						Devcontainer: &config.ClaudeDevcontainerConfig{
							Enabled: boolPtr(false),
						},
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Plugin{cfg: tt.cfg}
			if got := p.devcontainerEnabled(); got != tt.want {
				t.Errorf("devcontainerEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestScaffoldDevcontainer(t *testing.T) {
	dir := t.TempDir()

	cfg := &config.ClaudeDevcontainerConfig{
		Enabled: boolPtr(true),
	}

	if err := scaffoldDevcontainer(dir, cfg); err != nil {
		t.Fatalf("scaffoldDevcontainer failed: %v", err)
	}

	// Check devcontainer.json was created
	dcPath := filepath.Join(dir, ".devcontainer", "devcontainer.json")
	data, err := os.ReadFile(dcPath)
	if err != nil {
		t.Fatalf("failed to read devcontainer.json: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "ghcr.io/anthropics/devcontainer-features/claude-code:1.0") {
		t.Error("devcontainer.json should reference the Anthropic devcontainer feature")
	}
	if !strings.Contains(content, "GROVE_AGENT_MODE") {
		t.Error("devcontainer.json should set GROVE_AGENT_MODE env var")
	}
}

func TestScaffoldDevcontainerWithFirewall(t *testing.T) {
	dir := t.TempDir()

	cfg := &config.ClaudeDevcontainerConfig{
		Enabled:  boolPtr(true),
		Firewall: boolPtr(true),
		AllowedDomains: []string{
			"custom.example.com",
		},
	}

	if err := scaffoldDevcontainer(dir, cfg); err != nil {
		t.Fatalf("scaffoldDevcontainer failed: %v", err)
	}

	// Check firewall script was created
	fwPath := filepath.Join(dir, ".devcontainer", "init-firewall.sh")
	data, err := os.ReadFile(fwPath)
	if err != nil {
		t.Fatalf("failed to read init-firewall.sh: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "api.anthropic.com") {
		t.Error("firewall script should include default domain api.anthropic.com")
	}
	if !strings.Contains(content, "custom.example.com") {
		t.Error("firewall script should include custom domain custom.example.com")
	}

	// Check it's executable
	info, _ := os.Stat(fwPath)
	if info.Mode()&0o111 == 0 {
		t.Error("firewall script should be executable")
	}
}

func TestMergeAllowedDomains(t *testing.T) {
	domains := mergeAllowedDomains([]string{"api.anthropic.com", "custom.example.com"})

	seen := make(map[string]bool)
	for _, d := range domains {
		if seen[d] {
			t.Errorf("duplicate domain: %s", d)
		}
		seen[d] = true
	}

	if !seen["api.anthropic.com"] {
		t.Error("should include api.anthropic.com")
	}
	if !seen["custom.example.com"] {
		t.Error("should include custom.example.com")
	}
	if !seen["github.com"] {
		t.Error("should include default domain github.com")
	}
}

func TestInjectGroveContext(t *testing.T) {
	dir := t.TempDir()

	// Test creating CLAUDE.md from scratch
	if err := injectGroveContext(dir); err != nil {
		t.Fatalf("injectGroveContext failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("failed to read CLAUDE.md: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, groveContextStart) {
		t.Error("should contain grove context start marker")
	}
	if !strings.Contains(content, "grove new") {
		t.Error("should contain grove new command")
	}
	if !strings.Contains(content, "Multi-Agent Patterns") {
		t.Error("should contain multi-agent patterns section")
	}
}

func TestInjectGroveContextIdempotent(t *testing.T) {
	dir := t.TempDir()

	// Write initial content
	initial := "# My Project\n\nSome content.\n"
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(initial), 0o644)

	// Inject twice
	injectGroveContext(dir)
	injectGroveContext(dir)

	data, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	content := string(data)

	// Should only have one occurrence
	count := strings.Count(content, groveContextStart)
	if count != 1 {
		t.Errorf("expected 1 grove context block, got %d", count)
	}

	// Should preserve original content
	if !strings.Contains(content, "# My Project") {
		t.Error("should preserve original content")
	}
}

func TestRegisterHooks(t *testing.T) {
	p := New()
	registry := hooks.NewRegistry()

	if err := p.RegisterHooks(registry); err != nil {
		t.Fatalf("RegisterHooks failed: %v", err)
	}
}

func TestGeneratePermissionSettings(t *testing.T) {
	dir := t.TempDir()

	perms := &config.ClaudePermissionsConfig{
		AllowedTools: []string{"Bash", "Read", "Write"},
		AllowedMCPs:  []string{"github"},
		MaxTurns:     50,
	}

	if err := generatePermissionSettings(dir, perms); err != nil {
		t.Fatalf("generatePermissionSettings failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "claude-settings.json"))
	if err != nil {
		t.Fatalf("failed to read claude-settings.json: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "Bash") {
		t.Error("should contain allowed tool 'Bash'")
	}
	if !strings.Contains(content, "github") {
		t.Error("should contain allowed MCP 'github'")
	}
	if !strings.Contains(content, "50") {
		t.Error("should contain maxTurns 50")
	}
}

func TestSandboxContainerName(t *testing.T) {
	name := sandboxContainerName("/home/user/projects/grove-testing")
	if name != "grove-sandbox-grove-testing" {
		t.Errorf("expected 'grove-sandbox-grove-testing', got %q", name)
	}
}

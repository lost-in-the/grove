package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandConfigPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}

	tests := []struct {
		name    string
		path    string
		baseDir string
		want    string
	}{
		{"empty unchanged", "", "/anywhere", ""},
		{"absolute cleaned", "/tmp//foo/../bar", "/ignored", "/tmp/bar"},
		{"tilde expands", "~/projects/orchestrator", "/ignored", filepath.Join(home, "projects", "orchestrator")},
		{"bare tilde expands", "~", "/ignored", home},
		{"relative joins baseDir", "../orchestrator", "/home/user/code/app", "/home/user/code/orchestrator"},
		{"dot relative joins baseDir", "./shared-infra", "/home/user/code/app", "/home/user/code/app/shared-infra"},
		{"plain relative joins baseDir", "shared-infra", "/home/user/code/app", "/home/user/code/app/shared-infra"},
		{"relative without baseDir cleaned", "../orchestrator", "", "../orchestrator"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := expandConfigPath(tt.path, tt.baseDir)
			if err != nil {
				t.Fatalf("expandConfigPath(%q, %q) error = %v", tt.path, tt.baseDir, err)
			}
			if got != tt.want {
				t.Errorf("expandConfigPath(%q, %q) = %q, want %q", tt.path, tt.baseDir, got, tt.want)
			}
		})
	}
}

func TestProjectRootFor(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"project config", "/home/user/code/app/.grove/config.toml", "/home/user/code/app"},
		{"global config", "/home/user/.config/grove/config.toml", "/home/user/.config"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := projectRootFor(tt.path)
			if got != tt.want {
				t.Errorf("projectRootFor(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestResolveProjectPaths_RelativeExternalPath(t *testing.T) {
	cfg := &Config{
		Plugins: PluginsConfig{
			Docker: DockerPluginConfig{
				External: &ExternalComposeConfig{
					Path: "../orchestrator",
				},
			},
		},
	}

	if err := resolveProjectPaths(cfg, "/home/user/code/app"); err != nil {
		t.Fatalf("resolveProjectPaths() error = %v", err)
	}

	got := cfg.Plugins.Docker.External.Path
	want := "/home/user/code/orchestrator"
	if got != want {
		t.Errorf("External.Path = %q, want %q", got, want)
	}
}

func TestResolveProjectPaths_AbsolutePathUnchanged(t *testing.T) {
	cfg := &Config{
		Plugins: PluginsConfig{
			Docker: DockerPluginConfig{
				External: &ExternalComposeConfig{
					Path: "/abs/path/to/orchestrator",
				},
			},
		},
	}

	if err := resolveProjectPaths(cfg, "/home/user/code/app"); err != nil {
		t.Fatalf("resolveProjectPaths() error = %v", err)
	}

	got := cfg.Plugins.Docker.External.Path
	want := "/abs/path/to/orchestrator"
	if got != want {
		t.Errorf("External.Path = %q, want %q", got, want)
	}
}

func TestResolveProjectPaths_NoExternal(t *testing.T) {
	cfg := &Config{}
	if err := resolveProjectPaths(cfg, "/anywhere"); err != nil {
		t.Errorf("resolveProjectPaths() with no external should succeed, got %v", err)
	}
}

func TestResolveProjectPaths_TildeTemplatePath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}

	cfg := &Config{
		Plugins: PluginsConfig{
			Docker: DockerPluginConfig{
				External: &ExternalComposeConfig{
					Path: "/abs/orchestrator",
					Agent: &AgentStackConfig{
						TemplatePath: "~/work/agent.yml",
					},
				},
			},
		},
	}

	if err := resolveProjectPaths(cfg, "/abs/orchestrator"); err != nil {
		t.Fatalf("resolveProjectPaths() error = %v", err)
	}

	got := cfg.Plugins.Docker.External.Agent.TemplatePath
	want := filepath.Join(home, "work", "agent.yml")
	if got != want {
		t.Errorf("Agent.TemplatePath = %q, want %q", got, want)
	}
}

func TestResolveProjectPaths_TildeTemplateOverlays(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}

	cfg := &Config{
		Plugins: PluginsConfig{
			Docker: DockerPluginConfig{
				External: &ExternalComposeConfig{
					Path: "/abs/orchestrator",
					Agent: &AgentStackConfig{
						TemplatePath: "agent-stacks/template.yml",
						TemplateOverlays: []string{
							"~/work/overlay-a.yml",
							"overlay-b.yml",
							"/abs/overlay-c.yml",
						},
					},
				},
			},
		},
	}

	if err := resolveProjectPaths(cfg, "/abs/orchestrator"); err != nil {
		t.Fatalf("resolveProjectPaths() error = %v", err)
	}

	overlays := cfg.Plugins.Docker.External.Agent.TemplateOverlays
	want := []string{
		filepath.Join(home, "work", "overlay-a.yml"),
		"overlay-b.yml",
		"/abs/overlay-c.yml",
	}
	if len(overlays) != len(want) {
		t.Fatalf("TemplateOverlays length = %d, want %d", len(overlays), len(want))
	}
	for i, w := range want {
		if overlays[i] != w {
			t.Errorf("TemplateOverlays[%d] = %q, want %q", i, overlays[i], w)
		}
	}
}

func TestResolveProjectPaths_TemplatePathNonTildeUnchanged(t *testing.T) {
	cfg := &Config{
		Plugins: PluginsConfig{
			Docker: DockerPluginConfig{
				External: &ExternalComposeConfig{
					Path: "/abs/orchestrator",
					Agent: &AgentStackConfig{
						TemplatePath: "agent-stacks/template.yml",
					},
				},
			},
		},
	}

	if err := resolveProjectPaths(cfg, "/abs/orchestrator"); err != nil {
		t.Fatalf("resolveProjectPaths() error = %v", err)
	}

	got := cfg.Plugins.Docker.External.Agent.TemplatePath
	want := "agent-stacks/template.yml"
	if got != want {
		t.Errorf("Agent.TemplatePath = %q, want %q (relative paths preserved for compose-dir resolution)", got, want)
	}
}

// TestLoadConfigFromPaths_RelativeExternalPath end-to-end tests that a project
// config with a relative external.path resolves against the project root
// (parent of .grove/) when loaded via the normal config loading flow.
func TestLoadConfigFromPaths_RelativeExternalPath(t *testing.T) {
	tmpDir := t.TempDir()

	orchestratorDir := filepath.Join(tmpDir, "orchestrator")
	appDir := filepath.Join(orchestratorDir, "app")
	groveDir := filepath.Join(appDir, ".grove")

	if err := os.MkdirAll(orchestratorDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(groveDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configContent := `
[plugins.docker]
mode = "external"

[plugins.docker.external]
path = "../"
env_var = "APP_DIR"
services = ["app"]
`
	configPath := filepath.Join(groveDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadFromPaths(LoadDefaults(), "/nonexistent/global.toml", configPath, "")
	if err != nil {
		t.Fatalf("loadFromPaths() error = %v", err)
	}

	got := cfg.Plugins.Docker.External.Path
	want, err := filepath.EvalSymlinks(orchestratorDir)
	if err != nil {
		want = orchestratorDir
	}
	gotResolved, err := filepath.EvalSymlinks(got)
	if err != nil {
		gotResolved = got
	}
	if gotResolved != want {
		t.Errorf("External.Path = %q (resolved %q), want %q", got, gotResolved, want)
	}
}

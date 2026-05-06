package docker

import (
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/config"
)

func TestExternalRun_DefaultUsesNoDeps(t *testing.T) {
	cfg := &config.Config{
		Plugins: config.PluginsConfig{
			Docker: config.DockerPluginConfig{
				External: &config.ExternalComposeConfig{
					Path:     "/tmp/compose",
					EnvVar:   "APP_DIR",
					Services: []string{"app"},
				},
			},
		},
		Test: config.TestConfig{Command: "bin/rspec", Service: "app"},
	}

	args := buildRunArgs(cfg, "/tmp/wt", "app", "bin/rspec")

	found := false
	for _, a := range args {
		if a == "--no-deps" {
			found = true
		}
		if a == "-v" {
			t.Errorf("expected no -v flag (no bind_mount configured), got: %v", args)
		}
	}
	if !found {
		t.Errorf("expected --no-deps in args, got: %v", args)
	}
}

func TestExternalRun_IncludeDepsTrueOmitsNoDeps(t *testing.T) {
	trueVal := true
	cfg := &config.Config{
		Plugins: config.PluginsConfig{
			Docker: config.DockerPluginConfig{
				External: &config.ExternalComposeConfig{
					Path:     "/tmp/compose",
					EnvVar:   "APP_DIR",
					Services: []string{"app"},
				},
			},
		},
		Test: config.TestConfig{
			Command:     "bin/rspec",
			Service:     "app",
			IncludeDeps: &trueVal,
		},
	}

	args := buildRunArgs(cfg, "/tmp/wt", "app", "bin/rspec")

	for _, a := range args {
		if a == "--no-deps" {
			t.Errorf("expected --no-deps to be omitted, got: %v", args)
		}
	}
}

func TestExternalRun_BindMountAddsVolumeFlag(t *testing.T) {
	cfg := &config.Config{
		Plugins: config.PluginsConfig{
			Docker: config.DockerPluginConfig{
				External: &config.ExternalComposeConfig{
					Path:     "/tmp/compose",
					EnvVar:   "APP_DIR",
					Services: []string{"app"},
				},
			},
		},
		Test: config.TestConfig{
			Command:   "bin/rspec",
			Service:   "app",
			BindMount: "/app",
		},
	}

	args := buildRunArgs(cfg, "/tmp/wt", "app", "bin/rspec")

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-v /tmp/wt:/app") {
		t.Errorf("expected -v /tmp/wt:/app, got: %v", args)
	}
}

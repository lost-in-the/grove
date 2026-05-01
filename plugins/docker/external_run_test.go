package docker

import (
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/config"
)

// captureRunArgs intercepts the compose command construction so we can assert
// on the arg list without actually invoking docker.
func captureRunArgs(t *testing.T, cfg *config.Config, worktreePath, service, command string) []string {
	t.Helper()
	s := newExternalStrategy(cfg)
	args := s.buildRunArgs(worktreePath, service, command)
	return args
}

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

	args := captureRunArgs(t, cfg, "/tmp/wt", "app", "bin/rspec")

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
			IncludeDeps: true,
		},
	}

	args := captureRunArgs(t, cfg, "/tmp/wt", "app", "bin/rspec")

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

	args := captureRunArgs(t, cfg, "/tmp/wt", "app", "bin/rspec")

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-v /tmp/wt:/app") {
		t.Errorf("expected -v /tmp/wt:/app, got: %v", args)
	}
}

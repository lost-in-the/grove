package docker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lost-in-the/grove/internal/config"
)

func TestLocalStrategy_GetAutoStart_ExplicitValues(t *testing.T) {
	trueVal := true
	falseVal := false

	tests := []struct {
		name string
		cfg  *config.Config
		want bool
	}{
		{
			name: "explicit true",
			cfg: &config.Config{
				Plugins: config.PluginsConfig{
					Docker: config.DockerPluginConfig{
						AutoStart: &trueVal,
					},
				},
			},
			want: true,
		},
		{
			name: "explicit false",
			cfg: &config.Config{
				Plugins: config.PluginsConfig{
					Docker: config.DockerPluginConfig{
						AutoStart: &falseVal,
					},
				},
			},
			want: false,
		},
		{
			name: "nil AutoStart defaults to true",
			cfg:  &config.Config{},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &localStrategy{cfg: tt.cfg}
			if got := s.getAutoStart(); got != tt.want {
				t.Errorf("getAutoStart() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLocalStrategy_GetAutoStop_ExplicitValues(t *testing.T) {
	trueVal := true
	falseVal := false

	tests := []struct {
		name string
		cfg  *config.Config
		want bool
	}{
		{
			name: "explicit true",
			cfg: &config.Config{
				Plugins: config.PluginsConfig{
					Docker: config.DockerPluginConfig{
						AutoStop: &trueVal,
					},
				},
			},
			want: true,
		},
		{
			name: "explicit false",
			cfg: &config.Config{
				Plugins: config.PluginsConfig{
					Docker: config.DockerPluginConfig{
						AutoStop: &falseVal,
					},
				},
			},
			want: false,
		},
		{
			// AutoStop nil defaults to false (opposite of AutoStart which defaults to true)
			name: "nil AutoStop defaults to false",
			cfg:  &config.Config{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &localStrategy{cfg: tt.cfg}
			if got := s.getAutoStop(); got != tt.want {
				t.Errorf("getAutoStop() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLocalRun_DefaultUsesNoDeps(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "docker-compose.yml"), []byte("services:\n  app: {}\n"), 0644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	cfg := &config.Config{
		Test: config.TestConfig{Command: "bin/rspec", Service: "app"},
	}
	s := newLocalStrategy(cfg)
	args := s.buildRunArgs(tmpDir, "app", "bin/rspec")

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

func TestLocalRun_IncludeDepsTrueOmitsNoDeps(t *testing.T) {
	cfg := &config.Config{
		Test: config.TestConfig{
			Command:     "bin/rspec",
			Service:     "app",
			IncludeDeps: true,
		},
	}
	s := newLocalStrategy(cfg)
	args := s.buildRunArgs("/tmp", "app", "bin/rspec")

	for _, a := range args {
		if a == "--no-deps" {
			t.Errorf("expected --no-deps omitted, got: %v", args)
		}
	}
}

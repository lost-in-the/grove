package docker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/hooks"
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
	args := buildRunArgs(cfg, tmpDir, "app", "bin/rspec", nil)

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
	trueVal := true
	cfg := &config.Config{
		Test: config.TestConfig{
			Command:     "bin/rspec",
			Service:     "app",
			IncludeDeps: &trueVal,
		},
	}
	args := buildRunArgs(cfg, "/tmp", "app", "bin/rspec", nil)

	for _, a := range args {
		if a == "--no-deps" {
			t.Errorf("expected --no-deps omitted, got: %v", args)
		}
	}
}

// TestComposeEnvFileArgs locks the behavior introduced for issue #98: when a
// project configures env_file to something other than ".env", grove must still
// layer ".env" underneath so compose v2 can interpolate variables defined only
// in the committed defaults file.
func TestComposeEnvFileArgs(t *testing.T) {
	tests := []struct {
		name      string
		envFile   string
		writeEnv  bool // whether a .env file exists in composePath
		expectArg []string
	}{
		{
			name:      "empty envFile returns no args",
			envFile:   "",
			writeEnv:  true,
			expectArg: nil,
		},
		{
			name:      "envFile equals .env returns no args (compose default)",
			envFile:   ".env",
			writeEnv:  true,
			expectArg: nil,
		},
		{
			name:      "envFile=.env.local with .env present layers both",
			envFile:   ".env.local",
			writeEnv:  true,
			expectArg: []string{"--env-file", ".env", "--env-file", ".env.local"},
		},
		{
			name:      "envFile=.env.local without .env passes only configured",
			envFile:   ".env.local",
			writeEnv:  false,
			expectArg: []string{"--env-file", ".env.local"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.writeEnv {
				if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("FOO=bar\n"), 0o644); err != nil {
					t.Fatalf("write .env: %v", err)
				}
			}

			got := composeEnvFileArgs(dir, tt.envFile)
			if len(got) != len(tt.expectArg) {
				t.Fatalf("expected %v (len %d), got %v (len %d)", tt.expectArg, len(tt.expectArg), got, len(got))
			}
			for i := range got {
				if got[i] != tt.expectArg[i] {
					t.Errorf("args[%d]: expected %q, got %q (full: %v)", i, tt.expectArg[i], got[i], got)
				}
			}
		})
	}
}

// Two worktrees can share a name when one lives outside projects_dir. The
// switch hooks must act on the exact paths they are given, and a switch must
// not wait on a foreground `docker compose up`.
func TestLocalStrategy_SwitchHooksUseExactPathsAndDetach(t *testing.T) {
	inProjects := filepath.Join(t.TempDir(), "feature")
	elsewhere := filepath.Join(t.TempDir(), "feature")
	for _, dir := range []string{inProjects, elsewhere} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	yes := true
	cfg := &config.Config{ProjectsDir: filepath.Dir(inProjects)}
	cfg.Switch.ContainerSwitch = "auto"
	cfg.Plugins.Docker.AutoStart = &yes
	cfg.Plugins.Docker.AutoStop = &yes

	var upPath, downPath string
	var upDetach bool
	s := &localStrategy{
		cfg:    cfg,
		upFn:   func(path string, detach bool) error { upPath, upDetach = path, detach; return nil },
		downFn: func(path string) error { downPath = path; return nil },
	}

	ctx := &hooks.Context{
		Worktree:         "feature",
		WorktreePath:     elsewhere,
		PrevWorktree:     "feature",
		PrevWorktreePath: elsewhere,
		Config:           cfg,
	}
	if err := s.OnPreSwitch(ctx); err != nil {
		t.Fatalf("OnPreSwitch: %v", err)
	}
	if err := s.OnPostSwitch(ctx); err != nil {
		t.Fatalf("OnPostSwitch: %v", err)
	}

	if downPath != elsewhere {
		t.Errorf("stopped %q, want the exact previous path %q", downPath, elsewhere)
	}
	if upPath != elsewhere {
		t.Errorf("started %q, want the exact target path %q", upPath, elsewhere)
	}
	if !upDetach {
		t.Error("a switch started the stack in the foreground, want detached")
	}
}

package docker

import (
	"os"
	"path/filepath"
	"strings"
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

func TestPlugin_InitLocalStrategy(t *testing.T) {
	plugin := New()
	cfg := &config.Config{
		ProjectsDir: "/tmp/projects",
	}

	_ = plugin.Init(cfg)

	if plugin.strategy == nil && plugin.enabled {
		t.Error("Strategy should be set when plugin is enabled")
	}

	if plugin.enabled {
		if _, ok := plugin.strategy.(*localStrategy); !ok {
			t.Error("Default strategy should be localStrategy")
		}
	}
}

func TestPlugin_InitExternalStrategy(t *testing.T) {
	tmpDir := t.TempDir()

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
				},
			},
		},
	}

	_ = plugin.Init(cfg)

	if plugin.strategy == nil && plugin.enabled {
		t.Error("Strategy should be set when plugin is enabled")
	}

	if plugin.enabled {
		if _, ok := plugin.strategy.(*externalStrategy); !ok {
			t.Errorf("External mode should use externalStrategy, got %T", plugin.strategy)
		}
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
	_ = registry.Fire(hooks.EventPostCreate, ctx)
}

func TestHasDockerCompose(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "grove-docker-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{name: "docker-compose.yml exists", filename: "docker-compose.yml", want: true},
		{name: "docker-compose.yaml exists", filename: "docker-compose.yaml", want: true},
		{name: "compose.yml exists", filename: "compose.yml", want: true},
		{name: "compose.yaml exists", filename: "compose.yaml", want: true},
		{name: "no compose file", filename: "", want: false},
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

			got := hasDockerCompose(testDir)
			if got != tt.want {
				t.Errorf("hasDockerCompose() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLocalStrategy_GetWorktreePath(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.Config
		worktree    string
		wantContain string
	}{
		{
			name:        "absolute path",
			config:      &config.Config{ProjectsDir: "/tmp/projects"},
			worktree:    "/absolute/path/to/worktree",
			wantContain: "/absolute/path/to/worktree",
		},
		{
			name:        "relative with projects dir",
			config:      &config.Config{ProjectsDir: "/tmp/projects"},
			worktree:    "my-worktree",
			wantContain: "my-worktree",
		},
		{
			name:        "empty worktree",
			config:      &config.Config{ProjectsDir: "/tmp/projects"},
			worktree:    "",
			wantContain: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newLocalStrategy(tt.config)
			got := s.getWorktreePath(tt.worktree)
			if tt.wantContain != "" && got == "" {
				t.Errorf("getWorktreePath() returned empty string")
			}
			if tt.wantContain == "" && got != "" {
				t.Errorf("getWorktreePath() = %v, want empty", got)
			}
		})
	}
}

func TestLocalStrategy_GetAutoStart(t *testing.T) {
	s := newLocalStrategy(nil)
	if !s.getAutoStart() {
		t.Error("getAutoStart() should default to true")
	}
}

func TestLocalStrategy_GetAutoStop(t *testing.T) {
	s := newLocalStrategy(nil)
	if s.getAutoStop() {
		t.Error("getAutoStop() should default to false for local mode")
	}
}

func TestExternalStrategy_GetAutoStop(t *testing.T) {
	s := newExternalStrategy(&config.Config{
		Plugins: config.PluginsConfig{
			Docker: config.DockerPluginConfig{
				Mode: "external",
				External: &config.ExternalComposeConfig{
					Path:     "/tmp/shared-infra",
					EnvVar:   "APP_DIR",
					Services: []string{"app"},
				},
			},
		},
	})
	if !s.getAutoStop() {
		t.Error("getAutoStop() should default to true for external mode")
	}
}

func TestExternalStrategy_GetAutoStart(t *testing.T) {
	s := newExternalStrategy(&config.Config{
		Plugins: config.PluginsConfig{
			Docker: config.DockerPluginConfig{
				Mode: "external",
				External: &config.ExternalComposeConfig{
					Path:     "/tmp/shared-infra",
					EnvVar:   "APP_DIR",
					Services: []string{"app"},
				},
			},
		},
	})
	if !s.getAutoStart() {
		t.Error("getAutoStart() should default to true for external mode")
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

func TestPlugin_Up(t *testing.T) {
	plugin := New()

	tmpDir, err := os.MkdirTemp("", "grove-docker-up-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	tests := []struct {
		name        string
		createFile  bool
		wantErr     bool
		errContains string
	}{
		{
			name:        "no compose file",
			createFile:  false,
			wantErr:     true,
			errContains: "no docker-compose file found",
		},
		{
			name:       "with compose file",
			createFile: true,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := filepath.Join(tmpDir, tt.name)
			if err := os.MkdirAll(testDir, 0755); err != nil {
				t.Fatal(err)
			}

			if tt.createFile {
				file := filepath.Join(testDir, "docker-compose.yml")
				if err := os.WriteFile(file, []byte("version: '3'\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}

			cfg := &config.Config{ProjectsDir: "/tmp/projects"}
			_ = plugin.Init(cfg)

			err := plugin.Up(testDir, true)
			if tt.wantErr {
				if err == nil {
					t.Error("Up() expected error but got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Up() error = %v, want error containing %s", err, tt.errContains)
				}
			}
		})
	}
}

func TestPlugin_Down(t *testing.T) {
	plugin := New()

	tmpDir, err := os.MkdirTemp("", "grove-docker-down-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	tests := []struct {
		name        string
		createFile  bool
		wantErr     bool
		errContains string
	}{
		{
			name:        "no compose file",
			createFile:  false,
			wantErr:     true,
			errContains: "no docker-compose file found",
		},
		{
			name:       "with compose file",
			createFile: true,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := filepath.Join(tmpDir, tt.name)
			if err := os.MkdirAll(testDir, 0755); err != nil {
				t.Fatal(err)
			}

			if tt.createFile {
				file := filepath.Join(testDir, "docker-compose.yml")
				if err := os.WriteFile(file, []byte("version: '3'\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}

			cfg := &config.Config{ProjectsDir: "/tmp/projects"}
			_ = plugin.Init(cfg)

			err := plugin.Down(testDir)
			if tt.wantErr {
				if err == nil {
					t.Error("Down() expected error but got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Down() error = %v, want error containing %s", err, tt.errContains)
				}
			}
		})
	}
}

func TestComposeCommand(t *testing.T) {
	tests := []struct {
		name         string
		worktreePath string
		envFile      string
		env          []string
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
		{
			name:         "with env vars",
			worktreePath: "/tmp/test",
			env:          []string{"APP_DIR=./myapp-feature"},
			args:         []string{"up", "-d", "app"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := composeCommand(tt.worktreePath, tt.envFile, tt.env, tt.args...)
			if cmd == nil {
				t.Fatal("composeCommand() returned nil")
			}
			if cmd.Dir != tt.worktreePath {
				t.Errorf("Command Dir = %s, want %s", cmd.Dir, tt.worktreePath)
			}
			if len(tt.env) > 0 && len(cmd.Env) == 0 {
				t.Error("Expected env vars to be set")
			}
		})
	}
}

func TestComposeCommand_EnvFile(t *testing.T) {
	tests := []struct {
		name        string
		envFile     string
		wantEnvFile bool
	}{
		{
			name:        "empty env file does not add --env-file",
			envFile:     "",
			wantEnvFile: false,
		},
		{
			name:        ".env does not add --env-file",
			envFile:     ".env",
			wantEnvFile: false,
		},
		{
			name:        ".env.local adds --env-file",
			envFile:     ".env.local",
			wantEnvFile: true,
		},
		{
			name:        "custom env file adds --env-file",
			envFile:     ".env.grove",
			wantEnvFile: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := composeCommand("/tmp/test", tt.envFile, nil, "up", "-d")
			if cmd == nil {
				t.Fatal("composeCommand() returned nil")
			}

			args := strings.Join(cmd.Args, " ")
			hasEnvFile := strings.Contains(args, "--env-file")
			if hasEnvFile != tt.wantEnvFile {
				t.Errorf("--env-file in args = %v, want %v (args: %s)", hasEnvFile, tt.wantEnvFile, args)
			}

			if tt.wantEnvFile {
				if !strings.Contains(args, "--env-file "+tt.envFile) {
					t.Errorf("expected --env-file %s in args: %s", tt.envFile, args)
				}
			}
		})
	}
}

func TestExternalComposeConfig_EnvFileName(t *testing.T) {
	tests := []struct {
		name    string
		envFile string
		want    string
	}{
		{name: "default when empty", envFile: "", want: ".env"},
		{name: "custom env file", envFile: ".env.local", want: ".env.local"},
		{name: "explicit .env", envFile: ".env", want: ".env"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext := &config.ExternalComposeConfig{EnvFile: tt.envFile}
			got := ext.EnvFileName()
			if got != tt.want {
				t.Errorf("EnvFileName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExternalStrategy_RelativeWorktreePath(t *testing.T) {
	s := newExternalStrategy(&config.Config{
		Plugins: config.PluginsConfig{
			Docker: config.DockerPluginConfig{
				Mode: "external",
				External: &config.ExternalComposeConfig{
					Path:     "/home/dev/shared-infra",
					EnvVar:   "APP_DIR",
					Services: []string{"app"},
				},
			},
		},
	})

	tests := []struct {
		name    string
		absPath string
		want    string
	}{
		{
			name:    "sibling directory",
			absPath: "/home/dev/shared-infra/myapp-feature-x",
			want:    "./myapp-feature-x",
		},
		{
			name:    "subdirectory",
			absPath: "/home/dev/shared-infra/myapp",
			want:    "./myapp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.relativeWorktreePath(tt.absPath)
			if got != tt.want {
				t.Errorf("relativeWorktreePath(%q) = %q, want %q", tt.absPath, got, tt.want)
			}
		})
	}
}

func TestExternalStrategy_EnvForWorktree(t *testing.T) {
	s := newExternalStrategy(&config.Config{
		Plugins: config.PluginsConfig{
			Docker: config.DockerPluginConfig{
				Mode: "external",
				External: &config.ExternalComposeConfig{
					Path:     "/home/dev/shared-infra",
					EnvVar:   "APP_DIR",
					Services: []string{"app"},
				},
			},
		},
	})

	env := s.envForWorktree("/home/dev/shared-infra/myapp-feature-x")
	if len(env) != 1 {
		t.Fatalf("Expected 1 env var, got %d", len(env))
	}
	if env[0] != "APP_DIR=./myapp-feature-x" {
		t.Errorf("Expected APP_DIR=./myapp-feature-x, got %s", env[0])
	}
}

func TestExternalStrategy_PersistEnvVar(t *testing.T) {
	tests := []struct {
		name        string
		existing    string // existing .env content ("" means no file)
		worktree    string // relative to tmpDir (compose path)
		wantContain string
		wantLines   int // expected non-empty line count (0 = don't check)
	}{
		{
			name:        "no existing .env file",
			worktree:    "myapp-feature-x",
			wantContain: "APP_DIR=./myapp-feature-x",
		},
		{
			name:        "existing .env without env var",
			existing:    "USER\nDEPLOYER\n",
			worktree:    "myapp-feature-x",
			wantContain: "APP_DIR=./myapp-feature-x",
			wantLines:   3,
		},
		{
			name:        "existing .env with env var updates in place",
			existing:    "USER\nAPP_DIR=./myapp\nDEPLOYER\n",
			worktree:    "myapp-feature-x",
			wantContain: "APP_DIR=./myapp-feature-x",
			wantLines:   3,
		},
		{
			name:        "preserves other variables",
			existing:    "USER\nKNIFE_HOME=/app/.chef\n",
			worktree:    "myapp",
			wantContain: "KNIFE_HOME=/app/.chef",
		},
		{
			name:        "switching back to main",
			existing:    "APP_DIR=./myapp-feature-x\n",
			worktree:    "myapp",
			wantContain: "APP_DIR=./myapp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			envFile := filepath.Join(tmpDir, ".env")

			if tt.existing != "" {
				if err := os.WriteFile(envFile, []byte(tt.existing), 0644); err != nil {
					t.Fatal(err)
				}
			}

			s := newExternalStrategy(&config.Config{
				Plugins: config.PluginsConfig{
					Docker: config.DockerPluginConfig{
						Mode: "external",
						External: &config.ExternalComposeConfig{
							Path:     tmpDir,
							EnvVar:   "APP_DIR",
							Services: []string{"app"},
						},
					},
				},
			})

			worktreePath := filepath.Join(tmpDir, tt.worktree)
			err := s.persistEnvVar(worktreePath)
			if err != nil {
				t.Fatalf("persistEnvVar() error = %v", err)
			}

			data, err := os.ReadFile(envFile)
			if err != nil {
				t.Fatalf("Failed to read .env: %v", err)
			}

			content := string(data)
			if !strings.Contains(content, tt.wantContain) {
				t.Errorf(".env content = %q, want to contain %q", content, tt.wantContain)
			}

			if tt.wantLines > 0 {
				nonEmpty := 0
				for _, line := range strings.Split(content, "\n") {
					if line != "" {
						nonEmpty++
					}
				}
				if nonEmpty != tt.wantLines {
					t.Errorf("Expected %d non-empty lines, got %d in:\n%s", tt.wantLines, nonEmpty, content)
				}
			}
		})
	}
}

func TestExternalStrategy_PersistEnvVar_NoDoubleEntry(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")

	s := newExternalStrategy(&config.Config{
		Plugins: config.PluginsConfig{
			Docker: config.DockerPluginConfig{
				Mode: "external",
				External: &config.ExternalComposeConfig{
					Path:     tmpDir,
					EnvVar:   "APP_DIR",
					Services: []string{"app"},
				},
			},
		},
	})

	// Persist twice — should not duplicate
	_ = s.persistEnvVar(filepath.Join(tmpDir, "myapp-feature-x"))
	_ = s.persistEnvVar(filepath.Join(tmpDir, "myapp-feature-y"))

	data, _ := os.ReadFile(envFile)
	count := strings.Count(string(data), "APP_DIR=")
	if count != 1 {
		t.Errorf("Expected exactly 1 APP_DIR entry, got %d in:\n%s", count, string(data))
	}

	if !strings.Contains(string(data), "APP_DIR=./myapp-feature-y") {
		t.Errorf("Expected final value to be myapp-feature-y, got:\n%s", string(data))
	}
}

func TestExternalStrategy_PersistEnvVar_CustomEnvFile(t *testing.T) {
	tmpDir := t.TempDir()

	s := newExternalStrategy(&config.Config{
		Plugins: config.PluginsConfig{
			Docker: config.DockerPluginConfig{
				Mode: "external",
				External: &config.ExternalComposeConfig{
					Path:     tmpDir,
					EnvVar:   "APP_DIR",
					EnvFile:  ".env.local",
					Services: []string{"app"},
				},
			},
		},
	})

	worktreePath := filepath.Join(tmpDir, "myapp-feature-x")
	err := s.persistEnvVar(worktreePath)
	if err != nil {
		t.Fatalf("persistEnvVar() error = %v", err)
	}

	// Should write to .env.local, not .env
	localData, err := os.ReadFile(filepath.Join(tmpDir, ".env.local"))
	if err != nil {
		t.Fatalf("Failed to read .env.local: %v", err)
	}
	if !strings.Contains(string(localData), "APP_DIR=./myapp-feature-x") {
		t.Errorf(".env.local content = %q, want APP_DIR=./myapp-feature-x", string(localData))
	}

	// .env should NOT exist
	if _, err := os.Stat(filepath.Join(tmpDir, ".env")); !os.IsNotExist(err) {
		t.Error("Expected .env to not exist when env_file is .env.local")
	}
}

func TestExternalStrategy_PersistEnvVar_SkipsUnchanged(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")

	// Pre-populate with the value we'll persist
	if err := os.WriteFile(envFile, []byte("APP_DIR=./myapp-feature-x\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Record modification time
	infoBefore, _ := os.Stat(envFile)

	s := newExternalStrategy(&config.Config{
		Plugins: config.PluginsConfig{
			Docker: config.DockerPluginConfig{
				Mode: "external",
				External: &config.ExternalComposeConfig{
					Path:     tmpDir,
					EnvVar:   "APP_DIR",
					Services: []string{"app"},
				},
			},
		},
	})

	err := s.persistEnvVar(filepath.Join(tmpDir, "myapp-feature-x"))
	if err != nil {
		t.Fatalf("persistEnvVar() error = %v", err)
	}

	// File should not have been rewritten
	infoAfter, _ := os.Stat(envFile)
	if !infoBefore.ModTime().Equal(infoAfter.ModTime()) {
		t.Error("Expected file to not be rewritten when value is unchanged")
	}
}

func TestExternalStrategy_RemoveEnvVar(t *testing.T) {
	tests := []struct {
		name        string
		existing    string
		worktree    string
		wantContent string
	}{
		{
			name:        "removes matching entry",
			existing:    "OTHER=value\nAPP_DIR=./myapp-feature-x\nMORE=stuff\n",
			worktree:    "myapp-feature-x",
			wantContent: "OTHER=value\nMORE=stuff\n",
		},
		{
			name:        "leaves non-matching entry",
			existing:    "APP_DIR=./myapp-main\n",
			worktree:    "myapp-feature-x",
			wantContent: "APP_DIR=./myapp-main\n",
		},
		{
			name:        "handles missing file gracefully",
			existing:    "", // no file
			worktree:    "myapp-feature-x",
			wantContent: "", // still no file
		},
		{
			name:        "removes only entry leaves empty file",
			existing:    "APP_DIR=./myapp-feature-x\n",
			worktree:    "myapp-feature-x",
			wantContent: "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			envFile := filepath.Join(tmpDir, ".env")

			if tt.existing != "" {
				if err := os.WriteFile(envFile, []byte(tt.existing), 0644); err != nil {
					t.Fatal(err)
				}
			}

			s := newExternalStrategy(&config.Config{
				Plugins: config.PluginsConfig{
					Docker: config.DockerPluginConfig{
						Mode: "external",
						External: &config.ExternalComposeConfig{
							Path:     tmpDir,
							EnvVar:   "APP_DIR",
							Services: []string{"app"},
						},
					},
				},
			})

			worktreePath := filepath.Join(tmpDir, tt.worktree)
			err := s.removeEnvVar(worktreePath)
			if err != nil {
				t.Fatalf("removeEnvVar() error = %v", err)
			}

			if tt.wantContent == "" {
				// File should not exist
				if _, err := os.Stat(envFile); !os.IsNotExist(err) {
					t.Error("Expected env file to not exist")
				}
				return
			}

			data, err := os.ReadFile(envFile)
			if err != nil {
				t.Fatalf("Failed to read env file: %v", err)
			}
			if string(data) != tt.wantContent {
				t.Errorf("env file content = %q, want %q", string(data), tt.wantContent)
			}
		})
	}
}

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source file
	srcDir := filepath.Join(tmpDir, "src")
	_ = os.MkdirAll(srcDir, 0755)
	srcFile := filepath.Join(srcDir, "test.key")
	_ = os.WriteFile(srcFile, []byte("secret-key"), 0600)

	// Copy to destination (with nested directory)
	dstFile := filepath.Join(tmpDir, "dst", "config", "test.key")
	err := copyFile(srcFile, dstFile)
	if err != nil {
		t.Fatalf("copyFile() error = %v", err)
	}

	// Verify content
	data, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read copied file: %v", err)
	}
	if string(data) != "secret-key" {
		t.Errorf("Expected 'secret-key', got %q", string(data))
	}
}

func TestCopyFile_SourceNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	err := copyFile(filepath.Join(tmpDir, "nonexistent"), filepath.Join(tmpDir, "dst"))
	if err == nil {
		t.Error("Expected error for nonexistent source")
	}
}

func TestCreateSymlink(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source directory
	srcDir := filepath.Join(tmpDir, "src", "vendor", "bundle")
	_ = os.MkdirAll(srcDir, 0755)
	_ = os.WriteFile(filepath.Join(srcDir, "marker"), []byte("here"), 0644)

	// Create symlink
	dstLink := filepath.Join(tmpDir, "dst", "vendor", "bundle")
	err := createSymlink(srcDir, dstLink)
	if err != nil {
		t.Fatalf("createSymlink() error = %v", err)
	}

	// Verify it's a symlink
	info, err := os.Lstat(dstLink)
	if err != nil {
		t.Fatalf("Failed to stat symlink: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("Expected symlink, got regular file/dir")
	}

	// Verify content is accessible through symlink
	data, err := os.ReadFile(filepath.Join(dstLink, "marker"))
	if err != nil {
		t.Fatalf("Failed to read through symlink: %v", err)
	}
	if string(data) != "here" {
		t.Errorf("Expected 'here', got %q", string(data))
	}
}

func TestCreateSymlink_ReplaceExisting(t *testing.T) {
	tmpDir := t.TempDir()

	srcDir := filepath.Join(tmpDir, "src")
	_ = os.MkdirAll(srcDir, 0755)

	dstLink := filepath.Join(tmpDir, "link")

	// Create initial symlink
	_ = os.Symlink(srcDir, dstLink)

	// Replace with new symlink
	newSrc := filepath.Join(tmpDir, "newsrc")
	_ = os.MkdirAll(newSrc, 0755)

	err := createSymlink(newSrc, dstLink)
	if err != nil {
		t.Fatalf("createSymlink() replace error = %v", err)
	}

	// Verify it points to new target
	target, err := os.Readlink(dstLink)
	if err != nil {
		t.Fatalf("Failed to readlink: %v", err)
	}
	if target != newSrc {
		t.Errorf("Expected symlink to %q, got %q", newSrc, target)
	}
}

func TestExternalStrategy_SetupWorktree(t *testing.T) {
	tmpDir := t.TempDir()

	// Create main worktree with files to copy/symlink
	mainPath := filepath.Join(tmpDir, "myapp")
	_ = os.MkdirAll(filepath.Join(mainPath, "config", "credentials"), 0755)
	_ = os.WriteFile(filepath.Join(mainPath, "config", "credentials", "development.key"), []byte("dev-key"), 0600)

	_ = os.MkdirAll(filepath.Join(mainPath, "vendor", "bundle"), 0755)
	_ = os.WriteFile(filepath.Join(mainPath, "vendor", "bundle", "marker"), []byte("gems"), 0644)

	// Create new worktree directory
	newPath := filepath.Join(tmpDir, "myapp-feature-x")
	_ = os.MkdirAll(newPath, 0755)

	s := newExternalStrategy(&config.Config{
		Plugins: config.PluginsConfig{
			Docker: config.DockerPluginConfig{
				Mode: "external",
				External: &config.ExternalComposeConfig{
					Path:        tmpDir,
					EnvVar:      "APP_DIR",
					Services:    []string{"app"},
					CopyFiles:   []string{"config/credentials/development.key"},
					SymlinkDirs: []string{"vendor/bundle"},
				},
			},
		},
	})

	err := setupWorktreeFiles(s.ext, newPath, mainPath)
	if err != nil {
		t.Fatalf("setupWorktree() error = %v", err)
	}

	// Verify copied file
	data, err := os.ReadFile(filepath.Join(newPath, "config", "credentials", "development.key"))
	if err != nil {
		t.Fatalf("Failed to read copied file: %v", err)
	}
	if string(data) != "dev-key" {
		t.Errorf("Expected 'dev-key', got %q", string(data))
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

func TestPlugin_OnPostCreate_External(t *testing.T) {
	tmpDir := t.TempDir()

	mainPath := filepath.Join(tmpDir, "myapp")
	_ = os.MkdirAll(filepath.Join(mainPath, "config"), 0755)
	_ = os.WriteFile(filepath.Join(mainPath, "config", "master.key"), []byte("key"), 0600)

	newPath := filepath.Join(tmpDir, "myapp-feature")
	_ = os.MkdirAll(newPath, 0755)

	plugin := New()
	cfg := &config.Config{
		Plugins: config.PluginsConfig{
			Docker: config.DockerPluginConfig{
				Mode: "external",
				External: &config.ExternalComposeConfig{
					Path:      tmpDir,
					EnvVar:    "APP_DIR",
					Services:  []string{"app"},
					CopyFiles: []string{"config/master.key"},
				},
			},
		},
	}
	_ = plugin.Init(cfg)

	ctx := &hooks.Context{
		Worktree:     "feature",
		Config:       cfg,
		WorktreePath: newPath,
		MainPath:     mainPath,
	}

	err := plugin.onPostCreate(ctx)
	if err != nil {
		t.Fatalf("onPostCreate() error = %v", err)
	}

	// Verify file was copied
	data, err := os.ReadFile(filepath.Join(newPath, "config", "master.key"))
	if err != nil {
		t.Fatalf("Failed to read copied file: %v", err)
	}
	if string(data) != "key" {
		t.Errorf("Expected 'key', got %q", string(data))
	}
}

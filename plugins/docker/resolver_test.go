package docker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lost-in-the/grove/internal/config"
)

func externalCfg(composeDir string) *config.Config {
	cfg := &config.Config{}
	cfg.Plugins.Docker.Mode = "external"
	cfg.Plugins.Docker.External = &config.ExternalComposeConfig{
		Path:   composeDir,
		EnvVar: "APP_DIR",
	}
	return cfg
}

func TestConfiguredWorktreePath(t *testing.T) {
	tests := []struct {
		name     string
		envBody  string
		mutate   func(*config.Config)
		wantPath func(dir string) string
		wantOK   bool
	}{
		{
			name:     "absolute value",
			envBody:  "APP_DIR=/work/myapp-feature\n",
			wantPath: func(string) string { return "/work/myapp-feature" },
			wantOK:   true,
		},
		{
			name:     "relative value resolves against compose path",
			envBody:  "APP_DIR=./myapp-feature\n",
			wantPath: func(dir string) string { return filepath.Join(dir, "myapp-feature") },
			wantOK:   true,
		},
		{
			name:     "parent-relative value is cleaned",
			envBody:  "APP_DIR=../myapp-feature\n",
			wantPath: func(dir string) string { return filepath.Join(filepath.Dir(dir), "myapp-feature") },
			wantOK:   true,
		},
		{
			name:     "double-quoted absolute value",
			envBody:  "APP_DIR=\"/work/myapp-feature\"\n",
			wantPath: func(string) string { return "/work/myapp-feature" },
			wantOK:   true,
		},
		{
			name:     "single-quoted relative value",
			envBody:  "APP_DIR='./myapp-feature'\n",
			wantPath: func(dir string) string { return filepath.Join(dir, "myapp-feature") },
			wantOK:   true,
		},
		{
			name:    "quoted empty value",
			envBody: "APP_DIR=\"\"\n",
			wantOK:  false,
		},
		{
			name:    "var unset",
			envBody: "OTHER=1\n",
			wantOK:  false,
		},
		{
			name:    "var empty",
			envBody: "APP_DIR=\n",
			wantOK:  false,
		},
		{
			name:    "local mode",
			envBody: "APP_DIR=/work/myapp-feature\n",
			mutate:  func(c *config.Config) { c.Plugins.Docker.Mode = "local" },
			wantOK:  false,
		},
		{
			name:    "external block missing",
			envBody: "APP_DIR=/work/myapp-feature\n",
			mutate:  func(c *config.Config) { c.Plugins.Docker.External = nil },
			wantOK:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(tt.envBody), 0644); err != nil {
				t.Fatal(err)
			}
			cfg := externalCfg(dir)
			if tt.mutate != nil {
				tt.mutate(cfg)
			}
			got, ok := ConfiguredWorktreePath(cfg)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v (path %q)", ok, tt.wantOK, got)
			}
			if tt.wantOK && got != tt.wantPath(dir) {
				t.Errorf("path = %q, want %q", got, tt.wantPath(dir))
			}
		})
	}
}

func TestConfiguredWorktreePath_NilConfig(t *testing.T) {
	if _, ok := ConfiguredWorktreePath(nil); ok {
		t.Error("nil config must not be ok")
	}
}

func TestConfiguredWorktreePath_CustomEnvFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env.local"), []byte("APP_DIR=./myapp-main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := externalCfg(dir)
	cfg.Plugins.Docker.External.EnvFile = ".env.local"
	got, ok := ConfiguredWorktreePath(cfg)
	if !ok || got != filepath.Join(dir, "myapp-main") {
		t.Errorf("got (%q, %v)", got, ok)
	}
}

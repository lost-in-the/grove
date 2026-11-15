package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/hooks"
	"github.com/lost-in-the/grove/internal/state"
	"github.com/lost-in-the/grove/internal/worktree"
	"github.com/lost-in-the/grove/plugins/docker"
)

// Dashboard create and fork both bootstrap through the shared sequence, which
// must leave the external stack's env file on the worktree it was configured for.
func TestDashboardCreate_LeavesExternalEnvFile(t *testing.T) {
	composeDir := t.TempDir()
	cfg := &config.Config{
		Plugins: config.PluginsConfig{
			Docker: config.DockerPluginConfig{
				Mode: "external",
				External: &config.ExternalComposeConfig{
					Path:     composeDir,
					EnvVar:   "APP_DIR",
					EnvFile:  ".env.local",
					Services: []string{"app"},
				},
			},
		},
	}
	plugin := docker.New()
	if err := plugin.Init(cfg); err != nil {
		t.Skipf("docker plugin unavailable: %v", err)
	}
	if err := plugin.RegisterHooks(hooks.GlobalRegistry()); err != nil {
		t.Fatal(err)
	}
	envFile := filepath.Join(composeDir, ".env.local")
	const before = "APP_DIR=./myapp-current\n"
	if err := os.WriteFile(envFile, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}

	repo := filepath.Join(t.TempDir(), "myapp")
	groveDir := filepath.Join(repo, ".grove")
	if err := os.MkdirAll(groveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mgr, err := worktree.NewManager(repo)
	if err != nil {
		t.Fatal(err)
	}
	stateMgr, err := state.NewManager(groveDir)
	if err != nil {
		t.Fatal(err)
	}
	wtPath := mgr.PathForName("feature")
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatal(err)
	}

	ch := make(chan creationEvent, 100)
	go func() {
		for range ch {
		}
	}()
	defer close(ch)

	wt := &worktree.Worktree{Name: filepath.Base(wtPath), Path: wtPath, Branch: "feature", ShortName: "feature"}
	runPostCreateStreaming(ch, mgr, stateMgr, cfg, repo, "feature", wt)

	got, _ := os.ReadFile(envFile)
	if string(got) != before {
		t.Errorf("env file = %q, want %q", got, before)
	}
}

package commands

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/hooks"
	"github.com/lost-in-the/grove/internal/worktree"
	"github.com/lost-in-the/grove/plugins/docker"
)

const envFileBefore = "APP_DIR=./myapp-current\n"

// externalEnvFixture registers the docker plugin's post-create hook for an
// external-mode config whose env file already points at another worktree, and
// returns the config and the env file path.
func externalEnvFixture(t *testing.T) (*config.Config, string) {
	t.Helper()
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
	if err := os.WriteFile(envFile, []byte(envFileBefore), 0o600); err != nil {
		t.Fatal(err)
	}
	return cfg, envFile
}

func assertEnvFileUnchanged(t *testing.T, envFile string) {
	t.Helper()
	got, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != envFileBefore {
		t.Errorf("env file = %q, want %q: creating a worktree must not move the shared stack", got, envFileBefore)
	}
}

// grove new, grove open and grove fetch all create through setupCreatedWorktree.
func TestSetupCreatedWorktree_LeavesExternalEnvFile(t *testing.T) {
	cfg, envFile := externalEnvFixture(t)

	repoRoot := filepath.Join(t.TempDir(), "myapp")
	groveDir := filepath.Join(repoRoot, ".grove")
	if err := os.MkdirAll(groveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mgr, err := worktree.NewManager(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(mgr.PathForName("feature"), 0o755); err != nil {
		t.Fatal(err)
	}

	ctx := &GroveContext{GroveDir: groveDir, ProjectRoot: repoRoot, Config: cfg}
	w := cli.NewWriter(&bytes.Buffer{}, false)
	if _, err := setupCreatedWorktree(ctx, mgr, "feature", "feature", worktreeSetupOpts{}, w); err != nil {
		t.Fatalf("setupCreatedWorktree: %v", err)
	}
	assertEnvFileUnchanged(t, envFile)
}

// grove fork and grove adopt call BootstrapWorktree directly.
func TestBootstrapWorktree_LeavesExternalEnvFile(t *testing.T) {
	cfg, envFile := externalEnvFixture(t)

	root := t.TempDir()
	mainDir := filepath.Join(root, "myapp")
	wtPath := filepath.Join(root, "myapp-feature")
	for _, d := range []string{filepath.Join(mainDir, ".grove"), wtPath} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	err := worktree.BootstrapWorktree(nil, cfg, worktree.BootstrapOpts{
		Name:         "feature",
		Branch:       "feature",
		WorktreePath: wtPath,
		MainPath:     mainDir,
	}, nil)
	if err != nil {
		t.Fatalf("BootstrapWorktree: %v", err)
	}
	assertEnvFileUnchanged(t, envFile)
}

func TestCreateInitialWorktrees_LeavesExternalEnvFile(t *testing.T) {
	cfg, envFile := externalEnvFixture(t)
	ext := cfg.Plugins.Docker.External

	dir := t.TempDir()
	runAdoptGit(t, dir, "init", "-b", "main")
	runAdoptGit(t, dir, "commit", "--allow-empty", "-m", "init")

	groveDir := filepath.Join(dir, ".grove")
	if err := os.MkdirAll(groveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	toml := fmt.Sprintf(`project_name = "proj"

[plugins.docker]
mode = "external"

[plugins.docker.external]
path = %q
env_var = %q
env_file = %q
services = ["app"]
`, ext.Path, ext.EnvVar, ext.EnvFile)
	if err := os.WriteFile(filepath.Join(groveDir, "config.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}

	origTesting, origScratch, origFull := initWithTesting, initWithScratch, initFull
	defer func() { initWithTesting, initWithScratch, initFull = origTesting, origScratch, origFull }()
	initWithTesting, initWithScratch, initFull = true, false, false

	createInitialWorktrees(groveDir, dir, "proj")

	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "proj-testing")); err != nil {
		t.Fatalf("initial worktree was not created: %v", err)
	}
	assertEnvFileUnchanged(t, envFile)
}

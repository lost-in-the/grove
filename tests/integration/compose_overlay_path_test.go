//go:build integration

package integration_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/testhelper"
)

// TestComposeOverlayPath_RelativeResolvesAgainstProjectRoot verifies that a
// relative path in plugins.docker.external.path in .grove/config.toml is
// resolved relative to the project root (the parent of .grove/), not relative
// to the .grove directory itself or the process working directory.
//
// This covers the bug fixed in commit 92e196e: external compose paths must be
// resolved against the project root so that secondary worktrees (which cd into
// a different directory) still find the shared compose dir.
func TestComposeOverlayPath_RelativeResolvesAgainstProjectRoot(t *testing.T) {
	repo := testhelper.SetupRailsFixture(t)

	// Create a sibling "infra" directory (simulates a shared compose dir next to
	// the project root, referenced as "../infra" from the project).
	infraDir := filepath.Join(filepath.Dir(repo), "infra")
	if err := os.MkdirAll(infraDir, 0755); err != nil {
		t.Fatalf("MkdirAll infra: %v", err)
	}
	testhelper.WriteFile(t, filepath.Join(infraDir, "docker-compose.yml"),
		"services:\n  app:\n    image: alpine:3\n    command: [\"sleep\", \"infinity\"]\n")
	testhelper.WriteFile(t, filepath.Join(infraDir, ".env"), "APP_DIR=.\n")

	// Write a config.toml with a relative external.path pointing to infra/.
	// The relative path "../infra" is relative to the project root (repo/).
	// projectRootFor(".grove/config.toml") = parent of .grove/ = repo/.
	// So "../infra" should resolve to filepath.Dir(repo) + "/infra".
	groveDir := filepath.Join(repo, ".grove")
	testhelper.WriteFile(t, filepath.Join(groveDir, "config.toml"), `
project_name = "rails-app"

[plugins.docker]
enabled = true
mode = "external"

[plugins.docker.external]
path = "../infra"
env_var = "APP_DIR"
services = ["app"]
`)

	// Load config using the grove dir — this triggers resolveProjectPaths.
	cfg, err := config.LoadFromGroveDir(groveDir)
	if err != nil {
		t.Fatalf("LoadFromGroveDir: %v", err)
	}

	if cfg.Plugins.Docker.External == nil {
		t.Fatal("external config not loaded")
	}

	gotPath := cfg.Plugins.Docker.External.Path
	wantPath := infraDir

	if gotPath != wantPath {
		t.Errorf("external.path: got %q, want %q", gotPath, wantPath)
	}

	// Must be an absolute path (no relative segments).
	if !filepath.IsAbs(gotPath) {
		t.Errorf("external.path %q is not absolute after resolution", gotPath)
	}
}

// TestComposeOverlayPath_AbsolutePathUnchanged verifies that an absolute path
// in plugins.docker.external.path passes through unchanged.
func TestComposeOverlayPath_AbsolutePathUnchanged(t *testing.T) {
	repo := testhelper.SetupRailsFixture(t)

	infraDir := filepath.Join(t.TempDir(), "shared-infra")
	if err := os.MkdirAll(infraDir, 0755); err != nil {
		t.Fatalf("MkdirAll infra: %v", err)
	}
	testhelper.WriteFile(t, filepath.Join(infraDir, "docker-compose.yml"),
		"services:\n  app:\n    image: alpine:3\n    command: [\"sleep\", \"infinity\"]\n")
	testhelper.WriteFile(t, filepath.Join(infraDir, ".env"), "APP_DIR=.\n")

	groveDir := filepath.Join(repo, ".grove")
	testhelper.WriteFile(t, filepath.Join(groveDir, "config.toml"), `
project_name = "rails-app"

[plugins.docker]
enabled = true
mode = "external"

[plugins.docker.external]
path = "`+infraDir+`"
env_var = "APP_DIR"
services = ["app"]
`)

	cfg, err := config.LoadFromGroveDir(groveDir)
	if err != nil {
		t.Fatalf("LoadFromGroveDir: %v", err)
	}

	gotPath := cfg.Plugins.Docker.External.Path
	if gotPath != infraDir {
		t.Errorf("absolute path changed: got %q, want %q", gotPath, infraDir)
	}
}

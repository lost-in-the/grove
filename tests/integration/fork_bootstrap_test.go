//go:build integration

// Coverage for `grove fork` provisioning through the shared BootstrapWorktree
// (excludes, state+parent, SetupFiles, plugin + config post_create hooks)
// instead of a hand-rolled mirror — the DRY consolidation that keeps a fork
// from silently skipping post_create recipes (B32).
package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/lost-in-the/grove/internal/testhelper"
)

func TestFork_RunsPostCreateHooksViaBootstrap(t *testing.T) {
	binary := buildGroveBinary(t)
	repo := testhelper.SetupRailsFixture(t)

	if err := os.MkdirAll(filepath.Join(repo, ".grove"), 0o755); err != nil {
		t.Fatal(err)
	}
	hooksToml := `[hooks]
[[hooks.post_create]]
type = "command"
command = "touch fork-bootstrap-ran"
working_dir = "main"
`
	if err := os.WriteFile(filepath.Join(repo, ".grove", "hooks.toml"), []byte(hooksToml), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binary, "fork", "feat", "--no-switch", "--no-wip")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("grove fork failed: %v\n%s", err, out)
	}

	// The post_create hook runs only if fork went through the shared bootstrap.
	if _, err := os.Stat(filepath.Join(repo, "fork-bootstrap-ran")); err != nil {
		t.Errorf("post_create hook did not run on grove fork (bootstrap skipped?): %v", err)
	}
}

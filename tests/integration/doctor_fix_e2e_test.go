//go:build integration

// End-to-end coverage for `grove doctor --fix`. The underlying rewrite logic
// (fixHostInstallsInDockerProject) has rich unit coverage in doctor_test.go;
// this test pins the cobra flag wiring + binary invocation so a regression in
// the top-level glue surfaces in CI rather than during manual release sanity.

package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/testhelper"
)

func TestGroveDoctor_FixRewritesHostInstall(t *testing.T) {
	binary := buildGroveBinary(t)

	// Build a Rails-shaped project with compose + a hooks.toml carrying a
	// host bundle-install command. doctor --fix should rewrite this in place.
	repo := testhelper.SetupRailsFixture(t)
	mustWrite(t, filepath.Join(repo, "Dockerfile"), "FROM ruby\n")
	mustWrite(t, filepath.Join(repo, "docker-compose.yml"),
		"services:\n  web:\n    image: ruby\n")

	groveDir := filepath.Join(repo, ".grove")
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(groveDir, "state.json"),
		`{"version": 1, "worktrees": {}}`)
	hooksPath := filepath.Join(groveDir, "hooks.toml")
	mustWrite(t, hooksPath, `[[hooks.post_create]]
type = "command"
command = "bundle install"
`)

	cmd := exec.Command(binary, "doctor", "--fix")
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("grove doctor --fix failed: %v\noutput: %s", err, out)
	}

	got, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("read back hooks.toml: %v", err)
	}
	hooks := string(got)

	if !strings.Contains(hooks, `type = "docker:compose"`) {
		t.Errorf("hooks.toml not rewritten to docker:compose:\n%s", hooks)
	}
	if !strings.Contains(hooks, `service = "web"`) {
		t.Errorf("hooks.toml missing inferred service:\n%s", hooks)
	}
	// The original `bundle install` command should still be present (preserved
	// inside the now-routed compose hook), not replaced.
	if !strings.Contains(hooks, "bundle install") {
		t.Errorf("hooks.toml dropped the original command:\n%s", hooks)
	}
}

// TestGroveDoctor_NoFixLeavesHooksAlone asserts that running doctor without
// --fix leaves hooks.toml untouched (the diagnostic still prints, but the
// file isn't modified).
func TestGroveDoctor_NoFixLeavesHooksAlone(t *testing.T) {
	binary := buildGroveBinary(t)

	repo := testhelper.SetupRailsFixture(t)
	mustWrite(t, filepath.Join(repo, "Dockerfile"), "FROM ruby\n")
	mustWrite(t, filepath.Join(repo, "docker-compose.yml"),
		"services:\n  web:\n    image: ruby\n")

	groveDir := filepath.Join(repo, ".grove")
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(groveDir, "state.json"),
		`{"version": 1, "worktrees": {}}`)
	hooksPath := filepath.Join(groveDir, "hooks.toml")
	original := `[[hooks.post_create]]
type = "command"
command = "bundle install"
`
	mustWrite(t, hooksPath, original)

	cmd := exec.Command(binary, "doctor")
	cmd.Dir = repo
	if _, err := cmd.CombinedOutput(); err != nil {
		// doctor prints a warning and may exit non-zero when checks fail —
		// that's fine; we only care that the file is untouched.
		_ = err
	}

	got, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("read hooks.toml: %v", err)
	}
	if string(got) != original {
		t.Errorf("hooks.toml mutated without --fix:\nwant:\n%s\ngot:\n%s", original, got)
	}
}

func mustWrite(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

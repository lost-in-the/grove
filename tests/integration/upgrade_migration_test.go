//go:build integration

package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/testhelper"
)

// TestUpgradeMigration_ExcludesNoticeFiresOnce: repos initialized by an older
// grove carry a git-exclude entry that hides the committable
// .grove/config.toml. The first grove command after upgrading must self-heal
// the exclude file and print the one-time migration notice; every later
// command must stay silent (the migration is idempotent, so the notice
// self-extinguishes without any extra "seen" state).
func TestUpgradeMigration_ExcludesNoticeFiresOnce(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	binary := buildGroveBinary(t)
	repo := testhelper.SetupRailsFixture(t)

	// Recreate the legacy layout: the managed block as the old version wrote
	// it, config.toml entry included.
	legacyBlock := "# Grove (worktree manager) — machine-local, applies to all worktrees\n" +
		".grove/state.json\n.grove/state.json.bak\n.grove/state.lock\n" +
		".grove/ui_prefs.json\n.grove/.envrc\n.grove/config.local.toml\n" +
		".grove/config.toml\n"
	excludePath := filepath.Join(repo, ".git", "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(excludePath, []byte(legacyBlock), 0o644); err != nil {
		t.Fatal(err)
	}

	runLs := func() (stdout, stderr string) {
		t.Helper()
		cmd := exec.Command(binary, "ls")
		cmd.Dir = repo
		var out, errBuf strings.Builder
		cmd.Stdout = &out
		cmd.Stderr = &errBuf
		if err := cmd.Run(); err != nil {
			t.Fatalf("grove ls: %v\nstderr: %s", err, errBuf.String())
		}
		return out.String(), errBuf.String()
	}

	// First command after "upgrade": migrates and notifies.
	stdout, stderr := runLs()
	if !strings.Contains(stderr, "migrated git excludes") {
		t.Errorf("first run after upgrade: migration notice missing from stderr:\n%s", stderr)
	}
	if strings.Contains(stdout, "migrated git excludes") {
		t.Errorf("migration notice leaked to stdout (breaks --json consumers):\n%s", stdout)
	}
	content, err := os.ReadFile(excludePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), ".grove/config.toml") {
		t.Errorf("legacy config.toml entry survived the migration:\n%s", content)
	}

	// Second command: nothing left to migrate, no notice.
	_, stderr = runLs()
	if strings.Contains(stderr, "migrated git excludes") {
		t.Errorf("migration notice repeated on the second run:\n%s", stderr)
	}
}

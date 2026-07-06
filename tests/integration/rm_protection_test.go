//go:build integration

// End-to-end coverage for `grove rm` protection when the worktree is
// removed by a name that differs from the one listed in
// [protection].protected.
//
// mgr.Find resolves a worktree by short name, display name, branch name, or
// path basename — so a worktree whose directory is named "foo" but whose
// branch is "bar" can be targeted as either `grove rm foo` or `grove rm bar`.
// [protection].protected is configured against the worktree's short name
// ("foo"); removal must be blocked regardless of which name the user types.
package integration_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/testhelper"
)

func TestRmProtection_BlocksRemovalByBranchName(t *testing.T) {
	binary := buildGroveBinary(t)

	repo := testhelper.SetupRailsFixture(t)

	// Worktree short name "foo", branch "bar" — deliberately divergent so
	// `grove rm bar` exercises the Find-resolved-name path.
	wtPath := filepath.Join(filepath.Dir(repo), "rails-app-foo")
	testhelper.RunGit(t, repo, "worktree", "add", "-b", "bar", wtPath)

	// Protect the worktree under its short name ("foo"), not its branch name.
	testhelper.WriteFile(t, filepath.Join(repo, ".grove", "config.toml"), `project_name = "rails-app"

[plugins.docker]
enabled = true
auto_start = false
auto_stop = false

[protection]
protected = ["foo"]
`)

	// Attempt removal by branch name, without --force/--unprotect.
	cmd := exec.Command(binary, "rm", "bar")
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatalf("expected `grove rm bar` to fail for a config-protected worktree, but it succeeded: %s", out)
	}

	if !strings.Contains(string(out), "protected") {
		t.Errorf("expected output to mention protection, got: %s", out)
	}

	if _, statErr := exec.Command("git", "-C", repo, "worktree", "list").CombinedOutput(); statErr != nil {
		t.Fatalf("git worktree list failed: %v", statErr)
	}
	listOut, _ := exec.Command("git", "-C", repo, "worktree", "list").CombinedOutput()
	if !strings.Contains(string(listOut), wtPath) {
		t.Errorf("worktree %s should not have been removed:\n%s", wtPath, listOut)
	}
}

// TestRmProtection_AllowsRemovalByBranchNameWithForceUnprotect verifies the
// escape hatch still works when targeting the worktree by branch name.
func TestRmProtection_AllowsRemovalByBranchNameWithForceUnprotect(t *testing.T) {
	binary := buildGroveBinary(t)

	repo := testhelper.SetupRailsFixture(t)

	wtPath := filepath.Join(filepath.Dir(repo), "rails-app-foo")
	testhelper.RunGit(t, repo, "worktree", "add", "-b", "bar", wtPath)

	testhelper.WriteFile(t, filepath.Join(repo, ".grove", "config.toml"), `project_name = "rails-app"

[plugins.docker]
enabled = true
auto_start = false
auto_stop = false

[protection]
protected = ["foo"]
`)

	cmd := exec.Command(binary, "rm", "bar", "--force", "--unprotect", "--keep-branch")
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("grove rm bar --force --unprotect failed: %v\noutput: %s", err, out)
	}

	listOut, _ := exec.Command("git", "-C", repo, "worktree", "list").CombinedOutput()
	if strings.Contains(string(listOut), wtPath) {
		t.Errorf("worktree %s should have been removed:\n%s", wtPath, listOut)
	}
}

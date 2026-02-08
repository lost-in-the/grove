package git

import (
	"os"
	"os/exec"
	"testing"
)

// initTestRepo creates a bare git repo with an initial commit and returns its path.
func initTestRepo(t *testing.T, branchName string) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL=/dev/null",
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}

	run("git", "init", "-b", branchName)
	run("git", "commit", "--allow-empty", "-m", "init")

	return dir
}

func TestDetectDefaultBranch_NoRemote(t *testing.T) {
	// Repo with "main" branch but no remote — should find "main" via common names fallback
	dir := initTestRepo(t, "main")

	branch, err := detectDefaultBranch(dir)
	if err != nil {
		t.Fatalf("detectDefaultBranch() error = %v", err)
	}
	if branch != "main" {
		t.Errorf("detectDefaultBranch() = %q, want %q", branch, "main")
	}
}

func TestDetectDefaultBranch_FallbackCurrent(t *testing.T) {
	// Repo with a non-standard branch name, no remote — should fall back to current branch
	dir := initTestRepo(t, "develop")

	branch, err := detectDefaultBranch(dir)
	if err != nil {
		t.Fatalf("detectDefaultBranch() error = %v", err)
	}
	if branch != "develop" {
		t.Errorf("detectDefaultBranch() = %q, want %q", branch, "develop")
	}
}

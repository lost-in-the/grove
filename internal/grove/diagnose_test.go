package grove

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDiagnoseNoGrove(t *testing.T) {
	// Set up a git repo with no .grove
	dir := t.TempDir()
	runGit(t, dir, "init")

	result := DiagnoseNoGrove(dir)

	if result.Reason != ReasonNoGroveDir {
		t.Errorf("expected ReasonNoGroveDir, got %v", result.Reason)
	}
}

func TestDiagnoseNoGrove_NotGitRepo(t *testing.T) {
	dir := t.TempDir()

	result := DiagnoseNoGrove(dir)

	if result.Reason != ReasonNotGitRepo {
		t.Errorf("expected ReasonNotGitRepo, got %v", result.Reason)
	}
}

func TestDiagnoseNoGrove_WorktreeMainMissingGrove(t *testing.T) {
	// Set up main repo with no .grove, then create a worktree
	mainDir := t.TempDir()
	runGit(t, mainDir, "init")
	runGit(t, mainDir, "commit", "--allow-empty", "-m", "init")

	wtDir := filepath.Join(t.TempDir(), "worktree")
	runGit(t, mainDir, "worktree", "add", wtDir, "-b", "test-branch")

	result := DiagnoseNoGrove(wtDir)

	if result.Reason != ReasonMainWorktreeMissingGrove {
		t.Errorf("expected ReasonMainWorktreeMissingGrove, got %v", result.Reason)
	}
	// Resolve symlinks for comparison (macOS /var → /private/var)
	wantMain, _ := filepath.EvalSymlinks(mainDir)
	if result.MainWorktreePath != wantMain {
		t.Errorf("expected main path %s, got %s", wantMain, result.MainWorktreePath)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

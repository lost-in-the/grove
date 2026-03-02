package git

import (
	"os"
	"os/exec"
	"path/filepath"
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

// makeRunner returns a git command runner for the given directory that fatally
// fails the test on any error.
func makeRunner(t *testing.T, dir string) func(args ...string) {
	t.Helper()
	return func(args ...string) {
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
}

func TestNewBranchManager(t *testing.T) {
	t.Run("valid repo with main branch succeeds", func(t *testing.T) {
		dir := initTestRepo(t, "main")
		bm, err := NewBranchManager(dir)
		if err != nil {
			t.Fatalf("NewBranchManager() error = %v", err)
		}
		if bm == nil {
			t.Fatal("NewBranchManager() returned nil")
		}
		if bm.defaultBranch != "main" {
			t.Errorf("defaultBranch = %q, want %q", bm.defaultBranch, "main")
		}
	})

	t.Run("invalid repo path fails", func(t *testing.T) {
		_, err := NewBranchManager("/nonexistent/path/that/does/not/exist")
		if err == nil {
			t.Fatal("NewBranchManager() expected error for invalid path, got nil")
		}
	})
}

func TestListLocalBranches(t *testing.T) {
	t.Run("repo with multiple branches lists all of them", func(t *testing.T) {
		dir := initTestRepo(t, "main")
		run := makeRunner(t, dir)
		run("git", "checkout", "-b", "feature-one")
		run("git", "commit", "--allow-empty", "-m", "feature one")
		run("git", "checkout", "-b", "feature-two")
		run("git", "commit", "--allow-empty", "-m", "feature two")
		run("git", "checkout", "main")

		branches, err := ListLocalBranches(dir)
		if err != nil {
			t.Fatalf("ListLocalBranches() error = %v", err)
		}
		if len(branches) != 3 {
			t.Errorf("got %d branches, want 3: %v", len(branches), branches)
		}
		want := map[string]bool{"main": true, "feature-one": true, "feature-two": true}
		for _, b := range branches {
			if !want[b] {
				t.Errorf("unexpected branch %q in result", b)
			}
		}
	})

	t.Run("repo with single branch returns one entry", func(t *testing.T) {
		dir := initTestRepo(t, "main")

		branches, err := ListLocalBranches(dir)
		if err != nil {
			t.Fatalf("ListLocalBranches() error = %v", err)
		}
		if len(branches) != 1 {
			t.Errorf("got %d branches, want 1: %v", len(branches), branches)
		}
		if branches[0] != "main" {
			t.Errorf("got branch %q, want %q", branches[0], "main")
		}
	})
}

func TestIsBranchMerged(t *testing.T) {
	t.Run("feature branch merged into default returns true", func(t *testing.T) {
		dir := initTestRepo(t, "main")
		run := makeRunner(t, dir)
		run("git", "checkout", "-b", "merged-branch")
		run("git", "commit", "--allow-empty", "-m", "will be merged")
		run("git", "checkout", "main")
		run("git", "merge", "merged-branch")

		bm, err := NewBranchManager(dir)
		if err != nil {
			t.Fatalf("NewBranchManager() error = %v", err)
		}
		got, err := bm.isBranchMerged("merged-branch")
		if err != nil {
			t.Fatalf("isBranchMerged() error = %v", err)
		}
		if !got {
			t.Error("isBranchMerged() = false, want true")
		}
	})

	t.Run("feature branch not merged returns false", func(t *testing.T) {
		dir := initTestRepo(t, "main")
		run := makeRunner(t, dir)
		run("git", "checkout", "-b", "unmerged-branch")
		run("git", "commit", "--allow-empty", "-m", "not merged yet")
		run("git", "checkout", "main")

		bm, err := NewBranchManager(dir)
		if err != nil {
			t.Fatalf("NewBranchManager() error = %v", err)
		}
		got, err := bm.isBranchMerged("unmerged-branch")
		if err != nil {
			t.Fatalf("isBranchMerged() error = %v", err)
		}
		if got {
			t.Error("isBranchMerged() = true, want false")
		}
	})
}

func TestGetStatus(t *testing.T) {
	t.Run("branch with no remote tracking", func(t *testing.T) {
		dir := initTestRepo(t, "main")
		run := makeRunner(t, dir)
		// Create a branch that isn't checked out in any worktree
		run("git", "checkout", "-b", "no-remote-branch")
		run("git", "checkout", "main")

		bm, err := NewBranchManager(dir)
		if err != nil {
			t.Fatalf("NewBranchManager() error = %v", err)
		}
		status, err := bm.GetStatus("no-remote-branch", "")
		if err != nil {
			t.Fatalf("GetStatus() error = %v", err)
		}
		if status.Name != "no-remote-branch" {
			t.Errorf("Name = %q, want %q", status.Name, "no-remote-branch")
		}
		if status.HasRemote {
			t.Error("HasRemote = true, want false for branch with no remote")
		}
		if status.UsedByWorktree != "" {
			t.Errorf("UsedByWorktree = %q, want empty", status.UsedByWorktree)
		}
	})

	t.Run("branch that is merged", func(t *testing.T) {
		dir := initTestRepo(t, "main")
		run := makeRunner(t, dir)
		run("git", "checkout", "-b", "merged-branch")
		run("git", "commit", "--allow-empty", "-m", "will be merged")
		run("git", "checkout", "main")
		run("git", "merge", "merged-branch")

		bm, err := NewBranchManager(dir)
		if err != nil {
			t.Fatalf("NewBranchManager() error = %v", err)
		}
		status, err := bm.GetStatus("merged-branch", "")
		if err != nil {
			t.Fatalf("GetStatus() error = %v", err)
		}
		if !status.IsMerged {
			t.Error("IsMerged = false, want true for merged branch")
		}
	})
}

func TestDelete(t *testing.T) {
	t.Run("delete merged branch with force=false succeeds", func(t *testing.T) {
		dir := initTestRepo(t, "main")
		run := makeRunner(t, dir)
		run("git", "checkout", "-b", "merged-branch")
		run("git", "commit", "--allow-empty", "-m", "will be merged")
		run("git", "checkout", "main")
		run("git", "merge", "merged-branch")

		bm, err := NewBranchManager(dir)
		if err != nil {
			t.Fatalf("NewBranchManager() error = %v", err)
		}
		if err := bm.Delete("merged-branch", false); err != nil {
			t.Errorf("Delete() error = %v, want nil", err)
		}
	})

	t.Run("delete unmerged branch with force=false fails", func(t *testing.T) {
		dir := initTestRepo(t, "main")
		run := makeRunner(t, dir)
		run("git", "checkout", "-b", "unmerged-branch")
		run("git", "commit", "--allow-empty", "-m", "not merged")
		run("git", "checkout", "main")

		bm, err := NewBranchManager(dir)
		if err != nil {
			t.Fatalf("NewBranchManager() error = %v", err)
		}
		if err := bm.Delete("unmerged-branch", false); err == nil {
			t.Error("Delete() expected error for unmerged branch with force=false, got nil")
		}
	})

	t.Run("delete unmerged branch with force=true succeeds", func(t *testing.T) {
		dir := initTestRepo(t, "main")
		run := makeRunner(t, dir)
		run("git", "checkout", "-b", "unmerged-branch")
		run("git", "commit", "--allow-empty", "-m", "not merged")
		run("git", "checkout", "main")

		bm, err := NewBranchManager(dir)
		if err != nil {
			t.Fatalf("NewBranchManager() error = %v", err)
		}
		if err := bm.Delete("unmerged-branch", true); err != nil {
			t.Errorf("Delete() error = %v, want nil", err)
		}
	})

	t.Run("delete non-existent branch fails", func(t *testing.T) {
		dir := initTestRepo(t, "main")
		bm, err := NewBranchManager(dir)
		if err != nil {
			t.Fatalf("NewBranchManager() error = %v", err)
		}
		if err := bm.Delete("nonexistent-branch", false); err == nil {
			t.Error("Delete() expected error for non-existent branch, got nil")
		}
	})
}

func TestBranchUsedByWorktree(t *testing.T) {
	t.Run("branch used by a worktree returns the worktree path", func(t *testing.T) {
		dir := initTestRepo(t, "main")
		run := makeRunner(t, dir)
		wtPath := filepath.Join(t.TempDir(), "wt")
		run("git", "worktree", "add", wtPath, "-b", "wt-branch")

		// git resolves symlinks in worktree paths (relevant on macOS where /var -> /private/var)
		wantPath, err := filepath.EvalSymlinks(wtPath)
		if err != nil {
			t.Fatalf("EvalSymlinks(%q) error = %v", wtPath, err)
		}

		bm, err := NewBranchManager(dir)
		if err != nil {
			t.Fatalf("NewBranchManager() error = %v", err)
		}
		got, err := bm.branchUsedByWorktree("wt-branch", "")
		if err != nil {
			t.Fatalf("branchUsedByWorktree() error = %v", err)
		}
		if got != wantPath {
			t.Errorf("branchUsedByWorktree() = %q, want %q", got, wantPath)
		}
	})

	t.Run("branch not used by any worktree returns empty string", func(t *testing.T) {
		dir := initTestRepo(t, "main")
		run := makeRunner(t, dir)
		run("git", "checkout", "-b", "unused-branch")
		run("git", "checkout", "main")

		bm, err := NewBranchManager(dir)
		if err != nil {
			t.Fatalf("NewBranchManager() error = %v", err)
		}
		got, err := bm.branchUsedByWorktree("unused-branch", "")
		if err != nil {
			t.Fatalf("branchUsedByWorktree() error = %v", err)
		}
		if got != "" {
			t.Errorf("branchUsedByWorktree() = %q, want empty", got)
		}
	})

	t.Run("ExcludeWorktree correctly excludes a match", func(t *testing.T) {
		dir := initTestRepo(t, "main")
		run := makeRunner(t, dir)
		wtPath := filepath.Join(t.TempDir(), "wt")
		run("git", "worktree", "add", wtPath, "-b", "wt-branch")

		// git resolves symlinks, so we must pass the canonical path as excludeWorktree
		resolvedWtPath, err := filepath.EvalSymlinks(wtPath)
		if err != nil {
			t.Fatalf("EvalSymlinks(%q) error = %v", wtPath, err)
		}

		bm, err := NewBranchManager(dir)
		if err != nil {
			t.Fatalf("NewBranchManager() error = %v", err)
		}
		got, err := bm.branchUsedByWorktree("wt-branch", resolvedWtPath)
		if err != nil {
			t.Fatalf("branchUsedByWorktree() error = %v", err)
		}
		if got != "" {
			t.Errorf("branchUsedByWorktree() with excludeWorktree = %q, want empty", got)
		}
	})
}

func TestGetUnpushedCommits(t *testing.T) {
	t.Run("branch with no upstream returns nil no error", func(t *testing.T) {
		dir := initTestRepo(t, "main")
		bm, err := NewBranchManager(dir)
		if err != nil {
			t.Fatalf("NewBranchManager() error = %v", err)
		}
		commits, err := bm.GetUnpushedCommits("main", 10)
		if err != nil {
			t.Fatalf("GetUnpushedCommits() error = %v", err)
		}
		if commits != nil {
			t.Errorf("GetUnpushedCommits() = %v, want nil", commits)
		}
	})
}

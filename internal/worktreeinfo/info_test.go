package worktreeinfo_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/lost-in-the/grove/internal/worktreeinfo"
)

// setupRepo initializes a bare git repo in a temp dir, configures user
// identity, and makes one empty initial commit on the given branch.
// Returns the repo path.
func setupRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("setup: %s: %v\n%s", args, err, out)
		}
	}

	run("git", "init", "-b", "main")
	run("git", "config", "user.email", "test@example.com")
	run("git", "config", "user.name", "Test User")
	run("git", "commit", "--allow-empty", "-m", "initial commit")

	return dir
}

// gitRun runs a git command in dir and fails the test on error.
func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
}

// ---------------------------------------------------------------------------
// RecentCommit JSON tags
// ---------------------------------------------------------------------------

func TestRecentCommit_JSONTags(t *testing.T) {
	rc := worktreeinfo.RecentCommit{SHA: "abc1234", Message: "hello world"}
	data, err := json.Marshal(rc)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if _, ok := m["sha"]; !ok {
		t.Errorf("expected JSON key 'sha', got keys: %v", m)
	}
	if _, ok := m["message"]; !ok {
		t.Errorf("expected JSON key 'message', got keys: %v", m)
	}
	if v, _ := m["sha"].(string); v != "abc1234" {
		t.Errorf("sha = %q, want abc1234", v)
	}
	if v, _ := m["message"].(string); v != "hello world" {
		t.Errorf("message = %q, want hello world", v)
	}
}

// ---------------------------------------------------------------------------
// UpstreamInfo
// ---------------------------------------------------------------------------

func TestUpstreamInfo_NoUpstream(t *testing.T) {
	dir := setupRepo(t)
	ahead, behind, hasRemote, trackingBranch := worktreeinfo.UpstreamInfo(dir)
	if hasRemote {
		t.Errorf("hasRemote = true, want false (no upstream configured)")
	}
	if ahead != 0 || behind != 0 {
		t.Errorf("ahead=%d behind=%d, want 0 0", ahead, behind)
	}
	if trackingBranch != "" {
		t.Errorf("trackingBranch = %q, want empty", trackingBranch)
	}
}

func TestUpstreamInfo_InSync(t *testing.T) {
	// Create a "remote" (bare clone) and a local clone tracking it.
	bare := t.TempDir()
	gitRun(t, bare, "init", "--bare", "-b", "main")

	// Seed the bare repo with one commit via a temporary clone.
	seed := t.TempDir()
	gitRun(t, seed, "clone", bare, ".")
	gitRun(t, seed, "config", "user.email", "test@example.com")
	gitRun(t, seed, "config", "user.name", "Test User")
	gitRun(t, seed, "commit", "--allow-empty", "-m", "initial commit")
	gitRun(t, seed, "push", "origin", "main")

	// Main working clone.
	local := t.TempDir()
	gitRun(t, local, "clone", bare, ".")
	gitRun(t, local, "config", "user.email", "test@example.com")
	gitRun(t, local, "config", "user.name", "Test User")

	ahead, behind, hasRemote, trackingBranch := worktreeinfo.UpstreamInfo(local)
	if !hasRemote {
		t.Errorf("hasRemote = false, want true")
	}
	if ahead != 0 || behind != 0 {
		t.Errorf("ahead=%d behind=%d, want 0 0", ahead, behind)
	}
	if trackingBranch == "" {
		t.Errorf("trackingBranch is empty, want non-empty")
	}
}

func TestUpstreamInfo_AheadByN(t *testing.T) {
	bare := t.TempDir()
	gitRun(t, bare, "init", "--bare", "-b", "main")

	seed := t.TempDir()
	gitRun(t, seed, "clone", bare, ".")
	gitRun(t, seed, "config", "user.email", "test@example.com")
	gitRun(t, seed, "config", "user.name", "Test User")
	gitRun(t, seed, "commit", "--allow-empty", "-m", "initial commit")
	gitRun(t, seed, "push", "origin", "main")

	local := t.TempDir()
	gitRun(t, local, "clone", bare, ".")
	gitRun(t, local, "config", "user.email", "test@example.com")
	gitRun(t, local, "config", "user.name", "Test User")

	// Add 2 local commits not pushed.
	gitRun(t, local, "commit", "--allow-empty", "-m", "local commit 1")
	gitRun(t, local, "commit", "--allow-empty", "-m", "local commit 2")

	ahead, behind, hasRemote, _ := worktreeinfo.UpstreamInfo(local)
	if !hasRemote {
		t.Errorf("hasRemote = false, want true")
	}
	if ahead != 2 {
		t.Errorf("ahead = %d, want 2", ahead)
	}
	if behind != 0 {
		t.Errorf("behind = %d, want 0", behind)
	}
}

func TestUpstreamInfo_BehindByM(t *testing.T) {
	bare := t.TempDir()
	gitRun(t, bare, "init", "--bare", "-b", "main")

	seed := t.TempDir()
	gitRun(t, seed, "clone", bare, ".")
	gitRun(t, seed, "config", "user.email", "test@example.com")
	gitRun(t, seed, "config", "user.name", "Test User")
	gitRun(t, seed, "commit", "--allow-empty", "-m", "initial commit")
	gitRun(t, seed, "push", "origin", "main")

	local := t.TempDir()
	gitRun(t, local, "clone", bare, ".")
	gitRun(t, local, "config", "user.email", "test@example.com")
	gitRun(t, local, "config", "user.name", "Test User")

	// Push 3 more commits via seed so local is behind.
	gitRun(t, seed, "commit", "--allow-empty", "-m", "remote commit 1")
	gitRun(t, seed, "commit", "--allow-empty", "-m", "remote commit 2")
	gitRun(t, seed, "commit", "--allow-empty", "-m", "remote commit 3")
	gitRun(t, seed, "push", "origin", "main")

	// Fetch but don't merge.
	gitRun(t, local, "fetch", "origin")

	ahead, behind, hasRemote, _ := worktreeinfo.UpstreamInfo(local)
	if !hasRemote {
		t.Errorf("hasRemote = false, want true")
	}
	if behind != 3 {
		t.Errorf("behind = %d, want 3", behind)
	}
	if ahead != 0 {
		t.Errorf("ahead = %d, want 0", ahead)
	}
}

func TestUpstreamInfo_AheadAndBehind(t *testing.T) {
	bare := t.TempDir()
	gitRun(t, bare, "init", "--bare", "-b", "main")

	seed := t.TempDir()
	gitRun(t, seed, "clone", bare, ".")
	gitRun(t, seed, "config", "user.email", "test@example.com")
	gitRun(t, seed, "config", "user.name", "Test User")
	gitRun(t, seed, "commit", "--allow-empty", "-m", "initial commit")
	gitRun(t, seed, "push", "origin", "main")

	local := t.TempDir()
	gitRun(t, local, "clone", bare, ".")
	gitRun(t, local, "config", "user.email", "test@example.com")
	gitRun(t, local, "config", "user.name", "Test User")

	// Local adds 1 commit.
	gitRun(t, local, "commit", "--allow-empty", "-m", "local commit")

	// Remote adds 2 commits.
	gitRun(t, seed, "commit", "--allow-empty", "-m", "remote commit 1")
	gitRun(t, seed, "commit", "--allow-empty", "-m", "remote commit 2")
	gitRun(t, seed, "push", "origin", "main")

	// Fetch remote changes (but don't merge; local is diverged).
	gitRun(t, local, "fetch", "origin")

	ahead, behind, hasRemote, _ := worktreeinfo.UpstreamInfo(local)
	if !hasRemote {
		t.Errorf("hasRemote = false, want true")
	}
	if ahead != 1 {
		t.Errorf("ahead = %d, want 1", ahead)
	}
	if behind != 2 {
		t.Errorf("behind = %d, want 2", behind)
	}
}

func TestUpstreamInfo_NonexistentPath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "does-not-exist")
	ahead, behind, hasRemote, trackingBranch := worktreeinfo.UpstreamInfo(dir)
	if hasRemote {
		t.Errorf("hasRemote = true, want false for nonexistent path")
	}
	if ahead != 0 || behind != 0 {
		t.Errorf("ahead=%d behind=%d, want 0 0", ahead, behind)
	}
	if trackingBranch != "" {
		t.Errorf("trackingBranch = %q, want empty", trackingBranch)
	}
}

// ---------------------------------------------------------------------------
// CommitCountAhead
// ---------------------------------------------------------------------------

func TestCommitCountAhead_OnDefaultBranch(t *testing.T) {
	dir := setupRepo(t)
	count := worktreeinfo.CommitCountAhead(dir, "main")
	if count != 0 {
		t.Errorf("count = %d, want 0 (on default branch)", count)
	}
}

func TestCommitCountAhead_ThreeAhead(t *testing.T) {
	dir := setupRepo(t)
	gitRun(t, dir, "checkout", "-b", "feature")
	gitRun(t, dir, "commit", "--allow-empty", "-m", "feat 1")
	gitRun(t, dir, "commit", "--allow-empty", "-m", "feat 2")
	gitRun(t, dir, "commit", "--allow-empty", "-m", "feat 3")

	count := worktreeinfo.CommitCountAhead(dir, "main")
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

func TestCommitCountAhead_NonexistentDefaultBranch(t *testing.T) {
	dir := setupRepo(t)
	count := worktreeinfo.CommitCountAhead(dir, "nonexistent-branch")
	if count != 0 {
		t.Errorf("count = %d, want 0 (nonexistent branch)", count)
	}
}

// ---------------------------------------------------------------------------
// RecentCommits
// ---------------------------------------------------------------------------

func TestRecentCommits_FiveCommitsGetThree(t *testing.T) {
	dir := setupRepo(t)
	messages := []string{"commit two", "commit three", "commit four", "commit five"}
	for _, msg := range messages {
		gitRun(t, dir, "commit", "--allow-empty", "-m", msg)
	}

	commits := worktreeinfo.RecentCommits(dir, 3)
	if len(commits) != 3 {
		t.Fatalf("len(commits) = %d, want 3", len(commits))
	}

	// Most recent first.
	if commits[0].Message != "commit five" {
		t.Errorf("commits[0].Message = %q, want 'commit five'", commits[0].Message)
	}
	if commits[1].Message != "commit four" {
		t.Errorf("commits[1].Message = %q, want 'commit four'", commits[1].Message)
	}
	if commits[2].Message != "commit three" {
		t.Errorf("commits[2].Message = %q, want 'commit three'", commits[2].Message)
	}

	// SHA should be non-empty (short hash).
	for i, c := range commits {
		if c.SHA == "" {
			t.Errorf("commits[%d].SHA is empty", i)
		}
	}
}

func TestRecentCommits_EmptyRepo(t *testing.T) {
	// Repo with no commits at all (orphan).
	dir := t.TempDir()
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	// Configure identity but make NO commits.
	gitRunUnconfigured := func(args ...string) {
		c := exec.Command("git", args...)
		c.Dir = dir
		c.CombinedOutput() //nolint:errcheck
	}
	gitRunUnconfigured("config", "user.email", "test@example.com")
	gitRunUnconfigured("config", "user.name", "Test User")

	commits := worktreeinfo.RecentCommits(dir, 3)
	if commits != nil {
		t.Errorf("commits = %v, want nil for empty repo", commits)
	}
}

func TestRecentCommits_NEqualsZero(t *testing.T) {
	dir := setupRepo(t)
	commits := worktreeinfo.RecentCommits(dir, 0)
	// n=0 passes -0 to git log; git treats it as "no limit" — but
	// our implementation returns nil/empty because git -0 outputs nothing.
	// Either nil or empty slice is acceptable; we just verify no panic.
	_ = commits
}

func TestRecentCommits_MultiWordMessage(t *testing.T) {
	dir := setupRepo(t)
	msg := "fix: handle the edge case with spaces in message"
	gitRun(t, dir, "commit", "--allow-empty", "-m", msg)

	commits := worktreeinfo.RecentCommits(dir, 1)
	if len(commits) == 0 {
		t.Fatal("expected 1 commit, got 0")
	}
	if commits[0].Message != msg {
		t.Errorf("message = %q, want %q", commits[0].Message, msg)
	}
}

// ---------------------------------------------------------------------------
// StashCount
// ---------------------------------------------------------------------------

func TestStashCount_NoStashes(t *testing.T) {
	dir := setupRepo(t)
	count := worktreeinfo.StashCount(dir)
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

func TestStashCount_TwoStashes(t *testing.T) {
	dir := setupRepo(t)

	// Create and commit a tracked file.
	f := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(f, []byte("initial content"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	gitRun(t, dir, "add", "file.txt")
	gitRun(t, dir, "commit", "-m", "add file.txt")

	// Stash 1: change to "stash-one".
	if err := os.WriteFile(f, []byte("stash-one"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	gitRun(t, dir, "stash")

	// Restore a different base so stash 2 has different content.
	if err := os.WriteFile(f, []byte("between stashes"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	gitRun(t, dir, "add", "file.txt")
	gitRun(t, dir, "commit", "-m", "update file between stashes")

	// Stash 2: change to "stash-two".
	if err := os.WriteFile(f, []byte("stash-two"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	gitRun(t, dir, "stash")

	count := worktreeinfo.StashCount(dir)
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

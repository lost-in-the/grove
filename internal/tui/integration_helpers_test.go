//go:build integration

package tui

import (
	"testing"

	"github.com/lost-in-the/grove/internal/testhelper"
)

// gitEnv returns environment variables for isolated git operations.
func gitEnv() []string {
	return testhelper.GitEnv()
}

// runGit executes a git command in the given directory with isolated config.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	return testhelper.RunGit(t, dir, args...)
}

// setupRailsFixture creates a realistic Rails 8 repo with grove config.
// Returns the repo path (symlink-resolved).
func setupRailsFixture(t *testing.T) string {
	t.Helper()
	return testhelper.SetupRailsFixture(t)
}

// setupRailsFixtureWithWorktrees creates a fixture and adds worktrees.
func setupRailsFixtureWithWorktrees(t *testing.T, names ...string) string {
	t.Helper()
	return testhelper.SetupRailsFixtureWithWorktrees(t, names...)
}

// setupRailsFixtureWithDirtyWorktree creates a fixture with a dirty worktree.
func setupRailsFixtureWithDirtyWorktree(t *testing.T) string {
	t.Helper()
	return testhelper.SetupRailsFixtureWithDirtyWorktree(t)
}

// setupRailsFixtureWithUpstream creates a fixture with a bare remote and upstream tracking.
func setupRailsFixtureWithUpstream(t *testing.T) string {
	t.Helper()
	return testhelper.SetupRailsFixtureWithUpstream(t)
}

func must(t *testing.T, err error) {
	t.Helper()
	testhelper.Must(t, err)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	testhelper.WriteFile(t, path, content)
}

// getUpstreamCounts is a thin test-only wrapper around getUpstreamInfo
// that returns just the (ahead, behind) counts for assertion simplicity.
func getUpstreamCounts(worktreePath string) (ahead, behind int) {
	a, b, _, _ := getUpstreamInfo(worktreePath)
	return a, b
}

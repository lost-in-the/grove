//go:build integration

// End-to-end coverage for `grove browse --json` covering the three
// resolution paths from PR #53 (cherry-picked from release/v0.7.0):
//   1. open PR found for the current branch  → source = "pr"
//   2. worktree name "issue-<n>-..." parses out a known issue → "issue"
//   3. neither — compare-page fallback                       → "compare"
//
// gh is stubbed on PATH so the test runs offline; `auth status`, `repo view`,
// `pr view`, and `issue view` are scripted per scenario via the GROVE_TEST_GH
// env var the fake reads.
package integration_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/lost-in-the/grove/internal/testhelper"
)

var (
	groveBinaryOnce sync.Once
	groveBinaryPath string
	groveBinaryErr  error
)

// buildGroveBinary compiles cmd/grove into a tempdir once per test run and
// returns the path. Subsequent callers reuse the same binary.
func buildGroveBinary(t *testing.T) string {
	t.Helper()
	groveBinaryOnce.Do(func() {
		root, err := repoRootDir()
		if err != nil {
			groveBinaryErr = err
			return
		}
		dir, err := os.MkdirTemp("", "grove-bin-*")
		if err != nil {
			groveBinaryErr = err
			return
		}
		groveBinaryPath = filepath.Join(dir, "grove")
		cmd := exec.Command("go", "build", "-o", groveBinaryPath, "./cmd/grove")
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			groveBinaryErr = &buildErr{out: out, err: err}
		}
	})
	if groveBinaryErr != nil {
		t.Fatalf("build grove binary: %v", groveBinaryErr)
	}
	return groveBinaryPath
}

type buildErr struct {
	out []byte
	err error
}

func (b *buildErr) Error() string { return b.err.Error() + "\n" + string(b.out) }

// repoRootDir walks up from CWD until it finds go.mod.
func repoRootDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

// installFakeGh writes a shell script named "gh" into a fresh tempdir and
// returns the directory. Callers prepend this dir to PATH so the script
// shadows the real gh CLI.
func installFakeGh(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	script := `#!/bin/sh
case "$1 $2" in
  "auth status") exit 0 ;;
  "repo view")
    echo "test-org/test-repo"
    exit 0 ;;
  "pr view")
    case "$GROVE_TEST_GH" in
      pr-found)
        echo '{"number":42,"title":"Test PR","state":"OPEN","url":"https://github.com/test-org/test-repo/pull/42"}'
        exit 0 ;;
      *) exit 1 ;;
    esac ;;
  "issue view")
    case "$GROVE_TEST_GH" in
      issue-found)
        echo '{"number":7,"title":"Test Issue","state":"OPEN","url":"https://github.com/test-org/test-repo/issues/7"}'
        exit 0 ;;
      *) exit 1 ;;
    esac ;;
esac
exit 1
`
	path := filepath.Join(dir, "gh")
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	return dir
}

// setupBrowseRepo prepares a temp grove project with a single worktree,
// returning the worktree path so tests can run grove browse from inside it.
// worktreeName is used as both the worktree short name and the branch name
// (worktree dir layout: <project>-<name>, branch: <name>).
func setupBrowseRepo(t *testing.T, worktreeName string) string {
	t.Helper()
	repo := testhelper.SetupRailsFixture(t)

	// Make a real linked worktree so `grove browse` running inside it
	// resolves a non-empty branch and short name.
	wtPath := filepath.Join(filepath.Dir(repo), "rails-app-"+worktreeName)
	testhelper.RunGit(t, repo, "worktree", "add", "-b", worktreeName, wtPath)

	// Initialize .grove with a state.json that knows about both worktrees so
	// Manager.GetCurrent doesn't trip over an empty store.
	groveDir := filepath.Join(repo, ".grove")
	state := []byte(`{"version": 1, "worktrees": {}}`)
	if err := os.WriteFile(filepath.Join(groveDir, "state.json"), state, 0644); err != nil {
		t.Fatalf("write state.json: %v", err)
	}

	return wtPath
}

func runGroveBrowse(t *testing.T, wtPath, fakeGhDir, scenario string) (url, source string) {
	t.Helper()
	binary := buildGroveBinary(t)

	cmd := exec.Command(binary, "browse", "--json")
	cmd.Dir = wtPath
	cmd.Env = append(os.Environ(),
		"PATH="+fakeGhDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"GROVE_TEST_GH="+scenario,
	)

	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			t.Fatalf("grove browse failed: %v\nstderr: %s\nstdout: %s", err, ee.Stderr, out)
		}
		t.Fatalf("grove browse failed: %v\nstdout: %s", err, out)
	}

	var result struct {
		URL    string `json:"url"`
		Source string `json:"source"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("parse JSON output: %v\nraw: %s", err, out)
	}
	return result.URL, result.Source
}

func TestGroveBrowse_PRFound(t *testing.T) {
	wtPath := setupBrowseRepo(t, "feat-test")
	fakeGh := installFakeGh(t)

	url, source := runGroveBrowse(t, wtPath, fakeGh, "pr-found")

	if source != "pr" {
		t.Errorf("source = %q, want %q", source, "pr")
	}
	if url != "https://github.com/test-org/test-repo/pull/42" {
		t.Errorf("url = %q, want PR URL", url)
	}
}

func TestGroveBrowse_IssueFromWorktreeName(t *testing.T) {
	// Worktree name shaped as "issue-<n>-..." triggers the issue lookup path.
	wtPath := setupBrowseRepo(t, "issue-7-fix-something")
	fakeGh := installFakeGh(t)

	url, source := runGroveBrowse(t, wtPath, fakeGh, "issue-found")

	if source != "issue" {
		t.Errorf("source = %q, want %q", source, "issue")
	}
	if url != "https://github.com/test-org/test-repo/issues/7" {
		t.Errorf("url = %q, want issue URL", url)
	}
}

func TestGroveBrowse_CompareFallback(t *testing.T) {
	wtPath := setupBrowseRepo(t, "feat-no-pr")
	fakeGh := installFakeGh(t)

	url, source := runGroveBrowse(t, wtPath, fakeGh, "no-match")

	if source != "compare" {
		t.Errorf("source = %q, want %q", source, "compare")
	}
	want := "https://github.com/test-org/test-repo/compare/feat-no-pr"
	if url != want {
		t.Errorf("url = %q, want %q", url, want)
	}
}

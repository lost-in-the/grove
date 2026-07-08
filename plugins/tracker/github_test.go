package tracker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// installFakeGh shadows the real gh CLI with a script that mimics gh's
// argument handling for `pr view`: any --head flag is rejected (the real
// gh pr view has no such flag), and the branch must be positional.
func installFakeGh(t *testing.T, script string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake gh shell script not supported on windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "gh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// fakeGhPRView mimics real gh: `pr view` rejects --head (unknown flag) and
// returns PR JSON only when the branch is passed positionally.
const fakeGhPRView = `#!/bin/sh
if [ "$1 $2" != "pr view" ]; then
  echo "unexpected command: $@" >&2
  exit 1
fi
shift 2
for arg in "$@"; do
  if [ "$arg" = "--head" ]; then
    echo "unknown flag: --head" >&2
    exit 1
  fi
done
if [ "$1" = "feature-branch" ]; then
  echo '{"number":42,"title":"Test PR","state":"OPEN","url":"https://github.com/test-org/test-repo/pull/42"}'
  exit 0
fi
echo "no pull requests found for branch \"$1\"" >&2
exit 1
`

// Regression test for the --head bug: gh pr view has no --head flag, so the
// old invocation always errored and GetPRForBranch always returned nil.
func TestGetPRForBranch_PositionalBranch(t *testing.T) {
	installFakeGh(t, fakeGhPRView)

	adapter := NewGitHubAdapter("test-org/test-repo")
	pr, err := adapter.GetPRForBranch("feature-branch")
	if err != nil {
		t.Fatalf("GetPRForBranch() error = %v", err)
	}
	if pr == nil {
		t.Fatal("GetPRForBranch() = nil, want PR #42")
	}
	if pr.Number != 42 {
		t.Errorf("pr.Number = %d, want 42", pr.Number)
	}
	if pr.State != "open" {
		t.Errorf("pr.State = %q, want %q", pr.State, "open")
	}
	if pr.URL != "https://github.com/test-org/test-repo/pull/42" {
		t.Errorf("pr.URL = %q", pr.URL)
	}
}

func TestGetPRForBranch_NoPR(t *testing.T) {
	installFakeGh(t, fakeGhPRView)

	adapter := NewGitHubAdapter("test-org/test-repo")
	pr, err := adapter.GetPRForBranch("branch-without-pr")
	if err != nil {
		t.Fatalf("GetPRForBranch() error = %v, want nil (no PR is not an error)", err)
	}
	if pr != nil {
		t.Errorf("GetPRForBranch() = %+v, want nil", pr)
	}
}

func TestNewGitHubAdapter(t *testing.T) {
	adapter := NewGitHubAdapter("owner/repo")
	if adapter == nil {
		t.Fatal("NewGitHubAdapter() returned nil")
	}
	if adapter.repo != "owner/repo" {
		t.Errorf("adapter.repo = %v, want owner/repo", adapter.repo)
	}
}

func TestGitHubAdapter_Name(t *testing.T) {
	adapter := NewGitHubAdapter("")
	if adapter.Name() != "github" {
		t.Errorf("Name() = %v, want github", adapter.Name())
	}
}

func TestIsGHInstalled(t *testing.T) {
	// This test checks if gh is installed
	// It will pass if gh is installed and authenticated
	installed := IsGHInstalled()
	t.Logf("gh CLI installed and authenticated: %v", installed)

	// We don't fail if gh is not installed, just log the result
	if !installed {
		t.Skip("gh CLI not installed or not authenticated")
	}
}

// Integration tests below require gh CLI to be installed and authenticated
// They also require access to a GitHub repository

func TestGitHubAdapter_FetchIssue_Integration(t *testing.T) {
	if !IsGHInstalled() {
		t.Skip("gh CLI not installed or not authenticated")
	}

	// Use a public repo with known issues for testing
	_ = NewGitHubAdapter("cli/cli")

	// This test would need a known issue number
	// We'll skip it in automated tests
	t.Skip("Integration test - requires manual setup")
}

func TestGitHubAdapter_FetchPR_Integration(t *testing.T) {
	if !IsGHInstalled() {
		t.Skip("gh CLI not installed or not authenticated")
	}

	// Use a public repo with known PRs for testing
	_ = NewGitHubAdapter("cli/cli")

	// This test would need a known PR number
	// We'll skip it in automated tests
	t.Skip("Integration test - requires manual setup")
}

func TestGitHubAdapter_ListIssues_Integration(t *testing.T) {
	if !IsGHInstalled() {
		t.Skip("gh CLI not installed or not authenticated")
	}

	adapter := NewGitHubAdapter("cli/cli")

	opts := ListOptions{
		State: "open",
		Limit: 5,
	}

	issues, err := adapter.ListIssues(opts)
	if err != nil {
		t.Skipf("ListIssues() error (expected in CI): %v", err)
	}

	if len(issues) == 0 {
		t.Log("No issues found (may be expected)")
	} else {
		t.Logf("Found %d issues", len(issues))
		for _, issue := range issues {
			t.Logf("  #%d: %s", issue.Number, issue.Title)
		}
	}
}

func TestGitHubAdapter_ListPRs_Integration(t *testing.T) {
	if !IsGHInstalled() {
		t.Skip("gh CLI not installed or not authenticated")
	}

	adapter := NewGitHubAdapter("cli/cli")

	opts := ListOptions{
		State: "open",
		Limit: 5,
	}

	prs, err := adapter.ListPRs(opts)
	if err != nil {
		t.Skipf("ListPRs() error (expected in CI): %v", err)
	}

	if len(prs) == 0 {
		t.Log("No PRs found (may be expected)")
	} else {
		t.Logf("Found %d PRs", len(prs))
		for _, pr := range prs {
			t.Logf("  #%d: %s (branch: %s)", pr.Number, pr.Title, pr.Branch)
		}
	}
}

func TestDetectRepo_Integration(t *testing.T) {
	if !IsGHInstalled() {
		t.Skip("gh CLI not installed or not authenticated")
	}

	// This test requires being run in a git repository with a GitHub remote
	repo, err := DetectRepo()
	if err != nil {
		t.Skipf("DetectRepo() error (expected if not in a repo): %v", err)
	}

	t.Logf("Detected repo: %s", repo)

	if repo == "" {
		t.Error("DetectRepo() returned empty string")
	}
}

func TestGhIssue_ToIssue(t *testing.T) {
	raw := `{
		"number": 7,
		"title": "Test Issue",
		"body": "body text",
		"state": "OPEN",
		"author": {"login": "octocat"},
		"labels": [{"name": "bug"}, {"name": "help wanted"}],
		"url": "https://github.com/test-org/test-repo/issues/7"
	}`
	var gh ghIssue
	if err := json.Unmarshal([]byte(raw), &gh); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	issue := gh.toIssue()
	if issue.Number != 7 {
		t.Errorf("Number = %d, want 7", issue.Number)
	}
	if issue.State != "open" {
		t.Errorf("State = %q, want %q (should be lowercased)", issue.State, "open")
	}
	if issue.Author != "octocat" {
		t.Errorf("Author = %q, want octocat", issue.Author)
	}
	if len(issue.Labels) != 2 || issue.Labels[0] != "bug" || issue.Labels[1] != "help wanted" {
		t.Errorf("Labels = %v, want [bug, help wanted]", issue.Labels)
	}
}

func TestGhPR_ToPullRequest(t *testing.T) {
	raw := `{
		"number": 42,
		"title": "Test PR",
		"state": "MERGED",
		"author": {"login": "octocat"},
		"labels": [{"name": "enhancement"}],
		"headRefName": "feature-branch",
		"baseRefName": "main",
		"isDraft": true,
		"commits": [
			{"oid": "0123456789abcdef0123456789abcdef01234567", "messageHeadline": "first"},
			{"oid": "abc1234", "messageHeadline": "short sha kept as-is"}
		],
		"additions": 10,
		"deletions": 3,
		"reviewDecision": "APPROVED",
		"url": "https://github.com/test-org/test-repo/pull/42"
	}`
	var gh ghPR
	if err := json.Unmarshal([]byte(raw), &gh); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	pr := gh.toPullRequest()
	if pr.Number != 42 {
		t.Errorf("Number = %d, want 42", pr.Number)
	}
	if pr.State != "merged" {
		t.Errorf("State = %q, want %q (should be lowercased)", pr.State, "merged")
	}
	if pr.Branch != "feature-branch" || pr.BaseBranch != "main" {
		t.Errorf("Branch = %q, BaseBranch = %q", pr.Branch, pr.BaseBranch)
	}
	if !pr.IsDraft {
		t.Error("IsDraft = false, want true")
	}
	if pr.CommitCount != 2 || len(pr.Commits) != 2 {
		t.Fatalf("CommitCount = %d, Commits = %v", pr.CommitCount, pr.Commits)
	}
	if pr.Commits[0].SHA != "0123456" {
		t.Errorf("Commits[0].SHA = %q, want truncated %q", pr.Commits[0].SHA, "0123456")
	}
	if pr.Commits[1].SHA != "abc1234" {
		t.Errorf("Commits[1].SHA = %q, want %q", pr.Commits[1].SHA, "abc1234")
	}
	if pr.Additions != 10 || pr.Deletions != 3 {
		t.Errorf("Additions = %d, Deletions = %d", pr.Additions, pr.Deletions)
	}
	if pr.ReviewDecision != "APPROVED" {
		t.Errorf("ReviewDecision = %q, want APPROVED", pr.ReviewDecision)
	}
	if len(pr.Labels) != 1 || pr.Labels[0] != "enhancement" {
		t.Errorf("Labels = %v, want [enhancement]", pr.Labels)
	}
}

package tracker

import (
	"testing"
)

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

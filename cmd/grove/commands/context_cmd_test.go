package commands

import (
	"testing"
)

func TestContextCmd(t *testing.T) {
	if contextCmd == nil {
		t.Fatal("contextCmd is nil")
	}

	if contextCmd.Use != "context" {
		t.Errorf("contextCmd.Use = %q, want %q", contextCmd.Use, "context")
	}

	if contextCmd.RunE == nil {
		t.Error("contextCmd.RunE is nil")
	}

	// Verify alias
	found := false
	for _, a := range contextCmd.Aliases {
		if a == "ctx" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected alias 'ctx' to be registered")
	}
}

func TestContextFlags(t *testing.T) {
	flags := contextCmd.Flags()

	jsonFlag := flags.Lookup("json")
	if jsonFlag == nil {
		t.Error("expected --json flag to exist")
	}
}

func TestGetStashCount_InvalidPath(t *testing.T) {
	// An invalid path should return 0 (git fails, we treat as no stashes).
	count := getStashCount("/nonexistent/path")
	if count != 0 {
		t.Errorf("getStashCount with invalid path = %d, want 0", count)
	}
}

func TestGetRemoteInfo_NoUpstream(t *testing.T) {
	// A path with no upstream should return an empty remoteInfo without panicking.
	info := getRemoteInfo("/nonexistent/path")
	if info.Tracking != "" {
		t.Errorf("getRemoteInfo with invalid path Tracking = %q, want empty", info.Tracking)
	}
	if info.Ahead != 0 || info.Behind != 0 {
		t.Errorf("getRemoteInfo with invalid path Ahead=%d Behind=%d, want 0/0", info.Ahead, info.Behind)
	}
}

func TestGetRecentCommits_InvalidPath(t *testing.T) {
	// An invalid path should return nil commits without panicking.
	commits := getRecentCommits("/nonexistent/path", recentCommitsLimit)
	if commits != nil {
		t.Errorf("getRecentCommits with invalid path = %v, want nil", commits)
	}
}

func TestContextOutputFields(t *testing.T) {
	// Verify contextOutput struct fields round-trip correctly.
	out := contextOutput{
		Name:   "test",
		Status: statusClean,
	}

	if out.Name != "test" {
		t.Errorf("contextOutput.Name = %q, want %q", out.Name, "test")
	}
	if out.Status != statusClean {
		t.Errorf("contextOutput.Status = %q, want %q", out.Status, statusClean)
	}
}

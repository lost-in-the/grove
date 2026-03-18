package tui

import (
	"testing"

	"github.com/lost-in-the/grove/plugins/tracker"
)

func TestPRViewStateDetailFocusToggle(t *testing.T) {
	prs := []*tracker.PullRequest{
		{Number: 1, Title: "Test PR", Branch: "test", Author: "user", Body: "Body"},
	}
	s := &PRViewState{
		PRs: prs,
	}

	if len(s.PRs) != 1 {
		t.Errorf("PRs length = %d, want 1", len(s.PRs))
	}

	// Initially not focused
	if s.DetailFocused {
		t.Error("DetailFocused should be false initially")
	}

	// Toggle on
	s.DetailFocused = true
	if !s.DetailFocused {
		t.Error("DetailFocused should be true after toggle")
	}

	// Toggle off
	s.DetailFocused = false
	if s.DetailFocused {
		t.Error("DetailFocused should be false after second toggle")
	}
}

package tui

import (
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/mux"
)

func TestRenderAgentValue(t *testing.T) {
	tests := []struct {
		name   string
		status mux.AgentStatus
		want   string
	}{
		{name: "blocked", status: mux.AgentBlocked, want: "blocked"},
		{name: "done", status: mux.AgentDone, want: "done"},
		{name: "working", status: mux.AgentWorking, want: "working"},
		{name: "idle", status: mux.AgentIdle, want: "idle"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderAgentValue(&WorktreeItem{AgentStatus: tt.status})
			if !strings.Contains(got, tt.want) {
				t.Errorf("renderAgentValue(%q) = %q, want it to contain %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestRenderAgentValueUnreported(t *testing.T) {
	// Under tmux no session reports an agent, so the detail row must not
	// appear at all rather than rendering an empty-looking badge.
	for _, status := range []mux.AgentStatus{mux.AgentUnreported, mux.AgentUnknown} {
		if got := renderAgentValue(&WorktreeItem{AgentStatus: status}); got != "" {
			t.Errorf("renderAgentValue(%q) = %q, want empty", status, got)
		}
	}
}

func TestApplySessionRecordsAgent(t *testing.T) {
	ix := mux.NewIndex([]mux.Session{
		{
			Name:   "grove-testing",
			ID:     "w2",
			Path:   "/repos/grove-testing",
			Status: mux.StatusDetached,
			Agent:  mux.AgentBlocked,
		},
	})

	item := itemForTest("grove-testing", "testing", "/repos/grove-testing")
	applySession(item, ix, "grove", "herdr")

	if item.TmuxStatus != "detached" || item.SessionBackend != "herdr" {
		t.Errorf("TmuxStatus = %q, backend %q; want detached, herdr", item.TmuxStatus, item.SessionBackend)
	}
	if item.AgentStatus != mux.AgentBlocked {
		t.Errorf("AgentStatus = %q, want blocked", item.AgentStatus)
	}
}

func TestApplySessionWithNilIndex(t *testing.T) {
	// A failed listing degrades to "no sessions" rather than panicking.
	item := itemForTest("grove-testing", "testing", "/repos/grove-testing")
	item.AgentStatus = mux.AgentBlocked
	applySession(item, nil, "grove", "herdr")

	if item.TmuxStatus != "none" || item.AgentStatus != mux.AgentUnreported {
		t.Errorf("nil index set status: tmux=%q agent=%q", item.TmuxStatus, item.AgentStatus)
	}
}

// itemForTest builds the minimal item applySession reads.
func itemForTest(fullName, shortName, path string) *WorktreeItem {
	return &WorktreeItem{FullName: fullName, ShortName: shortName, Path: path}
}

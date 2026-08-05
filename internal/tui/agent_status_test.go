package tui

import (
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/mux"
	"github.com/lost-in-the/grove/internal/worktree"
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

func TestSetTmuxStatusRecordsAgent(t *testing.T) {
	fc := fetchContext{
		projectName: "grove",
		sessions: mux.NewIndex([]mux.Session{
			{
				Name:   "grove-testing",
				ID:     "w2",
				Path:   "/repos/grove-testing",
				Status: mux.StatusDetached,
				Agent:  mux.AgentBlocked,
			},
		}),
	}

	item := &WorktreeItem{}
	fc.setTmuxStatus(item, worktreeForTest("grove-testing", "testing", "/repos/grove-testing"))

	if item.TmuxStatus != "detached" {
		t.Errorf("TmuxStatus = %q, want detached", item.TmuxStatus)
	}
	if item.AgentStatus != mux.AgentBlocked {
		t.Errorf("AgentStatus = %q, want blocked", item.AgentStatus)
	}
}

func TestSetTmuxStatusWithNilIndex(t *testing.T) {
	// A failed listing degrades to "no sessions" rather than panicking.
	fc := fetchContext{projectName: "grove"}

	item := &WorktreeItem{}
	fc.setTmuxStatus(item, worktreeForTest("grove-testing", "testing", "/repos/grove-testing"))

	if item.TmuxStatus != "" || item.AgentStatus != mux.AgentUnreported {
		t.Errorf("nil index set status: tmux=%q agent=%q", item.TmuxStatus, item.AgentStatus)
	}
}

// worktreeForTest builds the minimal worktree the status setter reads.
func worktreeForTest(name, shortName, path string) worktree.Worktree {
	return worktree.Worktree{Name: name, ShortName: shortName, Path: path}
}

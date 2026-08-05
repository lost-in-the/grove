package commands

import (
	"testing"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/mux"
	"github.com/lost-in-the/grove/internal/worktree"
)

func TestAgentStatusFor(t *testing.T) {
	tree := &worktree.Worktree{
		Name:      "grove-testing",
		ShortName: "testing",
		Path:      "/repos/grove-testing",
	}

	sessions := mux.NewIndex([]mux.Session{
		{Name: "grove-testing", ID: "w2", Path: "/repos/grove-testing", Agent: mux.AgentBlocked},
	})

	if got := agentStatusFor(tree, "grove", sessions); got != mux.AgentBlocked {
		t.Errorf("agentStatusFor() = %q, want %q", got, mux.AgentBlocked)
	}
}

func TestAgentStatusForUnknownTree(t *testing.T) {
	tree := &worktree.Worktree{Name: "grove-other", ShortName: "other", Path: "/repos/grove-other"}
	sessions := mux.NewIndex([]mux.Session{
		{Name: "grove-testing", ID: "w2", Path: "/repos/grove-testing", Agent: mux.AgentBlocked},
	})

	if got := agentStatusFor(tree, "grove", sessions); got != mux.AgentUnreported {
		t.Errorf("agentStatusFor() = %q, want unreported", got)
	}
}

func TestAgentStatusForNilIndex(t *testing.T) {
	// tmux reports no agent state at all, so the index is legitimately nil.
	tree := &worktree.Worktree{Name: "grove-testing", ShortName: "testing", Path: "/repos/grove-testing"}

	if got := agentStatusFor(tree, "grove", nil); got != mux.AgentUnreported {
		t.Errorf("agentStatusFor(nil) = %q, want unreported", got)
	}
}

func TestAgentStatusLevel(t *testing.T) {
	tests := []struct {
		agent mux.AgentStatus
		want  cli.StatusLevel
	}{
		// Blocked means an agent is waiting on the user — the one state worth
		// interrupting them for, so it gets the danger color.
		{mux.AgentBlocked, cli.StatusError},
		{mux.AgentDone, cli.StatusOK},
		{mux.AgentWorking, cli.StatusWarning},
		{mux.AgentIdle, cli.StatusNone},
		{mux.AgentUnknown, cli.StatusNone},
		{mux.AgentUnreported, cli.StatusNone},
	}

	for _, tt := range tests {
		if got := agentStatusLevel(tt.agent); got != tt.want {
			t.Errorf("agentStatusLevel(%q) = %q, want %q", tt.agent, got, tt.want)
		}
	}
}

func TestAgentStatusDisplay(t *testing.T) {
	// An unreported agent must render as empty, not as the word "unreported" —
	// under tmux every row would otherwise carry noise.
	if got := agentStatusDisplay(mux.AgentUnreported); got != "" {
		t.Errorf("agentStatusDisplay(unreported) = %q, want empty", got)
	}
	if got := agentStatusDisplay(mux.AgentBlocked); got != "blocked" {
		t.Errorf("agentStatusDisplay(blocked) = %q, want %q", got, "blocked")
	}
}

func TestAnyAgentReported(t *testing.T) {
	trees := []*worktree.Worktree{
		{Name: "grove-a", ShortName: "a", Path: "/repos/grove-a"},
		{Name: "grove-b", ShortName: "b", Path: "/repos/grove-b"},
	}

	none := mux.NewIndex([]mux.Session{
		{Name: "grove-a", Path: "/repos/grove-a"},
		{Name: "grove-b", Path: "/repos/grove-b"},
	})
	if anyAgentReported(trees, "grove", none) {
		t.Error("anyAgentReported() = true when no session reports an agent")
	}

	some := mux.NewIndex([]mux.Session{
		{Name: "grove-a", Path: "/repos/grove-a"},
		{Name: "grove-b", Path: "/repos/grove-b", Agent: mux.AgentWorking},
	})
	if !anyAgentReported(trees, "grove", some) {
		t.Error("anyAgentReported() = false when a session reports an agent")
	}
}

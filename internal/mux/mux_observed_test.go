package mux

import "testing"

func TestAgentStatusObserved(t *testing.T) {
	observed := map[AgentStatus]bool{
		AgentIdle:       true,
		AgentWorking:    true,
		AgentBlocked:    true,
		AgentDone:       true,
		AgentUnknown:    false,
		AgentUnreported: false,
	}
	for status, want := range observed {
		if got := status.Observed(); got != want {
			t.Errorf("%q.Observed() = %v, want %v", status, got, want)
		}
	}
}

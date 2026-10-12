package tui

import "testing"

func TestWIPStrategyConstants(t *testing.T) {
	if WIPMove != 0 || WIPCopy != 1 || WIPLeave != 2 {
		t.Error("unexpected WIP strategy constant values")
	}
}

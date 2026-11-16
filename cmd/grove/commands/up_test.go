package commands

import (
	"strings"
	"testing"
)

// The stack commands describe themselves in the stack vocabulary, matching
// the README's Docker table.
func TestStackCommandsShortHelp(t *testing.T) {
	for _, tc := range []struct {
		name, got, want string
	}{
		{"up", upCmd.Short, "Bring the stack to this worktree"},
		{"down", downCmd.Short, "Stop the stack"},
		{"logs", logsCmd.Short, "Tail the stack's logs"},
		{"kick", restartCmd.Short, "Restart the stack's services"},
	} {
		if tc.got != tc.want {
			t.Errorf("grove %s Short = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}

// Long help names the stack, not "containers", where it means the stack.
func TestStackCommandsLongHelpSayStack(t *testing.T) {
	for name, long := range map[string]string{"up": upCmd.Long, "logs": logsCmd.Long, "kick": restartCmd.Long} {
		first := strings.SplitN(long, "\n", 2)[0]
		if !strings.Contains(first, "the stack") || strings.Contains(first, "container") {
			t.Errorf("grove %s Long opens with %q, want it to name the stack", name, first)
		}
	}
}

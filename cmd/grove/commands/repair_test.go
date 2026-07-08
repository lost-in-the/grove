package commands

import "testing"

func TestExtractQuotedName(t *testing.T) {
	tests := []struct {
		name string
		desc string
		want string
	}{
		{name: "state entry", desc: "State entry 'feature-x' points to missing directory: /tmp/x", want: "feature-x"},
		{name: "tmux session", desc: "Tmux session 'grove-web-checkout' has no corresponding worktree", want: "grove-web-checkout"},
		{name: "no quotes", desc: "no quoted name here", want: ""},
		{name: "single quote only", desc: "only one 'quote here", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractQuotedName(tt.desc); got != tt.want {
				t.Errorf("extractQuotedName(%q) = %q, want %q", tt.desc, got, tt.want)
			}
		})
	}
}

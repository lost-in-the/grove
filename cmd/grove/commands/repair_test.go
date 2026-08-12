package commands

import "testing"

func TestSessionWorktreeName(t *testing.T) {
	tests := []struct {
		name        string
		project     string
		session     string
		wantName    string
		wantMatched bool
	}{
		{
			name:        "own worktree session",
			project:     "grove",
			session:     "grove-feature-x",
			wantName:    "feature-x",
			wantMatched: true,
		},
		{
			name:        "no prefix match at all",
			project:     "grove",
			session:     "unrelated-session",
			wantMatched: false,
		},
		{
			name:        "sibling project's own root session",
			project:     "grove",
			session:     "grove-web",
			// "web" round-trips to "grove-web" via TmuxSessionName, so this
			// still can't be distinguished from a same-named worktree of
			// this project — the residual ambiguity the per-session
			// confirmation exists to cover.
			wantName:    "web",
			wantMatched: true,
		},
		{
			name:    "sibling project's root session named after the reserved 'root' worktree",
			project: "grove",
			session: "grove-root",
			// "root" is reserved (internal/worktree/naming.go) so a real
			// worktree of this project can never produce this session name —
			// TmuxSessionName("grove", "root") returns "grove", not
			// "grove-root". This session cannot be ours.
			wantMatched: false,
		},
		{
			name:        "manually named user session sharing the prefix",
			project:     "grove",
			session:     "grove-notes",
			wantName:    "notes",
			wantMatched: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotMatched := sessionWorktreeName(tt.project, tt.session)
			if gotMatched != tt.wantMatched {
				t.Fatalf("sessionWorktreeName(%q, %q) matched = %v, want %v", tt.project, tt.session, gotMatched, tt.wantMatched)
			}
			if gotMatched && gotName != tt.wantName {
				t.Errorf("sessionWorktreeName(%q, %q) name = %q, want %q", tt.project, tt.session, gotName, tt.wantName)
			}
		})
	}
}

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

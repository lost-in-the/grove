package commands

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/mux"
	"github.com/lost-in-the/grove/internal/worktree"
)

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
			name:    "sibling project's own root session",
			project: "grove",
			session: "grove-web",
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
		{name: "tmux session", desc: "Session 'grove-web-checkout' has no corresponding worktree", want: "grove-web-checkout"},
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

func repairWorktrees() []*worktree.Worktree {
	return []*worktree.Worktree{
		{Path: "/repos/grove", ShortName: "main", IsMain: true},
		{Path: "/repos/grove-alpha", ShortName: "alpha"},
	}
}

func orphanIDs(sessions []mux.Session) []string {
	ids := []string{}
	for _, s := range sessions {
		ids = append(ids, s.ID)
	}
	return ids
}

func TestOrphanSessionsDecidesByPathWhenTheBackendReportsOne(t *testing.T) {
	tests := []struct {
		name    string
		session mux.Session
		want    []string
	}{
		{
			name:    "relabelled session of a registered worktree is not an orphan",
			session: mux.Session{ID: "w1", Name: "grove-renamed-by-the-user", Path: "/repos/grove-alpha", Repo: "/repos/grove"},
			want:    []string{},
		},
		{
			name:    "another repository's session with this project's prefix is never offered",
			session: mux.Session{ID: "w2", Name: "grove-checkout", Path: "/repos/grove-web-checkout", Repo: "/repos/grove-web"},
			want:    []string{},
		},
		{
			name:    "session with a path but no repository is not provably ours",
			session: mux.Session{ID: "w3", Name: "grove-gone", Path: "/elsewhere/grove-gone"},
			want:    []string{},
		},
		{
			name:    "session of this repository whose checkout is no longer a worktree is an orphan",
			session: mux.Session{ID: "w4", Name: "anything", Path: "/repos/grove-gone", Repo: "/repos/grove"},
			want:    []string{"w4"},
		},
		{
			name:    "the repository's own session is not an orphan",
			session: mux.Session{ID: "w5", Name: "grove", Path: "/repos/grove", Repo: "/repos/grove"},
			want:    []string{},
		},
		{
			// herdr never updates a workspace's recorded checkout, so after
			// `grove rename` the live worktree's session still names the old
			// directory and is reachable by its label only.
			name:    "renamed worktree's session with a stale path is not an orphan",
			session: mux.Session{ID: "w6", Name: "grove-alpha", Path: "/repos/grove-before-rename", Repo: "/repos/grove"},
			want:    []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := orphanIDs(orphanSessions("grove", []mux.Session{tt.session}, repairWorktrees()))
			if !slices.Equal(got, tt.want) {
				t.Errorf("orphanSessions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOrphanSessionsMatchesPathsThroughSymlinks(t *testing.T) {
	real := t.TempDir()
	checkout := filepath.Join(real, "grove-alpha")
	if err := os.Mkdir(checkout, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	worktrees := []*worktree.Worktree{
		{Path: filepath.Join(link, "grove"), ShortName: "main", IsMain: true},
		{Path: filepath.Join(link, "grove-alpha"), ShortName: "alpha"},
	}
	sessions := []mux.Session{
		{ID: "w1", Name: "relabelled", Path: checkout, Repo: filepath.Join(real, "grove")},
		{ID: "w2", Name: "grove-gone", Path: filepath.Join(real, "grove-gone"), Repo: filepath.Join(real, "grove")},
	}

	got := orphanIDs(orphanSessions("grove", sessions, worktrees))
	if want := []string{"w2"}; !slices.Equal(got, want) {
		t.Errorf("orphanSessions() = %v, want %v: w1's path is a registered worktree, w2's repository is this one", got, want)
	}
}

func TestOrphanSessionsFallsBackToTheLabelWithoutAPath(t *testing.T) {
	sessions := []mux.Session{
		{ID: "grove-alpha", Name: "grove-alpha"},
		{ID: "grove-gone", Name: "grove-gone"},
		{ID: "grove", Name: "grove"},
		{ID: "unrelated", Name: "unrelated"},
	}

	got := orphanIDs(orphanSessions("grove", sessions, repairWorktrees()))
	if want := []string{"grove-gone"}; !slices.Equal(got, want) {
		t.Errorf("orphanSessions() = %v, want %v", got, want)
	}
}

// sessionKillingMux closes one exact session, as herdr can by workspace id.
type sessionKillingMux struct {
	*fakeMux
	closed []mux.Session
}

func (s *sessionKillingMux) KillSession(sess mux.Session) error {
	s.closed = append(s.closed, sess)
	return nil
}

func TestKillOrphanSessionClosesTheExactSessionWhenTheBackendCan(t *testing.T) {
	m := &sessionKillingMux{fakeMux: &fakeMux{OffBackend: mux.NewOff(), backend: mux.BackendHerdr}}
	orphan := mux.Session{ID: "w4", Name: "grove-gone", Path: "/repos/grove-gone"}

	if err := killOrphanSession(m, orphan); err != nil {
		t.Fatalf("killOrphanSession() error = %v", err)
	}
	if len(m.closed) != 1 || m.closed[0].ID != "w4" {
		t.Errorf("KillSession calls = %v, want exactly w4", m.closed)
	}
	if len(m.killed) != 0 {
		t.Errorf("Kill calls = %v, want none: a label lookup could close another session", m.killed)
	}
}

func TestKillOrphanSessionPassesNameAndPathToANameKeyedBackend(t *testing.T) {
	m := &fakeMux{OffBackend: mux.NewOff(), backend: mux.BackendTmux}

	if err := killOrphanSession(m, mux.Session{ID: "grove-gone", Name: "grove-gone", Path: "/repos/grove-gone"}); err != nil {
		t.Fatalf("killOrphanSession() error = %v", err)
	}
	want := mux.Target{Name: "grove-gone", Path: "/repos/grove-gone"}
	if len(m.killed) != 1 || m.killed[0] != want {
		t.Errorf("Kill calls = %v, want %v", m.killed, want)
	}
}

func TestOrphanSessionIssueNamesTheBackendAndThePath(t *testing.T) {
	issue := orphanSessionIssue(mux.BackendHerdr, mux.Session{ID: "w4", Name: "grove-gone", Path: "/repos/grove-gone"})

	for _, want := range []string{"herdr session 'grove-gone'", "/repos/grove-gone"} {
		if !strings.Contains(issue.Description, want) {
			t.Errorf("Description = %q, want it to contain %q", issue.Description, want)
		}
	}
	if strings.Contains(issue.Description+issue.Action, "tmux") {
		t.Errorf("issue mentions tmux under the herdr backend: %q / %q", issue.Description, issue.Action)
	}
	if issue.Type != "orphan_tmux" {
		t.Errorf("Type = %q, want the existing identifier", issue.Type)
	}
}

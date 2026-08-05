package mux

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIndexLookupByName(t *testing.T) {
	// tmux reports no path, so lookup must fall back to the session name.
	ix := NewIndex([]Session{
		{Name: "grove-testing", ID: "grove-testing", Status: StatusAttached},
		{Name: "grove-main", ID: "grove-main", Status: StatusDetached},
	})

	got, ok := ix.Lookup(Target{Name: "grove-testing", Path: "/repos/grove-testing"})
	if !ok {
		t.Fatal("Lookup by name failed")
	}
	if got.Status != StatusAttached {
		t.Errorf("Status = %q, want %q", got.Status, StatusAttached)
	}
}

func TestIndexPrefersPathOverName(t *testing.T) {
	// herdr labels are cosmetic and user-renameable. When a path match and a
	// name match disagree, the path is authoritative.
	ix := NewIndex([]Session{
		{Name: "grove-testing", ID: "w1", Path: "/repos/grove-other", Status: StatusDetached},
		{Name: "renamed-by-user", ID: "w2", Path: "/repos/grove-testing", Status: StatusAttached},
	})

	got, ok := ix.Lookup(Target{Name: "grove-testing", Path: "/repos/grove-testing"})
	if !ok {
		t.Fatal("Lookup failed")
	}
	if got.ID != "w2" {
		t.Errorf("ID = %q, want w2 (path match must win over name match)", got.ID)
	}
}

func TestIndexLookupNormalizesPaths(t *testing.T) {
	ix := NewIndex([]Session{
		{Name: "grove-testing", ID: "w1", Path: "/repos/grove-testing/", Status: StatusDetached},
	})

	got, ok := ix.Lookup(Target{Name: "other", Path: "/repos/./grove-testing"})
	if !ok {
		t.Fatal("Lookup with unnormalized path failed")
	}
	if got.ID != "w1" {
		t.Errorf("ID = %q, want w1", got.ID)
	}
}

func TestIndexLookupResolvesSymlinks(t *testing.T) {
	// herdr canonicalizes checkout paths; grove's configured projects_dir may
	// be reached through a symlink (the /tmp -> /private/tmp case on macOS).
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	ix := NewIndex([]Session{{Name: "grove-testing", ID: "w1", Path: real}})

	got, ok := ix.Lookup(Target{Name: "grove-testing", Path: link})
	if !ok {
		t.Fatal("Lookup through symlink failed")
	}
	if got.ID != "w1" {
		t.Errorf("ID = %q, want w1", got.ID)
	}
}

func TestIndexLookupMiss(t *testing.T) {
	ix := NewIndex([]Session{{Name: "grove-main", ID: "w1", Path: "/repos/grove"}})

	if _, ok := ix.Lookup(Target{Name: "grove-testing", Path: "/repos/grove-testing"}); ok {
		t.Error("Lookup returned a session for an unknown target")
	}
}

func TestIndexStatusFor(t *testing.T) {
	ix := NewIndex([]Session{{Name: "grove-main", ID: "w1", Status: StatusAttached}})

	if got := ix.StatusFor(Target{Name: "grove-main"}); got != StatusAttached {
		t.Errorf("StatusFor(known) = %q, want %q", got, StatusAttached)
	}
	if got := ix.StatusFor(Target{Name: "nope"}); got != StatusNone {
		t.Errorf("StatusFor(unknown) = %q, want %q", got, StatusNone)
	}
}

func TestNilIndexIsUsable(t *testing.T) {
	// Callers treat a failed List as "no sessions" rather than an error, so a
	// nil index must behave like an empty one.
	var ix *Index

	if _, ok := ix.Lookup(Target{Name: "grove-main"}); ok {
		t.Error("nil Index returned a session")
	}
	if got := ix.StatusFor(Target{Name: "grove-main"}); got != StatusNone {
		t.Errorf("nil Index StatusFor = %q, want %q", got, StatusNone)
	}
	if got := ix.AgentFor(Target{Name: "grove-main"}); got != AgentUnreported {
		t.Errorf("nil Index AgentFor = %q, want %q", got, AgentUnreported)
	}
}

func TestIndexAgentFor(t *testing.T) {
	ix := NewIndex([]Session{
		{Name: "grove-main", ID: "w1", Path: "/repos/grove", Agent: AgentBlocked},
		{Name: "grove-testing", ID: "w2", Path: "/repos/grove-testing"},
	})

	if got := ix.AgentFor(Target{Path: "/repos/grove"}); got != AgentBlocked {
		t.Errorf("AgentFor = %q, want %q", got, AgentBlocked)
	}
	if got := ix.AgentFor(Target{Path: "/repos/grove-testing"}); got != AgentUnreported {
		t.Errorf("AgentFor(no agent) = %q, want %q", got, AgentUnreported)
	}
}

func TestAgentStatusNeedsAttention(t *testing.T) {
	attention := map[AgentStatus]bool{
		AgentBlocked:    true,
		AgentDone:       true,
		AgentWorking:    false,
		AgentIdle:       false,
		AgentUnknown:    false,
		AgentUnreported: false,
	}
	for status, want := range attention {
		if got := status.NeedsAttention(); got != want {
			t.Errorf("%q.NeedsAttention() = %v, want %v", status, got, want)
		}
	}
}

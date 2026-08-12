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

// mkdirs creates real directories under one fresh temp dir and returns their
// paths in order. NewIndex ignores checkout paths that are not on disk, so a
// test describing a live session has to describe one that actually exists.
func mkdirs(t *testing.T, names ...string) []string {
	t.Helper()
	base := t.TempDir()
	paths := make([]string, len(names))
	for i, name := range names {
		p := filepath.Join(base, name)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", p, err)
		}
		paths[i] = p
	}
	return paths
}

func TestIndexPrefersPathOverName(t *testing.T) {
	// herdr labels are cosmetic and user-renameable. When a path match and a
	// name match disagree, the path is authoritative.
	dirs := mkdirs(t, "grove-other", "grove-testing")
	other, testing_ := dirs[0], dirs[1]

	ix := NewIndex([]Session{
		{Name: "grove-testing", ID: "w1", Path: other, Status: StatusDetached},
		{Name: "renamed-by-user", ID: "w2", Path: testing_, Status: StatusAttached},
	})

	got, ok := ix.Lookup(Target{Name: "grove-testing", Path: testing_})
	if !ok {
		t.Fatal("Lookup failed")
	}
	if got.ID != "w2" {
		t.Errorf("ID = %q, want w2 (path match must win over name match)", got.ID)
	}
}

func TestIndexLookupNormalizesPaths(t *testing.T) {
	dir := mkdirs(t, "grove-testing")[0]

	ix := NewIndex([]Session{
		{Name: "grove-testing", ID: "w1", Path: dir + "/", Status: StatusDetached},
	})

	unnormalized := filepath.Join(filepath.Dir(dir), ".", "grove-testing")
	got, ok := ix.Lookup(Target{Name: "other", Path: unnormalized})
	if !ok {
		t.Fatal("Lookup with unnormalized path failed")
	}
	if got.ID != "w1" {
		t.Errorf("ID = %q, want w1", got.ID)
	}
}

func TestIndexIgnoresVanishedCheckoutPaths(t *testing.T) {
	// herdr records a workspace's checkout when it opens and never updates it,
	// so after `grove rename` the workspace still names the pre-rename
	// directory. Indexing that dead path would let it shadow the workspace of
	// whatever worktree later occupies it — and because Lookup prefers paths,
	// the name fallback that makes rename self-heal would never run.
	live := mkdirs(t, "grove-beta")[0]
	vanished := filepath.Join(filepath.Dir(live), "grove-gone")

	ix := NewIndex([]Session{
		// The renamed workspace: label moved on, checkout path left behind and
		// now pointing at the directory `live` occupies.
		{Name: "grove-gamma", ID: "wStale", Path: vanished, Status: StatusDetached},
		{Name: "grove-beta", ID: "wLive", Path: live, Status: StatusAttached},
	})

	got, ok := ix.Lookup(Target{Name: "grove-beta", Path: live})
	if !ok {
		t.Fatal("Lookup of the live worktree failed")
	}
	if got.ID != "wLive" {
		t.Errorf("ID = %q, want wLive — a vanished path must not shadow a live one", got.ID)
	}

	// The stale workspace is still reachable, by name only. That is what makes
	// `grove rename` keep working against it.
	got, ok = ix.Lookup(Target{Name: "grove-gamma", Path: vanished})
	if !ok {
		t.Fatal("renamed workspace unreachable by name")
	}
	if got.ID != "wStale" {
		t.Errorf("ID = %q, want wStale", got.ID)
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
	dirs := mkdirs(t, "grove", "grove-testing")
	main, testing_ := dirs[0], dirs[1]

	ix := NewIndex([]Session{
		{Name: "grove-main", ID: "w1", Path: main, Agent: AgentBlocked},
		{Name: "grove-testing", ID: "w2", Path: testing_},
	})

	if got := ix.AgentFor(Target{Path: main}); got != AgentBlocked {
		t.Errorf("AgentFor = %q, want %q", got, AgentBlocked)
	}
	if got := ix.AgentFor(Target{Path: testing_}); got != AgentUnreported {
		t.Errorf("AgentFor(no agent) = %q, want %q", got, AgentUnreported)
	}
}

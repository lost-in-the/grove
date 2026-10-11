package tui

import "testing"

// A refresh that reorders rows while an overlay is open must not move the
// cursor: esc returns to the same worktree, not the same row.
func TestEscRestoresCursorToSameWorktreeAfterReorder(t *testing.T) {
	m := newTestModel(withItems(5), withSize(120, 40))
	m.list.Select(2)
	before, _ := m.selectedItem()
	if before.ShortName != "fix-bug" {
		t.Fatalf("setup: selected %q, want fix-bug", before.ShortName)
	}

	next, _ := m.Update(makeKeyMsg("R"))
	m = next.(Model)

	items := makeTestItems(5)
	reordered := []WorktreeItem{items[0], items[1], items[3], items[4], items[2]}
	next, _ = m.Update(worktreesFetchedMsg{items: reordered})
	m = next.(Model)

	next, _ = m.Update(makeKeyMsg("esc"))
	m = next.(Model)

	after, _ := m.selectedItem()
	if after.Path != before.Path {
		t.Errorf("after esc, selected %q; want %q", after.ShortName, before.ShortName)
	}
}

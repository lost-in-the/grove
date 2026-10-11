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

func TestHelpOpensAsFrameAndEscCloses(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	next, _ := m.Update(makeKeyMsg("?"))
	m = next.(Model)
	if m.activeView() != ViewHelp || !m.helpOverlay.Active {
		t.Fatalf("after ?: view=%v active=%v, want ViewHelp and true", m.activeView(), m.helpOverlay.Active)
	}
	next, _ = m.Update(makeKeyMsg("esc"))
	m = next.(Model)
	if m.activeView() != ViewDashboard || m.helpOverlay.Active {
		t.Errorf("after esc: view=%v active=%v, want ViewDashboard and false", m.activeView(), m.helpOverlay.Active)
	}
}

// The create wizard keeps ? for its text fields, so the reachable nested
// case is help over the delete prompt.
func TestHelpOverDelete_EscWalksBack(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40), withDeleteOverlay())
	next, _ := m.Update(makeKeyMsg("?"))
	m = next.(Model)
	if m.activeView() != ViewHelp {
		t.Fatalf("after ?: view=%v, want ViewHelp", m.activeView())
	}
	for _, want := range []ActiveView{ViewDelete, ViewDashboard} {
		next, _ = m.Update(makeKeyMsg("esc"))
		m = next.(Model)
		if m.activeView() != want {
			t.Fatalf("after esc: view=%v, want %v", m.activeView(), want)
		}
	}
}

func TestHelpKeepsFooterOfViewBeneath(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	before := m.helpFooter.CompactHeight(m.layoutView(), m.width-4)
	next, _ := m.Update(makeKeyMsg("?"))
	m = next.(Model)
	if after := m.helpFooter.CompactHeight(m.layoutView(), m.width-4); after != before {
		t.Errorf("footer height with help open = %d, want %d", after, before)
	}
}

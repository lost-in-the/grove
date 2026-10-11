package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

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

// An overlay draws over whatever frame it was opened from. Help is centered
// on top of the delete box and hides it, so the delete box is checked in the
// frame beneath help.
func TestRenderFrame_HelpOverDeleteDrawsDeleteBeneath(t *testing.T) {
	m := goldenModel(t, sizeUltraWide, withItems(5), withDeleteOverlay())
	withHelpExpanded()(&m)
	top := ansi.Strip(m.viewString())
	if !strings.Contains(top, "Toggle branch deletion") {
		t.Error("help panel missing")
	}
	if !strings.Contains(top, "test commit for root") {
		t.Error("dashboard rows missing beneath both overlays")
	}
	beneath := ansi.Strip(m.renderFrame(len(m.views) - 2))
	if !strings.Contains(beneath, `Delete "feature-auth"?`) {
		t.Errorf("delete box missing from the frame beneath help:\n%s", beneath)
	}
}

// Closing a frame over the same worktree must leave the detail pane alone:
// popping used to re-render it, which reset its scroll to the top.
func TestPop_KeepsDetailScrollWhenSelectionUnchanged(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m.detail.SetContent(strings.Repeat("detail line\n", 200))
	m.detail.SetYOffset(7)
	m.pushView(ViewHelp)
	m.popView()
	if got := m.detail.YOffset(); got != 7 {
		t.Errorf("detail scroll after pop = %d, want 7", got)
	}
}

// A focused detail pane keeps its focused rendering when help closes.
func TestPop_KeepsFocusedDetailRendering(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m.detailFocused = true
	m.refreshDetailForFocus()
	before := m.detail.View()
	m.pushView(ViewHelp)
	m.popView()
	if after := m.detail.View(); after != before {
		t.Error("focused detail pane re-rendered unfocused after pop")
	}
}

// When the selection does change on pop, the detail shows the new worktree
// from the top, in the focused style if the pane has focus.
func TestPop_RefreshesDetailWhenSelectionChanges(t *testing.T) {
	m := newTestModel(withItems(5), withSize(120, 40))
	m.detailFocused = true
	m.list.Select(2)
	m.pushView(ViewRename)
	m.list.Select(4)
	m.refreshDetailForFocus()
	m.detail.SetYOffset(3)
	m.popView()
	item, _ := m.selectedItem()
	if item.ShortName != "fix-bug" {
		t.Fatalf("selected %q after pop, want fix-bug", item.ShortName)
	}
	if got := m.detail.YOffset(); got != 0 {
		t.Errorf("detail scroll = %d, want 0 for a newly selected worktree", got)
	}
	m2 := m
	m2.refreshDetailForFocus()
	if m.detail.View() != m2.detail.View() {
		t.Error("detail after pop is not the focused rendering of the selected worktree")
	}
}

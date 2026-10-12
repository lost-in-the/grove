package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestLayoutTiers(t *testing.T) {
	for w, want := range map[int]Layout{0: LayoutGuard, 43: LayoutGuard, 44: LayoutList, 109: LayoutList, 110: LayoutSplit, 200: LayoutSplit} {
		if got := LayoutBreakpoint(w); got != want {
			t.Errorf("width %d: tier %v, want %v", w, got, want)
		}
	}
}

func TestLayout_DetailPaneFrom110(t *testing.T) {
	w := newTestModel(withItems(3), withSize(110, 30))
	w.updateDetailContent()
	wide := ansi.Strip(w.viewString())
	if !strings.Contains(wide, "╭─ root") {
		t.Errorf("110 columns should dock the detail pane:\n%s", wide)
	}
	n := newTestModel(withItems(3), withSize(109, 30))
	n.updateDetailContent()
	narrow := ansi.Strip(n.viewString())
	if strings.Contains(narrow, "╭─ root") {
		t.Errorf("109 columns should not show a detail card:\n%s", narrow)
	}
}

func TestLayout_SelectionLineBelow110(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	m.list.Select(1)
	lines := strings.Split(ansi.Strip(m.viewString()), "\n")
	footer := m.helpFooter.CompactHeight(ViewDashboard, m.width-4)
	sel := lines[len(lines)-1-footer]
	if !strings.Contains(sel, "feature-auth · test commit for feature-auth") {
		t.Errorf("selection line = %q", sel)
	}
	if len(lines) != 24 {
		t.Errorf("screen is %d lines, want 24", len(lines))
	}
}

func TestDetailSheet_TabAndLOpenEscCloses(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	m.list.Select(1)
	for _, open := range []string{"tab", "l"} {
		for _, close := range []string{"esc", "h", "tab"} {
			s := sendKey(m, open)
			if s.activeView() != ViewDetailSheet {
				t.Fatalf("%s: view %v, want the detail sheet", open, s.activeView())
			}
			if out := ansi.Strip(s.viewString()); !strings.Contains(out, "╭─ feature-auth") {
				t.Errorf("%s: sheet does not show the detail:\n%s", open, out)
			}
			s = sendKey(s, "j")
			if s.list.Index() != 1 {
				t.Errorf("j in the sheet moved the list to %d", s.list.Index())
			}
			s = sendKey(s, close)
			if s.activeView() != ViewDashboard || s.list.Index() != 1 {
				t.Errorf("%s then %s: view %v index %d, want dashboard at 1", open, close, s.activeView(), s.list.Index())
			}
		}
	}
}

func TestGuard_Under44(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	m = sendKey(m, "n")
	for _, r := range "abc" {
		m = sendKey(m, string(r))
	}
	next, _ := m.Update(tea.WindowSizeMsg{Width: 43, Height: 24})
	m = next.(Model)
	if got := strings.TrimSpace(ansi.Strip(m.viewString())); got != "grove needs ≥44 columns" {
		t.Errorf("guard = %q", got)
	}
	next, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = next.(Model)
	if m.activeView() != ViewBranchForm || m.branchForm.branchValue() != "abc" {
		t.Errorf("after widening: view %v branch %q, want the form with abc", m.activeView(), m.branchForm.branchValue())
	}
}

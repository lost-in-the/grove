package tui

import (
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

func TestNewWorktreeList_DisablesListQuit(t *testing.T) {
	l := newWorktreeList(rowDelegate{})
	if l.KeyMap.Quit.Enabled() {
		t.Fatal("list Quit binding is enabled; grove handles quitting, so the list's own binding must be off")
	}
}

// The list re-derives its bindings on SetItems, SetSize, and filter changes,
// so the guard must survive those, not just construction.
func TestNewWorktreeList_QuitStaysDisabledAfterListUpdates(t *testing.T) {
	l := newWorktreeList(rowDelegate{})
	l.SetItems([]list.Item{makeTestItems(1)[0]})
	l.SetSize(80, 24)
	l.SetFilteringEnabled(true)
	if l.KeyMap.Quit.Enabled() {
		t.Error("list Quit binding re-enabled after list updates")
	}
	if !l.KeyMap.ForceQuit.Enabled() {
		t.Error("list ForceQuit disabled; ctrl+c quits the dashboard through it")
	}
}

func TestNewWorktreeList_KeepsGroveBindings(t *testing.T) {
	l := newWorktreeList(rowDelegate{})
	if got := l.KeyMap.Filter.Keys(); len(got) != 1 || got[0] != "/" {
		t.Errorf("Filter keys = %v, want [/]", got)
	}
	if got := l.KeyMap.CursorUp.Keys(); len(got) != 2 || got[0] != "up" || got[1] != "k" {
		t.Errorf("CursorUp keys = %v, want [up k]", got)
	}
	if got := l.KeyMap.CursorDown.Keys(); len(got) != 2 || got[0] != "down" || got[1] != "j" {
		t.Errorf("CursorDown keys = %v, want [down j]", got)
	}
}

func TestDashboard_CtrlCQuits(t *testing.T) {
	m := newTestModel(withItems(1), withSize(80, 24))
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("ctrl+c returned no command; want quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("ctrl+c command produced %T, want tea.QuitMsg", cmd())
	}
}

// The list pages only on pgup/pgdown and jumps only on home/end, which
// leaves h l b u f d g G to grove (spec 6.2).
func TestNewWorktreeList_PagingKeys(t *testing.T) {
	l := newWorktreeList(rowDelegate{})
	for name, c := range map[string]struct {
		got  []string
		want string
	}{
		"PrevPage":  {l.KeyMap.PrevPage.Keys(), "pgup"},
		"NextPage":  {l.KeyMap.NextPage.Keys(), "pgdown"},
		"GoToStart": {l.KeyMap.GoToStart.Keys(), "home"},
		"GoToEnd":   {l.KeyMap.GoToEnd.Keys(), "end"},
	} {
		if len(c.got) != 1 || c.got[0] != c.want {
			t.Errorf("%s keys = %v, want [%s]", name, c.got, c.want)
		}
	}
}

func TestDashboard_MotionKeys(t *testing.T) {
	m := newTestModel(withItems(12), withSize(80, 24))
	m.updateLayout()
	m = sendKey(m, "G")
	if got := m.list.Index(); got != 11 {
		t.Errorf("G: index %d, want 11", got)
	}
	m = sendKey(m, "g")
	m = sendKey(m, "g")
	if got := m.list.Index(); got != 0 {
		t.Errorf("gg: index %d, want 0", got)
	}
	m = sendKey(m, "g")
	m = sendKey(m, "x")
	if got := m.list.Index(); got != 0 || m.pendingG {
		t.Errorf("g x: index %d pendingG %v, want 0 and cleared", got, m.pendingG)
	}
	half := m.halfPage()
	if half < 1 {
		t.Fatalf("halfPage = %d", half)
	}
	m = sendKey(m, "ctrl+d")
	if got := m.list.Index(); got != half {
		t.Errorf("ctrl+d: index %d, want %d", got, half)
	}
	m = sendKey(m, "ctrl+u")
	if got := m.list.Index(); got != 0 {
		t.Errorf("ctrl+u: index %d, want 0", got)
	}
}

func TestDashboard_UNoLongerPages(t *testing.T) {
	m := newTestModel(withItems(12), withSize(80, 24))
	m.updateLayout()
	m = sendKey(m, "pgdown")
	page := m.list.Paginator.Page
	m = sendKey(m, "u")
	if m.list.Paginator.Page != page {
		t.Errorf("u changed the page from %d to %d", page, m.list.Paginator.Page)
	}
}

func TestDashboard_LAndHFocusDetail(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m = sendKey(m, "l")
	if !m.detailFocused {
		t.Fatal("l should focus the detail pane")
	}
	m = sendKey(m, "h")
	if m.detailFocused {
		t.Error("h should return to the list")
	}
}

func TestDashboard_MotionKeysTypeIntoFilter(t *testing.T) {
	m := newTestModel(withItems(5), withSize(80, 24))
	m = sendKey(m, "/")
	m.pendingG = true // as if g had been pressed just before
	for _, k := range []string{"g", "G", "h", "l"} {
		m = sendKey(m, k)
	}
	if got := m.list.FilterValue(); got != "gGhl" {
		t.Errorf("filter text = %q, want the motion keys typed in", got)
	}
	if m.pendingG {
		t.Error("pendingG survived into filter typing")
	}
}

// ctrl+c quits even right after g.
func TestDashboard_CtrlCQuitsAfterG(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	m = sendKey(m, "g")
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("ctrl+c after g returned no command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("ctrl+c after g produced %T, want tea.QuitMsg", cmd())
	}
}

// A pending g does not outlive an overlay opened in between.
func TestDashboard_PendingGClearedByOverlay(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	m = sendKey(m, "g")
	m = sendKey(m, "?")
	m = sendKey(m, "esc")
	m = sendKey(m, "j")
	if got := m.list.Index(); got != 1 {
		t.Errorf("j after g, ?, esc: index %d, want 1", got)
	}
}

// Half a page is half the rows the list actually shows.
func TestDashboard_HalfPageIsHalfTheVisibleRows(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}} {
		m := newTestModel(withItems(60), withSize(size[0], size[1]))
		m.updateLayout()
		if want := max(1, m.list.Paginator.PerPage/2); m.halfPage() != want {
			t.Errorf("%dx%d: halfPage %d, want %d (PerPage %d)", size[0], size[1], m.halfPage(), want, m.list.Paginator.PerPage)
		}
	}
}

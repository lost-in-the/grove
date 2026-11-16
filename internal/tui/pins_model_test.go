package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// pinModel is a 120x40 dashboard with the given pins saved on disk, as the
// app has them after loading.
func pinModel(t *testing.T, pins pinSet) Model {
	t.Helper()
	m := newTestModel(withItems(4), withSize(120, 40))
	m.projectRoot = prefsRoot(t)
	if err := updateUIPrefs(m.projectRoot, func(p *UIPrefs) { p.Pins = pins }); err != nil {
		t.Fatal(err)
	}
	m.pins = pins
	m.refreshDelegate()
	return m
}

func pressKey(m Model, k string) (Model, tea.Cmd) {
	next, cmd := m.Update(makeKeyMsg(k))
	return next.(Model), cmd
}

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestPins_MThenDigitPins(t *testing.T) {
	m := pinModel(t, pinSet{})
	m.list.Select(1)
	m = sendKey(m, "m")
	m = sendKey(m, "3")
	if m.pins["3"] != "feature-auth" {
		t.Errorf("pins = %v, want slot 3 = feature-auth", m.pins)
	}
	if saved := loadUIPrefs(m.projectRoot); saved == nil || saved.Pins["3"] != "feature-auth" {
		t.Errorf("saved prefs = %+v", saved)
	}
}

func TestPins_MShowsStrip(t *testing.T) {
	m := pinModel(t, pinSet{"1": "root"})
	m.list.Select(1)
	m = sendKey(m, "m")
	out := ansi.Strip(m.viewString())
	for _, want := range []string{"m pin to slot · feature-auth is selected", "1 root", "2 ·", "0 unpin", "esc close"} {
		if !strings.Contains(out, want) {
			t.Errorf("strip missing %q:\n%s", want, out)
		}
	}
	m = sendKey(m, "esc")
	if strings.Contains(ansi.Strip(m.viewString()), "pin to slot") || len(m.pins) != 1 {
		t.Errorf("esc should close the strip and change nothing (pins %v)", m.pins)
	}
}

func TestPins_StripRestoresListHeight(t *testing.T) {
	m := pinModel(t, pinSet{})
	h := m.list.Height()
	m = sendKey(m, "m")
	if m.list.Height() >= h {
		t.Errorf("list height %d with the strip open, want less than %d", m.list.Height(), h)
	}
	m = sendKey(m, "esc")
	if m.list.Height() != h {
		t.Errorf("list height %d after closing, want %d", m.list.Height(), h)
	}
}

func TestPins_MOtherKeyCloses(t *testing.T) {
	m := pinModel(t, pinSet{})
	m = sendKey(m, "m")
	m = sendKey(m, "x")
	if m.pendingM || len(m.pins) != 0 {
		t.Errorf("pendingM %v pins %v, want closed and unchanged", m.pendingM, m.pins)
	}
}

func TestPins_DigitSwitches(t *testing.T) {
	m := pinModel(t, pinSet{"2": "fix-bug", "1": "root"})
	next, cmd := pressKey(m, "2")
	if next.switchTo != "/tmp/test-project-fix-bug" || !isQuit(cmd) {
		t.Errorf("2: switchTo %q quit %v", next.switchTo, isQuit(cmd))
	}
	next, cmd = pressKey(m, "1")
	if next.switchTo != "" || !isQuit(cmd) {
		t.Errorf("1 on the current worktree: switchTo %q quit %v, want quit without a switch", next.switchTo, isQuit(cmd))
	}
}

func TestPins_EmptySlotFlashes(t *testing.T) {
	m := pinModel(t, pinSet{})
	next, cmd := pressKey(m, "4")
	if isQuit(cmd) || next.switchTo != "" {
		t.Error("an empty slot must not switch")
	}
	if got := next.toast.Message(); got != "slot 4 is empty · m 4 to pin" {
		t.Errorf("toast = %q", got)
	}
}

func TestPins_NoPositionalQuickSwitch(t *testing.T) {
	m := pinModel(t, pinSet{})
	if next, cmd := pressKey(m, "2"); next.switchTo != "" || isQuit(cmd) {
		t.Error("2 with no pins switched by row position")
	}
}

func TestPins_DigitWithFilterApplied(t *testing.T) {
	m := pinModel(t, pinSet{"2": "fix-bug"})
	m.list.SetFilterText("test")
	next, cmd := pressKey(m, "2")
	if next.switchTo != "/tmp/test-project-fix-bug" || !isQuit(cmd) {
		t.Errorf("with a filter applied: switchTo %q", next.switchTo)
	}
}

func TestPins_DigitWhileTypingFilterTypes(t *testing.T) {
	m := pinModel(t, pinSet{"2": "fix-bug"})
	m = sendKey(m, "/")
	next, _ := pressKey(m, "2")
	if next.switchTo != "" || next.list.FilterValue() != "2" {
		t.Errorf("typing 2 in the filter: switchTo %q filter %q", next.switchTo, next.list.FilterValue())
	}
}

func TestPins_RenameMovesPin(t *testing.T) {
	m := pinModel(t, pinSet{"2": "feature-auth"})
	m = sendMsg(m, renameCompleteMsg{oldName: "feature-auth", newName: "auth"})
	if m.pins["2"] != "auth" {
		t.Errorf("pins = %v", m.pins)
	}
	if saved := loadUIPrefs(m.projectRoot); saved == nil || saved.Pins["2"] != "auth" {
		t.Errorf("saved = %+v", saved)
	}
}

func TestPins_DeleteDropsPin(t *testing.T) {
	m := pinModel(t, pinSet{"2": "feature-auth", "1": "root"})
	m = sendMsg(m, worktreeDeletedMsg{name: "feature-auth"})
	if _, ok := m.pins["2"]; ok || m.pins["1"] != "root" {
		t.Errorf("pins = %v", m.pins)
	}
}

func TestPins_PrunedOnlyAfterSuccessfulFetch(t *testing.T) {
	m := pinModel(t, pinSet{"5": "gone", "1": "root"})
	failed := sendMsg(m, worktreesFetchedMsg{err: errors.New("boom")})
	if failed.pins["5"] != "gone" {
		t.Error("a failed fetch pruned pins")
	}
	ok := sendMsg(m, worktreesFetchedMsg{items: makeTestItems(2)})
	if _, kept := ok.pins["5"]; kept || ok.pins["1"] != "root" {
		t.Errorf("successful fetch: pins %v, want only root", ok.pins)
	}
}

func TestLastActive_DashSwitches(t *testing.T) {
	m := pinModel(t, pinSet{})
	m.lastActive = "fix-bug"
	next, cmd := pressKey(m, "-")
	if next.switchTo != "/tmp/test-project-fix-bug" || !isQuit(cmd) {
		t.Errorf("-: switchTo %q quit %v", next.switchTo, isQuit(cmd))
	}
}

func TestLastActive_CurrentQuitsWithoutSwitch(t *testing.T) {
	m := pinModel(t, pinSet{})
	m.lastActive = "root"
	next, cmd := pressKey(m, "-")
	if next.switchTo != "" || !isQuit(cmd) {
		t.Errorf("- on the current worktree: switchTo %q quit %v", next.switchTo, isQuit(cmd))
	}
}

func TestLastActive_NoneFlashes(t *testing.T) {
	m := pinModel(t, pinSet{})
	next, cmd := pressKey(m, "-")
	if isQuit(cmd) || next.toast.Message() != "no previous worktree" {
		t.Errorf("toast %q quit %v", next.toast.Message(), isQuit(cmd))
	}
}

// After g, the next key belongs to the chord: pins, -, and m do nothing.
func TestPins_GChordOwnsTheNextKey(t *testing.T) {
	for _, k := range []string{"3", "-", "m"} {
		m := pinModel(t, pinSet{"3": "fix-bug"})
		m.lastActive = "fix-bug"
		m = sendKey(m, "g")
		next, cmd := pressKey(m, k)
		if next.switchTo != "" || isQuit(cmd) || next.pendingM || next.pendingG {
			t.Errorf("g %s: switchTo %q quit %v pendingM %v pendingG %v", k, next.switchTo, isQuit(cmd), next.pendingM, next.pendingG)
		}
	}
}

// m then ? or any other key only closes the strip, and the list gets its
// height back.
func TestPins_StripClosesOnHelpKey(t *testing.T) {
	m := pinModel(t, pinSet{})
	h := m.list.Height()
	m = sendKey(m, "m")
	m = sendKey(m, "?")
	if m.activeView() != ViewDashboard || m.pendingM || m.list.Height() != h {
		t.Errorf("m ?: view %v pendingM %v height %d (want dashboard, closed, %d)", m.activeView(), m.pendingM, m.list.Height(), h)
	}
}

// A long alias never makes the strip wider than the screen.
func TestPins_StripFitsWidth(t *testing.T) {
	m := newTestModel(withItems(3), withSize(60, 24))
	m.projectRoot = prefsRoot(t)
	items := m.list.Items()
	it := items[0].(WorktreeItem)
	it.ShortName = strings.Repeat("very-long-alias-", 4)
	items[0] = it
	m.list.SetItems(items)
	m.pins = pinSet{"1": it.ShortName}
	m = sendKey(m, "m")
	for i, line := range strings.Split(m.viewString(), "\n") {
		if w := lipgloss.Width(line); w > 60 {
			t.Errorf("line %d is %d cells", i, w)
		}
	}
}

// Two dashboards on one project keep each other's pins.
func TestPins_SaveKeepsAnotherInstancesPins(t *testing.T) {
	root := prefsRoot(t)
	a := newTestModel(withItems(4), withSize(120, 40))
	a.projectRoot, a.pins = root, pinSet{}
	b := newTestModel(withItems(4), withSize(120, 40))
	b.projectRoot, b.pins = root, pinSet{}
	a.list.Select(0)
	a = sendKey(a, "m")
	a = sendKey(a, "1")
	b.list.Select(2)
	b = sendKey(b, "m")
	b = sendKey(b, "2")
	saved := loadUIPrefs(root)
	if saved == nil || saved.Pins["1"] != "root" || saved.Pins["2"] != "fix-bug" {
		t.Errorf("saved pins = %+v, want both instances' pins", saved)
	}
	if b.pins["1"] != "root" {
		t.Errorf("b.pins = %v, want it to pick up slot 1 from disk", b.pins)
	}
}

func TestPins_FailedRenameOrDeleteKeepsPin(t *testing.T) {
	m := pinModel(t, pinSet{"2": "feature-auth"})
	r := sendMsg(m, renameCompleteMsg{oldName: "feature-auth", newName: "auth", err: errors.New("boom")})
	d := sendMsg(m, worktreeDeletedMsg{name: "feature-auth", err: errors.New("boom")})
	if r.pins["2"] != "feature-auth" || d.pins["2"] != "feature-auth" {
		t.Errorf("rename %v delete %v, want the pin kept after failures", r.pins, d.pins)
	}
	bd := sendMsg(m, worktreeDeletedMsg{name: "feature-auth", branchErr: errors.New("branch")})
	if _, ok := bd.pins["2"]; ok {
		t.Error("a delete that removed the worktree but not its branch kept the pin")
	}
}

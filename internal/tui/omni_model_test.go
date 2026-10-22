package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/lost-in-the/grove/plugins/tracker"
)

func typeOmni(m Model, text string) Model {
	for _, r := range text {
		m = sendKey(m, string(r))
	}
	return m
}

func TestOmni_SpaceOpens(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m.list.Select(1)
	next, cmd := m.Update(makeKeyMsg("space"))
	m = next.(Model)
	if m.activeView() != ViewOmni || m.omni == nil || m.omni.Scope != omniScopeAll {
		t.Fatalf("space: view %v omni %+v", m.activeView(), m.omni)
	}
	if m.omni.Target != "/tmp/test-project-feature-auth" {
		t.Errorf("target = %q", m.omni.Target)
	}
	// Opening omni never fetches: no browser state, no loading.
	_ = cmd
	if m.prState != nil || m.issueState != nil || m.loading {
		t.Error("opening omni started a fetch")
	}
}

func TestOmni_TypeFiltersAndEnterRunsAction(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m.list.Select(1)
	m = sendKey(m, "space")
	m = typeOmni(m, "rename")
	m = sendKey(m, "enter")
	if m.activeView() != ViewRename || m.renameState == nil || m.renameState.Item.ShortName != "feature-auth" {
		t.Errorf("view %v rename %+v, want rename for feature-auth", m.activeView(), m.renameState)
	}
}

// The action runs on the worktree selected when omni opened, even if the
// list cursor moved underneath.
func TestOmni_RunsOnTargetAfterPop(t *testing.T) {
	m := newTestModel(withItems(4), withSize(120, 40))
	m.list.Select(2)
	m = sendKey(m, "space")
	m.list.Select(3)
	m = typeOmni(m, "rename")
	m = sendKey(m, "enter")
	if m.renameState == nil || m.renameState.Item.ShortName != "fix-bug" {
		t.Errorf("rename %+v, want fix-bug", m.renameState)
	}

	gone := newTestModel(withItems(4), withSize(120, 40))
	gone.list.Select(2)
	gone = sendKey(gone, "space")
	gone.omni.Target = "/tmp/vanished"
	gone = typeOmni(gone, "rename")
	gone = sendKey(gone, "enter")
	if gone.activeView() != ViewDashboard || gone.toast.Message() == "" {
		t.Errorf("target gone: view %v toast %q, want dashboard and a flash", gone.activeView(), gone.toast.Message())
	}
}

func TestOmni_EnterOnWorktreeSwitches(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m = sendKey(m, "space")
	m = typeOmni(m, "fix-bug")
	next, cmd := pressKey(m, "enter")
	if next.switchTo != "/tmp/test-project-fix-bug" || !isQuit(cmd) {
		t.Errorf("switchTo %q quit %v", next.switchTo, isQuit(cmd))
	}
}

func TestOmni_EnterOnIssueOpensBrowserAtIt(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	issues := []*tracker.Issue{{Number: 3, Title: "Old"}, {Number: 9, Title: "Login page crashes"}}
	m = sendMsg(m, issuesFetchedMsg{issues: issues})
	m = sendKey(m, "space")
	m = typeOmni(m, "login page")
	m = sendKey(m, "enter")
	m = sendMsg(m, issuesFetchedMsg{issues: issues})
	if m.activeView() != ViewIssues || m.issueState == nil || m.issueState.Cursor != 1 {
		t.Errorf("view %v, want the issue browser at #9", m.activeView())
	}
}

func TestOmni_CtrlJKMovesAndEscCloses(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m = sendKey(m, "space")
	m = typeOmni(m, "e")
	if m.omni.Cursor != 0 {
		t.Fatalf("cursor %d after typing", m.omni.Cursor)
	}
	m = sendKey(m, "ctrl+j")
	m = sendKey(m, "ctrl+j")
	m = sendKey(m, "ctrl+k")
	if m.omni.Cursor != 1 {
		t.Errorf("cursor %d after j j k, want 1", m.omni.Cursor)
	}
	m = sendKey(m, "esc")
	if m.activeView() != ViewDashboard || m.omni != nil {
		t.Errorf("esc: view %v omni %v", m.activeView(), m.omni)
	}
}

func TestOmni_QuestionMarkScopesToActions(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m = sendKey(m, "?")
	if m.activeView() != ViewOmni || m.omni.Scope != omniScopeActions {
		t.Fatalf("? on the dashboard: view %v", m.activeView())
	}
	p := newTestModel(withItems(3), withSize(120, 40), withPRData())
	p = sendKey(p, "?")
	if p.activeView() != ViewHelp {
		t.Errorf("? in the PR browser: view %v, want help", p.activeView())
	}
}

// The old help content stays reachable as an action.
func TestOmni_KeyboardShortcutsOpensHelp(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m = sendKey(m, "?")
	m = typeOmni(m, "keyboard")
	m = sendKey(m, "enter")
	if m.activeView() != ViewHelp {
		t.Errorf("keyboard shortcuts: view %v, want help", m.activeView())
	}
}

// Typing never changes the frame: its size depends only on the terminal.
func TestOmni_FrameSizeIsFixed(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}, {60, 24}, {80, 12}} {
		m := newTestModel(withItems(8), withSize(size[0], size[1]))
		m = sendKey(m, "space")
		var w0, h0 int
		for i, q := range []string{"", "e", "zzzz", "re"} {
			if i > 0 {
				m.omni.Input.SetValue(q)
				m.omni.refresh()
			}
			out := renderOmni(m.omni, size[0], size[1])
			w, h := lipgloss.Width(out), lipgloss.Height(out)
			if i == 0 {
				w0, h0 = w, h
				continue
			}
			if w != w0 || h != h0 {
				t.Errorf("%dx%d query %q: frame %dx%d, want %dx%d", size[0], size[1], q, w, h, w0, h0)
			}
		}
		if h0 > size[1]-formTop {
			t.Errorf("%dx%d: frame height %d runs off the screen", size[0], size[1], h0)
		}
	}
}

// Matched letters are drawn differently from the rest of the row.
func TestOmni_HighlightsMatches(t *testing.T) {
	if Colors.Primary == nil {
		t.Skip("no colors")
	}
	plain := litRunes("rename worktree", nil, Styles.NormalItem)
	lit := litRunes("rename worktree", []int{0, 1}, Styles.NormalItem)
	if plain == lit {
		t.Error("highlighted runes render the same as plain text")
	}
	if ansi.Strip(lit) != "rename worktree" {
		t.Errorf("highlighting changed the text: %q", ansi.Strip(lit))
	}
}

// Actions that need no worktree run even with nothing selected.
func TestOmni_ActionWithoutSelection(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m.list.SetFilterText("zzzz-nothing")
	m = sendKey(m, "?")
	m = typeOmni(m, "config")
	m = sendKey(m, "enter")
	if m.activeView() != ViewConfig {
		t.Errorf("config with no selection: view %v toast %q", m.activeView(), m.toast.Message())
	}
}

// Every printable key types into the search; none acts.
func TestOmni_KeysTypeIntoInput(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m = sendKey(m, "space")
	for _, k := range []string{"j", "k", "q", "?", "g", "1", "-", "/", "u", "m"} {
		m = sendKey(m, k)
	}
	if got := m.omni.Input.Value(); got != "jkq?g1-/um" || m.activeView() != ViewOmni {
		t.Errorf("input %q view %v", got, m.activeView())
	}
}

func TestOmni_ResizeKeepsInputInFrame(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m = sendKey(m, "space")
	m = resize(m, 60, 24)
	w, _ := omniFrameSize(60, 24)
	if got := m.omni.Input.Width(); got > w-8 {
		t.Errorf("input width %d in a %d-wide frame", got, w)
	}
}

func TestOmni_RowsAlignAcrossGroups(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m = sendMsg(m, prsFetchedMsg{prs: []*tracker.PullRequest{{Number: 1, Title: "Fix the thing"}}})
	m = sendMsg(m, issuesFetchedMsg{issues: []*tracker.Issue{{Number: 2, Title: "Fix login"}}})
	m = sendKey(m, "space")
	m = typeOmni(m, "fix")
	col := -1
	for _, line := range strings.Split(ansi.Strip(renderOmni(m.omni, 120, 40)), "\n") {
		if strings.Contains(line, "⏎") {
			continue // the status line repeats a label
		}
		for _, label := range []string{"Fix the thing", "Fix login", "fix-bug"} {
			if i := strings.Index(line, label); i >= 0 {
				c := lipgloss.Width(line[:i])
				if col >= 0 && c != col {
					t.Errorf("%q starts at column %d, others at %d", label, c, col)
				}
				col = c
			}
		}
	}
}

func TestOmni_StatusOnCurrentWorktree(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m = sendKey(m, "space")
	m = typeOmni(m, "root")
	for i, r := range m.omni.rows() {
		if r.Item.Kind == omniWorktree && r.Item.Label == "root" {
			m.omni.Cursor = i
		}
	}
	if st, _ := omniStatus(m.omni); st != "⏎ stays on root" {
		t.Errorf("status = %q", st)
	}
}

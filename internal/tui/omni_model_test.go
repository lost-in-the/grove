package tui

import (
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
	// Opening omni never fetches: at most the input's cursor blink.
	if cmd != nil {
		if _, isFetch := cmd().(prsFetchedMsg); isFetch {
			t.Error("opening omni fetched PRs")
		}
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

func TestOmni_EnterOnPROpensBrowserAtIt(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	prs := []*tracker.PullRequest{{Number: 7, Title: "First"}, {Number: 42, Title: "Answer the question"}}
	m = sendMsg(m, prsFetchedMsg{prs: prs})
	m = sendKey(m, "space")
	m = typeOmni(m, "#42")
	m = sendKey(m, "enter")
	if m.activeView() != ViewPRs || m.prState == nil {
		t.Fatalf("view %v, want the PR browser", m.activeView())
	}
	m = sendMsg(m, prsFetchedMsg{prs: prs})
	if m.prState.Cursor != 1 {
		t.Errorf("cursor %d after the fetch, want 1 (#42)", m.prState.Cursor)
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

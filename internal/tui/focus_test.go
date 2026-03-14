package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// --- Dashboard focus tests ---

func TestDashboard_TabEntersFocus(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m = sendKey(m, "tab")
	if !m.detailFocused {
		t.Error("tab should set detailFocused = true")
	}
}

func TestDashboard_FocusEscReturns(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m = sendKey(m, "tab")
	m = sendKey(m, "esc")
	if m.detailFocused {
		t.Error("esc should set detailFocused = false")
	}
}

func TestDashboard_FocusTabReturns(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m = sendKey(m, "tab")
	m = sendKey(m, "tab")
	if m.detailFocused {
		t.Error("second tab should set detailFocused = false")
	}
}

func TestDashboard_FocusScrollSwallowsKeys(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	cursorBefore := m.list.Index()
	m = sendKey(m, "tab")
	m = sendKey(m, "j")
	if m.list.Index() != cursorBefore {
		t.Errorf("j while focused should not move list cursor: got %d, want %d", m.list.Index(), cursorBefore)
	}
}

func TestDashboard_FocusQStillQuits(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m = sendKey(m, "tab")
	result, cmd := m.Update(makeKeyMsg("q"))
	m = result.(Model)
	if cmd == nil {
		t.Error("q while detail focused should return a non-nil cmd (quit)")
	}
}

func TestDashboard_FocusBNoPR(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m = sendKey(m, "tab")
	// Item has no AssociatedPR, pressing B should not panic
	m = sendKey(m, "B")
	if !m.detailFocused {
		t.Error("B with no PR should not change focus state")
	}
}

func TestDashboard_FocusedHintsWithPR(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	// Set AssociatedPR on the selected item
	item, ok := m.selectedItem()
	if !ok {
		t.Fatal("expected a selected item")
	}
	item.AssociatedPR = &PRInfo{Number: 42, URL: "https://github.com/test/test/pull/42"}
	// Update the item in the list
	items := m.list.Items()
	items[m.list.Index()] = item
	m.list.SetItems(items)

	hints := m.dashboardDetailFocusedHints()
	found := false
	for _, h := range hints {
		if h.Key == "B" {
			found = true
			break
		}
	}
	if !found {
		t.Error("hints should include B when item has AssociatedPR")
	}
}

func TestDashboard_FocusedHintsWithoutPR(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	hints := m.dashboardDetailFocusedHints()
	for _, h := range hints {
		if h.Key == "B" {
			t.Error("hints should NOT include B when item has no AssociatedPR")
		}
	}
}

// --- PR view focus tests ---

func TestPR_TabEntersFocus(t *testing.T) {
	m := newTestModel(withPRData(), withSize(120, 40))
	m.activeView = ViewPRs
	m = sendKey(m, "tab")
	if !m.prState.DetailFocused {
		t.Error("tab should set prState.DetailFocused = true")
	}
}

func TestPR_FocusEscUnfocuses(t *testing.T) {
	m := newTestModel(withPRData(), withSize(120, 40))
	m.activeView = ViewPRs
	m = sendKey(m, "tab")
	m = sendKey(m, "esc")
	if m.prState.DetailFocused {
		t.Error("esc should set DetailFocused = false")
	}
	if m.activeView != ViewPRs {
		t.Errorf("esc from detail focus should stay in PR view, got activeView=%d", m.activeView)
	}
}

func TestPR_FocusTabUnfocuses(t *testing.T) {
	m := newTestModel(withPRData(), withSize(120, 40))
	m.activeView = ViewPRs
	m = sendKey(m, "tab")
	m = sendKey(m, "tab")
	if m.prState.DetailFocused {
		t.Error("second tab should set DetailFocused = false")
	}
}

func TestPR_FocusScrollSwallowsKeys(t *testing.T) {
	m := newTestModel(withPRData(), withSize(120, 40))
	m.activeView = ViewPRs
	cursorBefore := m.prState.Cursor
	m = sendKey(m, "tab")
	m = sendKey(m, "j")
	if m.prState.Cursor != cursorBefore {
		t.Errorf("j while PR detail focused should not move cursor: got %d, want %d", m.prState.Cursor, cursorBefore)
	}
}

func TestPR_FocusVimKeys(t *testing.T) {
	m := newTestModel(withPRData(), withSize(120, 40))
	m.activeView = ViewPRs
	m = sendKey(m, "tab")
	// These should not panic
	m = sendKey(m, "g")
	m = sendKey(m, "G")
	if !m.prState.DetailFocused {
		t.Error("vim keys should not change focus state")
	}
}

// --- Issue view focus tests ---

func TestIssue_TabEntersFocus(t *testing.T) {
	m := newTestModel(withIssueData(), withSize(120, 40))
	m.activeView = ViewIssues
	m = sendKey(m, "tab")
	if !m.issueState.DetailFocused {
		t.Error("tab should set issueState.DetailFocused = true")
	}
}

func TestIssue_FocusEscUnfocuses(t *testing.T) {
	m := newTestModel(withIssueData(), withSize(120, 40))
	m.activeView = ViewIssues
	m = sendKey(m, "tab")
	m = sendKey(m, "esc")
	if m.issueState.DetailFocused {
		t.Error("esc should set DetailFocused = false")
	}
	if m.activeView != ViewIssues {
		t.Errorf("esc from detail focus should stay in issue view, got activeView=%d", m.activeView)
	}
}

func TestIssue_FocusScrollSwallowsKeys(t *testing.T) {
	m := newTestModel(withIssueData(), withSize(120, 40))
	m.activeView = ViewIssues
	cursorBefore := m.issueState.Cursor
	m = sendKey(m, "tab")
	m = sendKey(m, "j")
	if m.issueState.Cursor != cursorBefore {
		t.Errorf("j while issue detail focused should not move cursor: got %d, want %d", m.issueState.Cursor, cursorBefore)
	}
}

// Verify tea.Quit is returned from the quit command by checking the message type.
func isQuitCmd(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	msg := cmd()
	_, ok := msg.(tea.QuitMsg)
	return ok
}

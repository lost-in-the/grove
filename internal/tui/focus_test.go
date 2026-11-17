package tui

import (
	"strings"
	"testing"
)

func TestListCursorFollowsFocus(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []testOpt
	}{
		{"compact", []testOpt{withItems(5), withCompact()}},
		{"detailed", []testOpt{withItems(5), withDetailed()}},
		{"prs", []testOpt{withItems(3), withPRData()}},
		{"issues", []testOpt{withItems(3), withIssueData()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestModel(append(tc.opts, withSize(120, 40))...)
			check := func(want int) {
				t.Helper()
				if got := strings.Count(stripAnsi(m.viewString()), "❯"); got != want {
					t.Fatalf("got %d cursors, want %d:\n%s", got, want, stripAnsi(m.viewString()))
				}
			}
			check(1)
			m = sendKey(m, "tab")
			check(0)
			m = sendKey(m, "esc")
			check(1)
			if tc.name == "prs" || tc.name == "issues" {
				prompt := &ExistingWorktreePrompt{Branch: "main", WorktreeName: "root"}
				if m.prState != nil {
					m.prState.ExistsPrompt = prompt
				} else {
					m.issueState.ExistsPrompt = prompt
				}
				check(0)
				m = sendKey(m, "esc")
				check(1)
			}
			if tc.name == "compact" || tc.name == "detailed" {
				m = sendKey(m, "space")
				check(1) // only the go-to result owns the cursor
				m = sendKey(m, "esc")
				check(1)
			}
		})
	}
}

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
	_, cmd := m.Update(makeKeyMsg("q"))
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

	hints := m.dashboardDetailFooter().Hints
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
	hints := m.dashboardDetailFooter().Hints
	for _, h := range hints {
		if h.Key == "B" {
			t.Error("hints should NOT include B when item has no AssociatedPR")
		}
	}
}

// --- PR view focus tests ---

func TestPR_TabEntersFocus(t *testing.T) {
	m := newTestModel(withPRData(), withSize(120, 40))
	m.setViewForTest(ViewPRs)
	m = sendKey(m, "tab")
	if !m.prState.DetailFocused {
		t.Error("tab should set prState.DetailFocused = true")
	}
}

func TestPR_FocusEscUnfocuses(t *testing.T) {
	m := newTestModel(withPRData(), withSize(120, 40))
	m.setViewForTest(ViewPRs)
	m = sendKey(m, "tab")
	m = sendKey(m, "esc")
	if m.prState.DetailFocused {
		t.Error("esc should set DetailFocused = false")
	}
	if m.activeView() != ViewPRs {
		t.Errorf("esc from detail focus should stay in PR view, got activeView=%d", m.activeView())
	}
}

func TestPR_FocusTabUnfocuses(t *testing.T) {
	m := newTestModel(withPRData(), withSize(120, 40))
	m.setViewForTest(ViewPRs)
	m = sendKey(m, "tab")
	m = sendKey(m, "tab")
	if m.prState.DetailFocused {
		t.Error("second tab should set DetailFocused = false")
	}
}

func TestPR_FocusScrollSwallowsKeys(t *testing.T) {
	m := newTestModel(withPRData(), withSize(120, 40))
	m.setViewForTest(ViewPRs)
	cursorBefore := m.prState.Cursor
	m = sendKey(m, "tab")
	m = sendKey(m, "j")
	if m.prState.Cursor != cursorBefore {
		t.Errorf("j while PR detail focused should not move cursor: got %d, want %d", m.prState.Cursor, cursorBefore)
	}
}

func TestPR_FocusVimKeys(t *testing.T) {
	m := newTestModel(withPRData(), withSize(120, 40))
	m.setViewForTest(ViewPRs)
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
	m.setViewForTest(ViewIssues)
	m = sendKey(m, "tab")
	if !m.issueState.DetailFocused {
		t.Error("tab should set issueState.DetailFocused = true")
	}
}

func TestIssue_FocusEscUnfocuses(t *testing.T) {
	m := newTestModel(withIssueData(), withSize(120, 40))
	m.setViewForTest(ViewIssues)
	m = sendKey(m, "tab")
	m = sendKey(m, "esc")
	if m.issueState.DetailFocused {
		t.Error("esc should set DetailFocused = false")
	}
	if m.activeView() != ViewIssues {
		t.Errorf("esc from detail focus should stay in issue view, got activeView=%d", m.activeView())
	}
}

func TestIssue_FocusScrollSwallowsKeys(t *testing.T) {
	m := newTestModel(withIssueData(), withSize(120, 40))
	m.setViewForTest(ViewIssues)
	cursorBefore := m.issueState.Cursor
	m = sendKey(m, "tab")
	m = sendKey(m, "j")
	if m.issueState.Cursor != cursorBefore {
		t.Errorf("j while issue detail focused should not move cursor: got %d, want %d", m.issueState.Cursor, cursorBefore)
	}
}

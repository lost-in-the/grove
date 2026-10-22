package tui

import (
	"strings"
	"testing"

	"github.com/lost-in-the/grove/plugins/tracker"
)

var reviewPRs = []*tracker.PullRequest{
	{Number: 7, Title: "First change", Branch: "first"},
	{Number: 42, Title: "Answer the question", Branch: "answer"},
}

// openBrowser opens the PR browser on reviewPRs with the cursor on index.
func openBrowser(t *testing.T, index int) Model {
	t.Helper()
	m := newTestModel(withItems(3), withSize(120, 40))
	next, _ := m.enterPRView()
	m = next.(Model)
	m = sendMsg(m, prsFetchedMsg{prs: reviewPRs})
	m.prState.Cursor = index
	return m
}

func TestPRBrowser_EnterOpensReview(t *testing.T) {
	m := sendKey(openBrowser(t, 1), "enter")
	if m.activeView() != ViewPRReview || m.prReview == nil || m.prReview.PR.Number != 42 {
		t.Fatalf("view %v, review %+v; want the review of #42", m.activeView(), m.prReview)
	}
	header := strings.Split(stripAnsi(m.viewString()), "\n")[0]
	if !strings.Contains(header, "#42 Answer the question") {
		t.Errorf("header %q", header)
	}
}

// n is what enter used to be: a worktree from the PR's branch, or the
// existing-worktree prompt when the branch already has one.
func TestPRBrowser_NCreatesWorktree(t *testing.T) {
	m := sendKey(openBrowser(t, 1), "n")
	if m.activeView() != ViewBranchForm || m.branchForm == nil {
		t.Fatalf("n: view %v, want the branch form", m.activeView())
	}

	m = openBrowser(t, 1)
	m.prState.WorktreeBranches = map[string]string{"answer": "answer-wt"}
	m = sendKey(m, "n")
	if p := m.prState.ExistsPrompt; p == nil || p.WorktreeName != "answer-wt" {
		t.Errorf("n on a branch with a worktree: prompt %+v", p)
	}
}

func TestOmni_PRResultOpensReview(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m = sendMsg(m, prsFetchedMsg{prs: reviewPRs})
	m = sendKey(m, "space")
	m = typeOmni(m, "#42")
	m = sendKey(m, "enter")
	if m.activeView() != ViewPRReview || m.prReview == nil || m.prReview.PR.Number != 42 {
		t.Fatalf("view %v; want the review of #42", m.activeView())
	}
	if m = sendKey(m, "esc"); m.activeView() != ViewDashboard {
		t.Errorf("esc from a review opened by omni: view %v, want the dashboard", m.activeView())
	}
}

// Spec 18: PR browser → review → back pops one level each, and the
// browser keeps its cursor.
func TestPRReview_EscPopsToBrowser(t *testing.T) {
	m := sendKey(openBrowser(t, 1), "enter")
	m = sendKey(m, "esc")
	if m.activeView() != ViewPRs || m.prState.Cursor != 1 || m.prReview != nil {
		t.Fatalf("after esc: view %v, cursor %d, review %v", m.activeView(), m.prState.Cursor, m.prReview)
	}
	if m = sendKey(m, "esc"); m.activeView() != ViewDashboard {
		t.Errorf("second esc: view %v, want the dashboard", m.activeView())
	}
}

// The frame opens on the conversation; 1-4 raise a tab, and counts read
// "-" until their data arrives.
func TestPRReview_TabKeysAndCounts(t *testing.T) {
	m := sendKey(openBrowser(t, 0), "enter")
	if m.prReview.Tab != 1 {
		t.Errorf("opens on tab %d, want 1 (conversation)", m.prReview.Tab)
	}
	tabs := strings.Split(stripAnsi(m.viewString()), "\n")[1]
	for _, want := range []string{"[1] conversation -", "[2] commits -", "[3] checks -", "[4] files -"} {
		if !strings.Contains(tabs, want) {
			t.Errorf("tab row %q lacks %q", tabs, want)
		}
	}
	for _, k := range []string{"1", "2", "3", "4"} {
		if m = sendKey(m, k); m.prReview.Tab != int(k[0]-'0') {
			t.Errorf("%s: tab %d", k, m.prReview.Tab)
		}
	}
}

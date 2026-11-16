package tui

import (
	"strings"
	"testing"

	"github.com/lost-in-the/grove/plugins/tracker"
)

func TestCheckBucket(t *testing.T) {
	for state, want := range map[string]int{
		"SUCCESS": checkPassed, "NEUTRAL": checkPassed, "SKIPPED": checkPassed,
		"FAILURE": checkFailed, "ERROR": checkFailed, "TIMED_OUT": checkFailed, "ACTION_REQUIRED": checkFailed,
		"CANCELLED": checkFailed, "STARTUP_FAILURE": checkFailed, //nolint:misspell // GitHub's spelling
		"IN_PROGRESS": checkRunning, "QUEUED": checkRunning, "PENDING": checkRunning, "": checkRunning,
	} {
		if got := checkBucket(state); got != want {
			t.Errorf("checkBucket(%q) = %d, want %d", state, got, want)
		}
	}
}

// Failing first, then running, then passing; two runs with one name both
// show (a push and a pull_request run of one workflow).
func TestRenderChecks_OrderAndGlyphs(t *testing.T) {
	checks := []tracker.PRCheck{
		{Name: "Build", State: "SUCCESS"},
		{Name: "pii-check", State: "SUCCESS"},
		{Name: "Test", State: "IN_PROGRESS"},
		{Name: "Lint", State: "FAILURE"},
		{Name: "pii-check", State: "SUCCESS"},
	}
	var got []string
	for _, it := range checkItems(checks) {
		got = append(got, strings.TrimRight(stripAnsi(it[0]), " "))
	}
	want := []string{"✗ Lint  FAILURE", "● Test  IN_PROGRESS", "✓ Build  SUCCESS", "✓ pii-check  SUCCESS", "✓ pii-check  SUCCESS"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("checks:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestPRReview_ChecksEmpty(t *testing.T) {
	m := openReview(t)
	m = sendMsg(m, prDetailMsg{gen: m.prReview.Gen, detail: &tracker.PRDetail{Number: 42}})
	if text := stripAnsi(sendKey(m, "3").viewString()); !strings.Contains(text, "no checks") {
		t.Errorf("empty checks tab:\n%s", text)
	}
}

func TestPRReview_OpensOnConversation(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	next, _ := m.openPRReview(&tracker.PullRequest{Number: 42})
	if tab := next.(Model).prReview.Tab; tab != 1 {
		t.Errorf("opens on tab %d, want 1", tab)
	}
}

// j and k move the checks tab's cursor, clamped at both ends.
func TestPRReview_ChecksCursor(t *testing.T) {
	m := openReview(t)
	m = sendMsg(m, prDetailMsg{gen: m.prReview.Gen, detail: &tracker.PRDetail{Number: 42,
		Checks: []tracker.PRCheck{{Name: "a", State: "SUCCESS"}, {Name: "b", State: "SUCCESS"}}}})
	m = sendKey(m, "3")
	m = sendKey(sendKey(sendKey(m, "j"), "j"), "j")
	if c := m.prReview.Cursor[3]; c != 1 {
		t.Errorf("cursor %d after three j over two checks, want 1", c)
	}
	if m = sendKey(sendKey(m, "k"), "k"); m.prReview.Cursor[3] != 0 {
		t.Errorf("cursor %d after k k", m.prReview.Cursor[3])
	}
}

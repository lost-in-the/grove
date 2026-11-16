package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/git"
	"github.com/lost-in-the/grove/plugins/tracker"
)

// A diff is the PR author's text: no control character in it may reach
// the terminal (plan 8b decision 8).
func TestSafeText(t *testing.T) {
	cases := map[string]string{
		"plain":            "plain",
		"esc\x1b[2J":       "esc␛[2J",
		"bell\a":           "bell␇",
		"del\x7f":          "del␡",
		"a\tb":             "a   b",
		"abcd\te":          "abcd    e",
		"crlf\r":           "crlf",
		"café ✓":           "café ✓",
		"csi\u009b2J":      "csi\\x9b2J",
		"nul\x00":          "nul␀",
		"\x1b]52;c;aGk=\a": "␛]52;c;aGk=␇",
	}
	for in, want := range cases {
		if got := safeText(in); got != want {
			t.Errorf("safeText(%q) = %q, want %q", in, got, want)
		}
	}
}

var sampleFile = git.FileDiff{OldPath: "main.go", NewPath: "main.go", Hunks: []git.Hunk{{
	Header: "@@ -9,3 +9,4 @@ func main() {", OldStart: 9, OldLines: 3, NewStart: 9, NewLines: 4,
	Lines: []git.DiffLine{
		{Kind: git.LineContext, Text: "a", OldNo: 9, NewNo: 9},
		{Kind: git.LineRemoved, Text: "b", OldNo: 10},
		{Kind: git.LineAdded, Text: "c", NewNo: 10},
		{Kind: git.LineAdded, Text: "d", NewNo: 11},
		{Kind: git.LineContext, Text: "e", OldNo: 11, NewNo: 12},
	},
}}}

func TestRenderFileDiff_NumbersAndKinds(t *testing.T) {
	got := renderFileDiff(sampleFile, 60, false)
	want := []string{
		"@@ -9,3 +9,4 @@ func main() {",
		" 9  9 │ a",
		"10    │-b",
		"   10 │+c",
		"   11 │+d",
		"11 12 │ e",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d lines:\n%s", len(got), strings.Join(got, "\n"))
	}
	for i := range want {
		if s := strings.TrimRight(stripAnsi(got[i]), " "); s != want[i] {
			t.Errorf("line %d = %q, want %q", i, s, want[i])
		}
	}
}

func TestRenderFileDiff_Binary(t *testing.T) {
	if got := renderFileDiff(git.FileDiff{NewPath: "x.png", Binary: true}, 60, false); len(got) != 1 || stripAnsi(got[0]) != "binary file" {
		t.Errorf("%q", got)
	}
}

func TestRenderFileDiff_PureRename(t *testing.T) {
	f := git.FileDiff{OldPath: "a.go", NewPath: "b.go", Status: git.FileRenamed}
	if got := renderFileDiff(f, 60, false); len(got) != 1 || stripAnsi(got[0]) != "renamed from a.go" {
		t.Errorf("%q", got)
	}
}

// Folding collapses a run of 4 or more removed lines; shorter runs stay.
func TestRenderFileDiff_Fold(t *testing.T) {
	var lines []git.DiffLine
	for i := 1; i <= 5; i++ {
		lines = append(lines, git.DiffLine{Kind: git.LineRemoved, Text: "x", OldNo: i})
	}
	lines = append(lines, git.DiffLine{Kind: git.LineContext, Text: "k", OldNo: 6, NewNo: 1})
	for i := 7; i <= 9; i++ {
		lines = append(lines, git.DiffLine{Kind: git.LineRemoved, Text: "y", OldNo: i})
	}
	f := git.FileDiff{NewPath: "f", Hunks: []git.Hunk{{Header: "@@ -1,9 +1,1 @@", Lines: lines}}}
	got := stripAnsi(strings.Join(renderFileDiff(f, 60, true), "\n"))
	if !strings.Contains(got, "… 5 removed lines") || strings.Count(got, "│-y") != 3 || strings.Contains(got, "│-x") {
		t.Errorf("folded:\n%s", got)
	}
}

// Text the PR author controls never reaches the terminal raw, wherever the
// frame draws it.
func TestPRReview_ViewHasNoRawEscape(t *testing.T) {
	m := openReview(t)
	m = sendMsg(m, prDiffMsg{gen: m.prReview.Gen, files: []git.FileDiff{{
		NewPath: "evil\x1b[2J.go",
		Hunks: []git.Hunk{{Header: "@@ -1 +1 @@ \x1b[2J", Lines: []git.DiffLine{
			{Kind: git.LineAdded, Text: "x\x1b]52;c;aGk=\a\u009b2J", NewNo: 1},
		}}},
	}}})
	view := m.viewString()
	for _, bad := range []string{"\x1b[2J", "\x1b]52;", "\u009b"} {
		if strings.Contains(view, bad) {
			t.Errorf("view holds %q", bad)
		}
	}
	if !strings.Contains(stripAnsi(view), "␛]52;") {
		t.Error("the neutralized text should still show")
	}
}

var hostile = "a \x1b[2J \x1b]52;c;aGk=\a \u009b2J"

func assertNoRawEscape(t *testing.T, label, view string) {
	t.Helper()
	for _, bad := range []string{"\x1b[2J", "\x1b]52;", "\u009b", "\a"} {
		if strings.Contains(view, bad) {
			t.Errorf("%s holds %q", label, bad)
		}
	}
}

// The PR's title and gh's error text reach the review's frame safely too.
func TestPRReview_TitleAndErrorsSafe(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	next, _ := m.openPRReview(&tracker.PullRequest{Number: 42, Title: hostile})
	m = next.(Model)
	m = sendMsg(m, prReviewRepoMsg{gen: m.prReview.Gen, repo: "o/r"})
	m = sendMsg(m, prDiffMsg{gen: m.prReview.Gen, err: errors.New(hostile)})
	assertNoRawEscape(t, "review", sendKey(m, "4").viewString())
}

// Titles, labels, and bodies from gh are made safe as they enter the TUI,
// so every list, detail panel, and Find result draws them safely.
func TestPRAndIssueText_Safe(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	next, _ := m.enterPRView()
	m = next.(Model)
	m = sendMsg(m, prsFetchedMsg{prs: []*tracker.PullRequest{{Number: 1, Title: hostile, Body: "body " + hostile, Labels: []string{hostile}}}})
	assertNoRawEscape(t, "PR browser", m.viewString())
	m = sendKey(m, "tab") // the detail panel renders the body
	assertNoRawEscape(t, "PR detail", m.viewString())

	m = newTestModel(withItems(3), withSize(120, 40))
	m = sendMsg(m, prCacheLoadedMsg{prs: []*tracker.PullRequest{{Number: 1, Title: hostile}}})
	if strings.Contains(m.prCache[0].Title, "\x1b") {
		t.Error("a cached title kept its escape")
	}

	m = newTestModel(withItems(3), withSize(120, 40))
	next, _ = m.enterIssueView()
	m = next.(Model)
	m = sendMsg(m, issuesFetchedMsg{issues: []*tracker.Issue{{Number: 2, Title: hostile, Body: hostile, Labels: []string{hostile}}}})
	assertNoRawEscape(t, "issue browser", m.viewString())
}

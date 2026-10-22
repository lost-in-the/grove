package tui

import (
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/git"
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

package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/exp/golden"

	"github.com/lost-in-the/grove/internal/git"
	"github.com/lost-in-the/grove/plugins/tracker"
)

func diffFile(path string, added, removed int) git.FileDiff {
	var lines []git.DiffLine
	for i := 1; i <= added; i++ {
		lines = append(lines, git.DiffLine{Kind: git.LineAdded, Text: fmt.Sprintf("line %d", i), NewNo: i})
	}
	for i := 1; i <= removed; i++ {
		lines = append(lines, git.DiffLine{Kind: git.LineRemoved, Text: "gone", OldNo: i})
	}
	return git.FileDiff{OldPath: path, NewPath: path, Added: added, Removed: removed,
		Hunks: []git.Hunk{{Header: "@@ -1 +1 @@", OldStart: 1, OldLines: removed, NewStart: 1, NewLines: added, Lines: lines}}}
}

var railFiles = []git.FileDiff{
	diffFile("a.go", 1, 0),
	diffFile("cmd/grove/main.go", 2, 1),
	diffFile("internal/tui/model.go", 3, 2),
	diffFile("internal/tui/view.go", 1, 1),
}

// Folders sum their files; a folder with one child folder joins it; the
// current file is marked and viewed files carry ✓. Folders come before
// files at each level.
func TestRenderRail_TreeAndRollups(t *testing.T) {
	order, lines := railLines(railFiles, map[string]bool{"a.go": true})
	got := make([]string, len(lines))
	for i, l := range lines {
		got[i] = strings.TrimRight(stripAnsi(l.text), " ")
	}
	want := []string{
		"cmd/grove/ +2 -1",
		"  main.go +2 -1",
		"internal/tui/ +4 -3",
		"  model.go +3 -2",
		"  view.go +1 -1",
		"✓ a.go +1 -0",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("rail:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if fmt.Sprint(order) != "[1 2 3 0]" {
		t.Errorf("file order %v, want the rail's order [1 2 3 0]", order)
	}
}

func reviewWithFiles(t *testing.T, w, h int, files []git.FileDiff) Model {
	t.Helper()
	isolateCache(t)
	m := newTestModel(withItems(3), withSize(w, h))
	m.projectRoot = t.TempDir()
	if err := os.MkdirAll(filepath.Join(m.projectRoot, ".grove"), 0o755); err != nil {
		t.Fatal(err)
	}
	next, _ := m.openPRReview(&tracker.PullRequest{Number: 42, Title: "Answer"})
	m = next.(Model)
	m = sendMsg(m, prReviewRepoMsg{gen: m.prReview.Gen, repo: "o/r"})
	return sendMsg(m, prDiffMsg{gen: m.prReview.Gen, files: files})
}

func currentPath(m Model) string { return m.prReview.Files[m.prReview.FileIdx].Path() }

// J and K walk the files in the rail's order and stop at both ends; a new
// file starts at its top.
func TestPRReview_FileKeys(t *testing.T) {
	m := reviewWithFiles(t, 120, 40, railFiles)
	if p := currentPath(m); p != "cmd/grove/main.go" {
		t.Fatalf("opens on %q, want the rail's first file", p)
	}
	m = sendKey(m, "K")
	if p := currentPath(m); p != "cmd/grove/main.go" {
		t.Errorf("K at the first file moved to %q", p)
	}
	m = sendKey(m, "j")
	m = sendKey(m, "J")
	if p := currentPath(m); p != "internal/tui/model.go" || m.prReview.Scroll != 0 {
		t.Errorf("J: %q scroll %d", p, m.prReview.Scroll)
	}
	m = sendKey(m, "ctrl+n")
	m = sendKey(m, "ctrl+n")
	m = sendKey(m, "J")
	if p := currentPath(m); p != "a.go" {
		t.Errorf("past the end: %q, want a.go", p)
	}
	if m = sendKey(m, "ctrl+p"); currentPath(m) != "internal/tui/view.go" {
		t.Errorf("ctrl+p: %q", currentPath(m))
	}
}

func TestPRReview_RailToggleAndWidth(t *testing.T) {
	for _, c := range []struct {
		w, h int
		want bool
	}{{120, 40, true}, {80, 24, false}, {44, 24, false}} {
		m := reviewWithFiles(t, c.w, c.h, railFiles)
		if got := strings.Contains(stripAnsi(m.viewString()), "internal/tui/ +4 -3"); got != c.want {
			t.Errorf("%dx%d: rail shown %v, want %v", c.w, c.h, got, c.want)
		}
		m = sendKey(m, "t")
		if got := strings.Contains(stripAnsi(m.viewString()), "internal/tui/ +4 -3"); got == c.want {
			t.Errorf("%dx%d: t did not flip the rail", c.w, c.h)
		}
	}
}

// The frame fills the terminal exactly, at every size and after resizing.
func TestPRReview_FrameFillsTerminal(t *testing.T) {
	check := func(m Model, w, h int, label string) {
		t.Helper()
		lines := strings.Split(m.viewString(), "\n")
		if len(lines) != h {
			t.Errorf("%s: %d lines, want %d", label, len(lines), h)
		}
		for i, l := range lines {
			if lw := lipgloss.Width(l); lw > w {
				t.Errorf("%s: line %d is %d wide", label, i, lw)
			}
		}
	}
	for _, s := range [][2]int{{44, 24}, {80, 24}, {110, 40}, {120, 40}} {
		check(reviewWithFiles(t, s[0], s[1], railFiles), s[0], s[1], fmt.Sprintf("%dx%d", s[0], s[1]))
	}
	m := reviewWithFiles(t, 120, 40, railFiles)
	for _, s := range [][2]int{{80, 24}, {120, 40}} {
		m = sendMsg(m, tea.WindowSizeMsg{Width: s[0], Height: s[1]})
		check(m, s[0], s[1], fmt.Sprintf("resized to %dx%d", s[0], s[1]))
	}
}

// With more files than rows, the rail scrolls so the current file shows.
func TestRenderRail_ScrollsToCurrent(t *testing.T) {
	var files []git.FileDiff
	for i := range 60 {
		files = append(files, diffFile(fmt.Sprintf("f%02d.go", i), 1, 0))
	}
	m := reviewWithFiles(t, 120, 20, files)
	for range 50 {
		m = sendKey(m, "J")
	}
	if p := currentPath(m); p != "f50.go" || !strings.Contains(stripAnsi(m.viewString()), "f50.go +1 -0") {
		t.Errorf("current %q not on screen", p)
	}
}

func TestPRReview_ViewedSurvivesRestart(t *testing.T) {
	m := reviewWithFiles(t, 120, 40, railFiles)
	m = sendKey(m, "v")
	root := m.projectRoot
	again := newTestModel(withItems(3), withSize(120, 40))
	again.projectRoot = root
	next, _ := again.openPRReview(&tracker.PullRequest{Number: 42})
	again = next.(Model)
	again = sendMsg(again, prReviewRepoMsg{gen: again.prReview.Gen, repo: "o/r"})
	again = sendMsg(again, prDiffMsg{gen: again.prReview.Gen, files: railFiles})
	if !strings.Contains(stripAnsi(again.viewString()), "✓ main.go") {
		t.Errorf("viewed mark lost after a restart:\n%s", stripAnsi(again.viewString()))
	}
	again = sendKey(again, "v")
	if strings.Contains(stripAnsi(again.viewString()), "✓ main.go") {
		t.Error("v again should unmark")
	}
}

// A file whose diff changed loses its mark.
func TestPRReview_ViewedDroppedWhenFileChanges(t *testing.T) {
	m := reviewWithFiles(t, 120, 40, railFiles)
	m = sendKey(m, "v")
	changed := append([]git.FileDiff(nil), railFiles...)
	changed[1] = diffFile("cmd/grove/main.go", 5, 1)
	again := newTestModel(withItems(3), withSize(120, 40))
	again.projectRoot = m.projectRoot
	next, _ := again.openPRReview(&tracker.PullRequest{Number: 42})
	again = next.(Model)
	again = sendMsg(again, prReviewRepoMsg{gen: again.prReview.Gen, repo: "o/r"})
	again = sendMsg(again, prDiffMsg{gen: again.prReview.Gen, files: changed})
	if strings.Contains(stripAnsi(again.viewString()), "✓ main.go") {
		t.Error("a changed file kept its viewed mark")
	}
}

func TestPRReview_FoldToggle(t *testing.T) {
	m := reviewWithFiles(t, 120, 40, []git.FileDiff{diffFile("big.go", 1, 6)})
	if strings.Contains(stripAnsi(m.viewString()), "removed lines") {
		t.Fatal("folded before z")
	}
	if m = sendKey(m, "z"); !strings.Contains(stripAnsi(m.viewString()), "… 6 removed lines") {
		t.Errorf("z did not fold:\n%s", stripAnsi(m.viewString()))
	}
}

// j, G, and g scroll the hunk pane within the file.
func TestPRReview_Scroll(t *testing.T) {
	m := reviewWithFiles(t, 80, 24, []git.FileDiff{diffFile("long.go", 80, 0)})
	m = sendKey(m, "j")
	if m.prReview.Scroll != 1 {
		t.Errorf("j: scroll %d", m.prReview.Scroll)
	}
	m = sendKey(m, "G")
	if !strings.Contains(stripAnsi(m.viewString()), "line 80") {
		t.Error("G should show the last line")
	}
	if m = sendKey(m, "g"); m.prReview.Scroll != 0 {
		t.Errorf("g: scroll %d", m.prReview.Scroll)
	}
}

func TestGolden_PRReviewFiles(t *testing.T) {
	cases := map[string]struct {
		size   termSize
		folded bool
	}{
		"rail_120x40":   {sizeWide, false},
		"no_rail_80x24": {sizeStandard, false},
		"folded_80x24":  {sizeStandard, true},
		"narrow_44x24":  {termSize{"narrow_44x24", 44, 24}, false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			m := goldenModel(t, c.size, withItems(3))
			next, _ := m.openPRReview(&tracker.PullRequest{Number: 42, Title: "Add user authentication flow"})
			m = next.(Model)
			m = sendMsg(m, prReviewRepoMsg{gen: m.prReview.Gen, repo: "o/r"})
			m = sendMsg(m, prDiffMsg{gen: m.prReview.Gen, files: append(railFiles[:3:3], diffFile("internal/tui/view.go", 2, 5))})
			if c.folded {
				m = sendKey(sendKey(sendKey(m, "J"), "J"), "z") // view.go, 5 removed lines
			}
			golden.RequireEqual(t, []byte(m.viewString()))
		})
	}
}

// The raised tab and the diff's colors are color-only (spec 18).
func TestGolden_Themed_PRReviewFiles(t *testing.T) {
	m := goldenModelThemed(t, sizeWide, withItems(3))
	next, _ := m.openPRReview(&tracker.PullRequest{Number: 42, Title: "Add user authentication flow"})
	m = next.(Model)
	m = sendMsg(m, prReviewRepoMsg{gen: m.prReview.Gen, repo: "o/r"})
	m = sendMsg(m, prDiffMsg{gen: m.prReview.Gen, files: railFiles})
	golden.RequireEqual(t, []byte(m.viewString()))
}

// t is a choice that holds across resizes: hidden stays hidden when the
// terminal narrows, shown stays shown.
func TestPRReview_RailChoiceSurvivesResize(t *testing.T) {
	m := reviewWithFiles(t, 120, 40, railFiles)
	m = sendKey(m, "t") // hide
	m = sendMsg(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sendMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	if strings.Contains(stripAnsi(m.viewString()), "internal/tui/ +4 -3") {
		t.Error("the rail came back after t hid it")
	}
	m = sendKey(m, "t") // show
	m = sendMsg(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	if !strings.Contains(stripAnsi(m.viewString()), "internal/tui/ +4 -3") {
		t.Error("the rail went away after t showed it")
	}
}

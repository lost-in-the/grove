package tui

import (
	"regexp"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestHeaderView_Format(t *testing.T) {
	h := Header{ProjectName: "grove", WorktreeCount: 5, SortLabel: "name", Cursor: 1, Total: 5}
	view := stripAnsi(h.View(80))
	if lipgloss.Width(view) != 80 {
		t.Errorf("header is %d cells, want 80", lipgloss.Width(view))
	}
	if !strings.HasPrefix(view, " grove · 5 worktrees · ⬍ name") {
		t.Errorf("left side: %q", view)
	}
	if !strings.HasSuffix(view, "1/5 ") {
		t.Errorf("right side should be the cursor position: %q", view)
	}
	for _, gone := range []string{"on main", "●"} {
		if strings.Contains(view, gone) {
			t.Errorf("header still shows %q: %q", gone, view)
		}
	}
}

func TestHeaderView_SortReversed(t *testing.T) {
	h := Header{ProjectName: "grove", WorktreeCount: 5, SortLabel: "age", SortReversed: true, Cursor: 2, Total: 5}
	if view := stripAnsi(h.View(80)); !strings.Contains(view, "⬍ age ↑") {
		t.Errorf("reversed sort not shown: %q", view)
	}
}

// At 44 columns the count drops its word and the project name truncates;
// the cursor position stays.
func TestHeaderView_Narrow(t *testing.T) {
	h := Header{ProjectName: "a-rather-long-project-name", WorktreeCount: 5, SortLabel: "name", Cursor: 3, Total: 5}
	view := stripAnsi(h.View(44))
	if lipgloss.Width(view) != 44 {
		t.Errorf("header is %d cells, want 44", lipgloss.Width(view))
	}
	if strings.Contains(view, "worktrees") {
		t.Errorf("count kept its word at 44: %q", view)
	}
	if !strings.Contains(view, "…") || !strings.HasSuffix(view, "3/5 ") {
		t.Errorf("want a truncated name and 3/5: %q", view)
	}
}

func TestDashboardHeader_TracksFilteredCursor(t *testing.T) {
	m := newTestModel(withItems(5), withSize(80, 24))
	m.list.Select(2)
	if h := m.buildDashboardHeader(); h.Cursor != 3 || h.Total != 5 {
		t.Errorf("cursor %d/%d, want 3/5", h.Cursor, h.Total)
	}
	m.list.SetFilterText("fix")
	if h := m.buildDashboardHeader(); h.Total != len(m.list.VisibleItems()) || h.Total == 5 {
		t.Errorf("total %d should track the %d filtered rows", h.Total, len(m.list.VisibleItems()))
	}
}

func TestHeaderView_LongProjectName(t *testing.T) {
	h := Header{ProjectName: "my-very-long-project-name-that-might-overflow", WorktreeCount: 2, Cursor: 1, Total: 2}
	view := h.View(50)
	if lipgloss.Width(view) > 50 {
		t.Errorf("header exceeded width 50: got %d", lipgloss.Width(view))
	}
}

func TestHeaderView_Unicode(t *testing.T) {
	h := Header{ProjectName: "проект", WorktreeCount: 3, Cursor: 1, Total: 3}
	view := h.View(80)
	plain := stripAnsi(view)
	if !strings.Contains(plain, "проект") {
		t.Errorf("header should contain unicode project name, got: %s", plain)
	}
}

func TestHeaderView_MinimumWidth(t *testing.T) {
	h := Header{ProjectName: "proj", WorktreeCount: 1, Cursor: 1, Total: 1}
	// Very narrow — should not panic
	view := h.View(5)
	if view == "" {
		t.Error("header should produce output even at very narrow widths")
	}
}

// stripAnsi removes ANSI escape codes for plain text comparison.
func stripAnsi(s string) string {
	var result strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}

// Under herdr the header counts agents that need you, after the worktree
// count and before the sort (spec 5.5).
func TestHeader_TriageSummary(t *testing.T) {
	h := Header{ProjectName: "grove", WorktreeCount: 5, Blocked: 2, Done: 1, SortLabel: "name", Cursor: 1, Total: 5}
	view := stripAnsi(h.View(100))
	if !strings.HasPrefix(view, " grove · 5 worktrees · ! 2 blocked · ✓ 1 done · ⬍ name") {
		t.Errorf("wide: %q", view)
	}

	view = stripAnsi(h.View(44))
	if !strings.Contains(view, "! 2 · ✓ 1") || strings.Contains(view, "blocked") {
		t.Errorf("at 44 the summary should drop its words: %q", view)
	}
	if lipgloss.Width(view) != 44 || !strings.HasSuffix(view, "1/5 ") {
		t.Errorf("at 44 the header should stay 44 cells with 1/5: %q", view)
	}

	h.Done = 0
	if view := stripAnsi(h.View(100)); strings.Contains(view, "✓") || !strings.Contains(view, "! 2 blocked · ⬍") {
		t.Errorf("a zero count should show nothing: %q", view)
	}
	h.Blocked = 0
	if view := stripAnsi(h.View(100)); strings.Contains(view, "!") {
		t.Errorf("no summary when nothing needs you: %q", view)
	}
}

func TestDashboardHeader_CountsAgentsOnlyWhenAgentAware(t *testing.T) {
	// withHerdrItems(5) cycles blocked, working, idle, done, unknown.
	m := newTestModel(withHerdrItems(5), withSize(100, 24))
	if h := m.buildDashboardHeader(); h.Blocked != 1 || h.Done != 1 {
		t.Errorf("herdr: blocked %d done %d, want 1 and 1", h.Blocked, h.Done)
	}
	m.agentAware = false
	if h := m.buildDashboardHeader(); h.Blocked != 0 || h.Done != 0 {
		t.Errorf("tmux: blocked %d done %d, want none", h.Blocked, h.Done)
	}
}

// The upgrade badge sits before the cursor position and never sheds, even
// beside the heaviest left side (spec 5.5).
func TestHeader_UpgradeBadge(t *testing.T) {
	h := Header{ProjectName: "grove", WorktreeCount: 5, SortLabel: "name", Upgrade: "0.12.0", Cursor: 1, Total: 5}
	if view := stripAnsi(h.View(100)); !strings.HasSuffix(view, "⭡0.12.0  1/5 ") {
		t.Errorf("wide: %q", view)
	}

	h = Header{ProjectName: "a-rather-long-project-name", WorktreeCount: 12, Blocked: 2, Done: 1,
		SortLabel: "attention", SortReversed: true, Upgrade: "0.12.0", Cursor: 10, Total: 10}
	view := stripAnsi(h.View(44))
	if lipgloss.Width(view) != 44 {
		t.Errorf("header is %d cells, want 44: %q", lipgloss.Width(view), view)
	}
	if !strings.HasSuffix(view, "⭡0.12.0  10/10 ") {
		t.Errorf("at 44 the badge and 10/10 should stay: %q", view)
	}
	// The clipped left side ends in an ellipsis, so the cut is visible.
	if left := strings.TrimRight(strings.TrimSuffix(view, "⭡0.12.0  10/10 "), " "); !strings.HasSuffix(left, "…") {
		t.Errorf("the clipped left side should end in …: %q", left)
	}
}

// Every cell of a header bar sits on the bar's background: a styled segment
// ends in a reset, which must not drop the background for what follows.
func TestHeaderBar_BackgroundCoversEveryCell(t *testing.T) {
	Colors = NewColorScheme()
	Styles = NewStyleSet(Colors)
	t.Cleanup(func() {
		Colors = NewColorScheme()
		Styles = NewStyleSet(Colors)
	})
	h := Header{ProjectName: "grove", WorktreeCount: 3, Blocked: 1, SortLabel: "name", Upgrade: "1.2.3", Cursor: 1, Total: 3}
	m := openConfigWith(t, 60, 24, nil)
	m = sendConfigKeys(t, m, "x")
	for name, bar := range map[string]string{
		"dashboard": h.View(60),
		"narrow":    h.View(30),
		"config":    m.configHeader(),
		"context":   renderContextHeader("Pull Requests", "grove", 60),
	} {
		if off := cellsOffBackground(bar); off != "" {
			t.Errorf("%s header: %q is off the bar background in %q", name, off, bar)
		}
	}
}

var sgr = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

// cellsOffBackground returns the text of a rendered line drawn with no
// background color set, or "" when every cell has one.
func cellsOffBackground(line string) string {
	var off strings.Builder
	bg := false
	at := 0
	for _, loc := range sgr.FindAllStringSubmatchIndex(line, -1) {
		if !bg {
			off.WriteString(line[at:loc[0]])
		}
		params := line[loc[2]:loc[3]]
		switch {
		case params == "" || params == "0":
			bg = false
		case strings.Contains(";"+params, ";48;"):
			bg = true
		}
		at = loc[1]
	}
	if !bg {
		off.WriteString(line[at:])
	}
	return off.String()
}

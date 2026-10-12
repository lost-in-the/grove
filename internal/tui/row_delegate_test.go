package tui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/list"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/lost-in-the/grove/internal/mux"
	"github.com/lost-in-the/grove/internal/plugins"
)

// renderRows renders every item through d at width and returns the raw
// lines, one entry per item (two-line rows joined by "\n").
func renderRows(d rowDelegate, items []WorktreeItem, width, selected int) []string {
	li := make([]list.Item, len(items))
	for i, it := range items {
		li[i] = it
	}
	l := list.New(li, d, width, 40)
	l.Select(selected)
	out := make([]string, len(items))
	for i := range items {
		var b bytes.Buffer
		d.Render(&b, l, i, li[i])
		out[i] = b.String()
	}
	return out
}

func rowFixture() []WorktreeItem {
	return []WorktreeItem{
		{
			ShortName: "root", Branch: "main", IsMain: true, IsCurrent: true, IsDirty: true,
			DirtyFiles: make([]string, 13), BehindCount: 11,
			CommitTime: testNow.Add(-48 * time.Hour), CommitMessage: "test commit for root",
			TmuxStatus:     string(mux.StatusAttached),
			PluginStatuses: []plugins.StatusEntry{{Short: "web", Level: plugins.StatusActive}},
		},
		{
			ShortName: "auth", Branch: "feat/oauth", AheadCount: 2,
			CommitTime: testNow.Add(-2 * time.Hour), TmuxStatus: string(mux.StatusDetached),
		},
		{ShortName: "login", Branch: "feature/login", CommitTime: testNow.Add(-5 * 7 * 24 * time.Hour)},
	}
}

func testRowDelegate() rowDelegate {
	return rowDelegate{Secondary: "branch", Now: func() time.Time { return testNow }}
}

func TestRowDelegate_OneLineAnatomy(t *testing.T) {
	rows := renderRows(testRowDelegate(), rowFixture(), 80, 0)
	line := ansi.Strip(rows[0])
	if strings.Contains(line, "\n") {
		t.Fatalf("one-line row has two lines: %q", line)
	}
	if !strings.HasPrefix(line, "❯ ● root") {
		t.Errorf("row should start with cursor, status, alias: %q", line)
	}
	for _, want := range []string{"↓11 ~13", "2d", "⬢ ◆"} {
		if !strings.Contains(line, want) {
			t.Errorf("row missing %q: %q", want, line)
		}
	}
	if !strings.HasSuffix(strings.TrimRight(line, " "), "⬢ ◆") {
		t.Errorf("tail should end the row: %q", line)
	}
}

func TestRowDelegate_SelectedKeepsStatus(t *testing.T) {
	rows := renderRows(testRowDelegate(), rowFixture(), 80, 1)
	line := ansi.Strip(rows[1])
	if !strings.HasPrefix(line, "❯ ○ auth") {
		t.Errorf("selected row should show cursor and status: %q", line)
	}
	if other := ansi.Strip(rows[0]); !strings.HasPrefix(other, "  ● root") {
		t.Errorf("unselected row: %q", other)
	}
}

// Every cell of the selected row carries the selection background: after a
// style reset, text never continues unstyled.
func TestRowDelegate_SelectedRowHasNoBareCells(t *testing.T) {
	if Colors.SelectionBg == nil {
		t.Skip("no colors")
	}
	raw := renderRows(testRowDelegate(), rowFixture(), 80, 0)[0]
	parts := strings.Split(raw, "\x1b[m")
	for i, p := range parts[:len(parts)-1] {
		if i > 0 && p != "" && !strings.HasPrefix(p, "\x1b[") {
			t.Fatalf("unstyled text after a reset: %q", p)
		}
	}
	if last := parts[len(parts)-1]; last != "" {
		t.Errorf("row ends with unstyled text: %q", last)
	}
}

func TestRowDelegate_ChipOnlyWhenBranchDiffers(t *testing.T) {
	rows := renderRows(testRowDelegate(), rowFixture(), 120, 0)
	if l := ansi.Strip(rows[1]); !strings.Contains(l, "⎇ feat/oauth") {
		t.Errorf("auth on feat/oauth should show a chip: %q", l)
	}
	if l := ansi.Strip(rows[2]); strings.Contains(l, "⎇") {
		t.Errorf("login on feature/login should not show a chip: %q", l)
	}
	off := testRowDelegate()
	off.Secondary = "off"
	for _, r := range renderRows(off, rowFixture(), 120, 0) {
		if strings.Contains(ansi.Strip(r), "⎇") {
			t.Errorf("list_secondary off still shows a chip: %q", ansi.Strip(r))
		}
	}
}

func TestRowDelegate_AgentColumnOnlyWhenAware(t *testing.T) {
	items := rowFixture()
	items[1].AgentStatus = mux.AgentBlocked
	plain := ansi.Strip(renderRows(testRowDelegate(), items, 80, 0)[1])
	if strings.Contains(plain, "!") {
		t.Errorf("agent glyph drawn without agentAware: %q", plain)
	}
	aware := testRowDelegate()
	aware.AgentAware = true
	rows := renderRows(aware, items, 80, 0)
	if l := ansi.Strip(rows[1]); !strings.HasPrefix(l, "!   ○ auth") {
		t.Errorf("agent lead column: %q", l)
	}
	if l := ansi.Strip(rows[0]); !strings.HasPrefix(l, "  ❯ ● root") {
		t.Errorf("rows without an agent keep the lead column blank: %q", l)
	}
}

func TestRowDelegate_AgentGlyphs(t *testing.T) {
	aware := testRowDelegate()
	aware.AgentAware = true
	for status, glyph := range map[mux.AgentStatus]string{
		mux.AgentBlocked: "!", mux.AgentWorking: "▸", mux.AgentDone: "✓",
	} {
		items := rowFixture()
		items[2].AgentStatus = status
		if l := ansi.Strip(renderRows(aware, items, 80, 0)[2]); !strings.HasPrefix(l, glyph+" ") {
			t.Errorf("%s: row = %q, want lead %q", status, l, glyph)
		}
	}
}

func TestRowDelegate_TwoLineShowsCommit(t *testing.T) {
	d := testRowDelegate()
	d.TwoLine = true
	if d.Height() != 2 {
		t.Fatalf("height = %d, want 2", d.Height())
	}
	lines := strings.Split(ansi.Strip(renderRows(d, rowFixture(), 80, 0)[0]), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[1], "    test commit for root") {
		t.Errorf("second line should be the commit, indented to the alias: %q", lines)
	}
}

func TestRowDelegate_WidthIsExact(t *testing.T) {
	for _, two := range []bool{false, true} {
		d := testRowDelegate()
		d.TwoLine = two
		d.AgentAware = two
		for w := 44; w <= 160; w++ {
			for i, r := range renderRows(d, rowFixture(), w, 0) {
				for _, line := range strings.Split(r, "\n") {
					if got := lipgloss.Width(line); got != w {
						t.Fatalf("two=%v width %d row %d: line is %d cells: %q", two, w, i, got, ansi.Strip(line))
					}
				}
			}
		}
	}
}

// agentAware comes from the first fetch and never flips, so the agent
// column cannot appear or vanish mid-session.
func TestAgentAware_SetOnceFromFirstFetch(t *testing.T) {
	m := newTestModel(withSize(120, 40))
	next, _ := m.Update(worktreesFetchedMsg{items: makeTestItems(2), backend: "herdr"})
	m = next.(Model)
	if !m.agentAware {
		t.Fatal("first fetch on herdr should make the dashboard agent-aware")
	}
	next, _ = m.Update(worktreesFetchedMsg{items: makeTestItems(2), backend: "tmux"})
	if !next.(Model).agentAware {
		t.Error("a later fetch flipped agentAware off")
	}

	tm := newTestModel(withSize(120, 40))
	next, _ = tm.Update(worktreesFetchedMsg{items: makeTestItems(2), backend: "tmux"})
	next, _ = next.(Model).Update(worktreesFetchedMsg{items: makeTestItems(2), backend: "herdr"})
	if next.(Model).agentAware {
		t.Error("a later fetch flipped agentAware on")
	}
}

package tui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/list"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/lost-in-the/grove/internal/config"
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
		if p == "" {
			continue
		}
		// The SGR sequences that open this segment must set a background.
		lead := p[:len(p)-len(strings.TrimLeft(p, "\x1b[0123456789;:m"))]
		if !strings.Contains(lead, "48;") && !strings.Contains(lead, "48:") {
			t.Fatalf("segment %d has no selection background: %q", i, p)
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
		d.Pins = two
		d.PinOf = map[string]string{"auth": "9"}
		for w := 44; w <= 200; w++ {
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

func TestStartupCompact(t *testing.T) {
	yes, no := true, false
	for _, c := range []struct {
		name         string
		config, pref *bool
		want         bool
	}{
		{"default is one line", nil, nil, true},
		{"saved false keeps two lines", nil, &no, false},
		{"config false gives two lines", &no, nil, false},
		{"saved pref beats config", &no, &yes, true},
	} {
		if got := startupCompact(c.config, c.pref); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestDensity_VToggles(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m.projectRoot = t.TempDir()
	m.compactMode = true
	m.refreshDelegate()
	m = sendKey(m, "v")
	if h := m.rowDelegate().Height(); h != 2 {
		t.Errorf("after v: height %d, want 2", h)
	}
	m = sendKey(m, "v")
	if h := m.rowDelegate().Height(); h != 1 {
		t.Errorf("after v again: height %d, want 1", h)
	}
}

func TestListSecondaryFromConfig(t *testing.T) {
	for in, want := range map[string]string{"": "branch", "branch": "branch", "dir": "dir", "off": "off"} {
		cfg := &config.Config{TUI: config.TUIConfig{ListSecondary: in}}
		if got := listSecondary(cfg); got != want {
			t.Errorf("listSecondary(%q) = %q, want %q", in, got, want)
		}
	}
	if got := listSecondary(nil); got != "branch" {
		t.Errorf("nil config = %q", got)
	}
}

// The first fetch decides agentAware even when no backend was available.
func TestAgentAware_SetByFirstFetchWithoutBackend(t *testing.T) {
	m := newTestModel(withSize(120, 40))
	next, _ := m.Update(worktreesFetchedMsg{items: makeTestItems(2), backend: ""})
	next, _ = next.(Model).Update(worktreesFetchedMsg{items: makeTestItems(2), backend: "herdr"})
	if next.(Model).agentAware {
		t.Error("a later fetch turned agentAware on after a first fetch with no backend")
	}
}

// The focused config button draws surface-colored text on Primary, the pair
// TestPalette_ButtonContrast checks (TextBright on Primary is under 3:1).
func TestConfigForm_FocusedButtonUsesSurfaceText(t *testing.T) {
	s := groveHuhTheme().Theme(true)
	if got := s.Focused.FocusedButton.GetForeground(); got != Colors.SurfaceBg {
		t.Errorf("focused button text = %v, want SurfaceBg %v", got, Colors.SurfaceBg)
	}
}

func TestRowDelegate_PinColumnShowsSlot(t *testing.T) {
	d := testRowDelegate()
	d.Pins = true
	d.PinOf = map[string]string{"auth": "2"}
	rows := renderRows(d, rowFixture(), 80, 0)
	if l := ansi.Strip(rows[1]); !strings.HasPrefix(l, "2   ○ auth") {
		t.Errorf("pinned row: %q", l)
	}
	if l := ansi.Strip(rows[0]); !strings.HasPrefix(l, "  ❯ ● root") {
		t.Errorf("unpinned row keeps a blank pin column: %q", l)
	}
}

func TestDashboard_PinColumnOnlyWithPins(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	if m.rowDelegate().Pins {
		t.Error("pin column drawn with no pins")
	}
	m.pins = pinSet{"1": "root"}
	if d := m.rowDelegate(); !d.Pins || d.PinOf["root"] != "1" {
		t.Errorf("delegate = %+v, want the pin column with root in slot 1", d)
	}
}

func TestRowDelegate_LastActiveMarker(t *testing.T) {
	d := testRowDelegate()
	d.LastActive = "auth"
	rows := renderRows(d, rowFixture(), 80, 0)
	if l := ansi.Strip(rows[1]); !strings.HasPrefix(l, "↩ ○ auth") {
		t.Errorf("last-active row: %q", l)
	}
	rows = renderRows(d, rowFixture(), 80, 1)
	if l := ansi.Strip(rows[1]); !strings.HasPrefix(l, "❯ ○ auth") {
		t.Errorf("selected last-active row shows the cursor: %q", l)
	}
}

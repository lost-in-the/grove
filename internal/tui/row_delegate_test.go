package tui

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/list"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/mux"
	"github.com/lost-in-the/grove/internal/plugins"
	"github.com/lost-in-the/grove/internal/theme"
)

// renderRows renders every item through d at width and returns the raw
// rows, one entry per item (a detailed row's two lines joined by "\n").
func renderRows(d rowDelegate, items []WorktreeItem, width, selected int) []string {
	l := rowList(d, items, width)
	l.Select(selected)
	out := make([]string, len(items))
	for i, li := range l.Items() {
		var b bytes.Buffer
		d.Render(&b, l, i, li)
		out[i] = b.String()
	}
	return out
}

func rowList(d rowDelegate, items []WorktreeItem, width int) list.Model {
	li := make([]list.Item, len(items))
	for i, it := range items {
		li[i] = it
	}
	return list.New(li, d, width, 40)
}

// plainRows is renderRows without styling.
func plainRows(d rowDelegate, items []WorktreeItem, width, selected int) []string {
	rows := renderRows(d, items, width, selected)
	for i, r := range rows {
		rows[i] = ansi.Strip(r)
	}
	return rows
}

// rowFixture is a realistic list: the main checkout, a selected-looking
// feature, a clean one, a folder off the naming pattern, and a very long
// name on a very long branch with every state symbol.
func rowFixture() []WorktreeItem {
	return []WorktreeItem{
		{
			ShortName: "root", Path: "/src/myapp", Branch: "main", IsMain: true, IsCurrent: true, IsDirty: true,
			DirtyFiles: make([]string, 1), BehindCount: 12,
			CommitTime: testNow.Add(-3 * 7 * 24 * time.Hour), CommitMessage: "test commit for root",
			TmuxStatus:     string(mux.StatusAttached),
			PluginStatuses: []plugins.StatusEntry{{Short: "web", Level: plugins.StatusActive}},
		},
		{
			ShortName: "checkout", Path: "/src/myapp-checkout", Branch: "feat/checkout-redesign", IsDirty: true,
			DirtyFiles: make([]string, 13), CommitTime: testNow.Add(-2 * 24 * time.Hour),
			CommitMessage: "redesign the checkout", TmuxStatus: string(mux.StatusDetached),
		},
		{
			ShortName: "fix-login", Path: "/src/myapp-fix-login", Branch: "fix/login-timeout",
			CommitTime: testNow.Add(-5 * time.Hour), CommitMessage: "time out the login",
		},
		{
			ShortName: "legacy-api", Path: "/src/legacy-api", Branch: "feat/api-v2-pagination", FolderOffPattern: true,
			IsDirty: true, DirtyFiles: make([]string, 2), AheadCount: 3,
			CommitTime: testNow.Add(-24 * time.Hour), CommitMessage: "page the api",
		},
		{
			ShortName: "payments-reconciliation-audit", Path: "/src/myapp-payments-reconciliation-audit",
			Branch: "agent/payments-reconciliation-audit-x7k2p", IsDirty: true, DirtyFiles: make([]string, 4),
			AheadCount: 1, BehindCount: 2, CommitTime: testNow.Add(-8 * 30 * 24 * time.Hour),
			CommitMessage: "audit the reconciliation",
		},
	}
}

func testRowDelegate() rowDelegate {
	return rowDelegate{Secondary: "branch", Now: func() time.Time { return testNow }}
}

func detailedDelegate() rowDelegate {
	d := testRowDelegate()
	d.Detailed = true
	return d
}

// cellIndex is the display column where sub starts in line, or -1.
func cellIndex(line, sub string) int {
	i := strings.Index(line, sub)
	if i < 0 {
		return -1
	}
	return lipgloss.Width(line[:i])
}

// --- compact ---

func TestCompactRow_Anatomy(t *testing.T) {
	rows := plainRows(testRowDelegate(), rowFixture(), 78, 1)
	line := rows[1]
	if strings.Contains(line, "\n") {
		t.Fatalf("compact row has two lines: %q", line)
	}
	if !strings.HasPrefix(line, "❯ ● checkout ") {
		t.Errorf("row should start with cursor, status, name: %q", line)
	}
	for _, want := range []string{"feat/checkout-redesign ·", "· ~13", "2d", "⬡"} {
		if !strings.Contains(line, want) {
			t.Errorf("row missing %q: %q", want, line)
		}
	}
	if strings.Contains(line, "redesign the checkout") {
		t.Errorf("compact row shows the commit subject: %q", line)
	}
}

// Every branch starts in one column, and the state and age end in one.
func TestCompactRow_ColumnsAlign(t *testing.T) {
	for _, w := range []int{42, 64, 70, 78, 94} {
		items := rowFixture()
		rows := plainRows(testRowDelegate(), items, w, 0)
		col := -1
		for i, r := range rows {
			start := cellIndex(r, items[i].Branch[:4])
			if start < 0 {
				t.Fatalf("width %d: row %d has no branch: %q", w, i, r)
			}
			if col >= 0 && start != col {
				t.Errorf("width %d: row %d branch at %d, others at %d:\n%s", w, i, start, col, strings.Join(rows, "\n"))
			}
			col = start
		}
	}
}

// A dotted leader joins the branch to the state; with no state it runs to
// the end of the state column.
func TestCompactRow_Leader(t *testing.T) {
	rows := plainRows(testRowDelegate(), rowFixture(), 78, 0)
	if !strings.Contains(rows[2], "fix/login-timeout ···") {
		t.Errorf("no leader after the branch: %q", rows[2])
	}
	stateEnd := cellIndex(rows[1], "~13") + 3
	if got := cellIndex(rows[2], "· ") + 1; got != stateEnd {
		t.Errorf("leader of a row without state ends at %d, want the state column end %d:\n%s\n%s", got, stateEnd, rows[1], rows[2])
	}
}

func TestCompactRow_MismatchMark(t *testing.T) {
	rows := plainRows(testRowDelegate(), rowFixture(), 78, 0)
	if !strings.Contains(rows[3], "legacy-api ≠ ") {
		t.Errorf("off-pattern folder has no ≠ after the name: %q", rows[3])
	}
	for _, i := range []int{0, 1, 2, 4} {
		if strings.Contains(rows[i], "≠") {
			t.Errorf("row %d has ≠: %q", i, rows[i])
		}
	}
}

// Long names and branches are cut in the middle, keeping both ends.
func TestCompactRow_LongNameAndBranchCutInTheMiddle(t *testing.T) {
	for _, w := range []int{42, 70, 78} {
		r := plainRows(testRowDelegate(), rowFixture(), w, 0)[4]
		name := strings.Fields(r)[1]
		if !strings.HasPrefix(name, "pay") || !strings.HasSuffix(name, "audit") || !strings.Contains(name, "…") {
			t.Errorf("width %d: name %q should keep its start and end around …: %q", w, name, r)
		}
		branch := strings.Fields(r)[2]
		head := "agent/"
		if w < 60 {
			head = "agen"
		}
		if !strings.HasPrefix(branch, head) || !strings.HasSuffix(branch, "x7k2p") || !strings.Contains(branch, "…") {
			t.Errorf("width %d: branch %q should keep its start and end around …: %q", w, branch, r)
		}
	}
}

func TestCompactRow_AllStateSymbols(t *testing.T) {
	r := plainRows(testRowDelegate(), rowFixture(), 78, 0)[4]
	if !strings.Contains(r, "↑1 ↓2 ~4") {
		t.Errorf("row with every state symbol: %q", r)
	}
}

// Pins have their strip; compact rows draw no digit for them.
func TestCompactRow_NoPinColumn(t *testing.T) {
	d := testRowDelegate()
	d.Pins = true
	d.PinOf = map[string]string{"checkout": "2"}
	rows := plainRows(d, rowFixture(), 78, 0)
	if !strings.HasPrefix(rows[1], "  ● checkout") {
		t.Errorf("compact row with a pin: %q", rows[1])
	}
}

func TestCompactHeading(t *testing.T) {
	d := testRowDelegate()
	items := rowFixture()
	l := rowList(d, items, 78)
	h := ansi.Strip(d.heading(l))
	if lipgloss.Width(h) != 78 {
		t.Errorf("heading is %d cells, want 78", lipgloss.Width(h))
	}
	row := plainRows(d, items, 78, 0)[1]
	for word, under := range map[string]string{"NAME": "checkout", "BRANCH": "feat/"} {
		if got, want := cellIndex(h, word), cellIndex(row, under); got != want {
			t.Errorf("%s at %d, its column at %d:\n%s\n%s", word, got, want, h, row)
		}
	}
	if got, want := cellIndex(h, "STATE")+5, cellIndex(row, "~13")+3; got != want {
		t.Errorf("STATE ends at %d, the state column at %d:\n%s\n%s", got, want, h, row)
	}
	if got, want := cellIndex(h, "AGE")+3, cellIndex(row, "2d")+2; got != want {
		t.Errorf("AGE ends at %d, the age column at %d:\n%s\n%s", got, want, h, row)
	}
	if d.Detailed = true; d.heading(l) != "" {
		t.Error("detailed mode has a heading row")
	}
}

// list_secondary picks compact mode's second column.
func TestCompactRow_ListSecondary(t *testing.T) {
	items := rowFixture()
	dir := testRowDelegate()
	dir.Secondary = "dir"
	rows := plainRows(dir, items, 78, 0)
	if !strings.Contains(rows[1], "myapp-checkout ·") || strings.Contains(rows[1], "feat/") {
		t.Errorf("dir: %q", rows[1])
	}
	if h := ansi.Strip(dir.heading(rowList(dir, items, 78))); !strings.Contains(h, "FOLDER") {
		t.Errorf("dir heading: %q", h)
	}

	off := testRowDelegate()
	off.Secondary = "off"
	rows = plainRows(off, items, 78, 0)
	if !strings.Contains(rows[1], "checkout ···") || strings.Contains(rows[1], "feat/") {
		t.Errorf("off: the leader should follow the name: %q", rows[1])
	}
	if h := ansi.Strip(off.heading(rowList(off, items, 78))); strings.Contains(h, "BRANCH") || strings.Contains(h, "FOLDER") {
		t.Errorf("off heading: %q", h)
	}
}

// --- detailed ---

func TestDetailedRow_Anatomy(t *testing.T) {
	d := detailedDelegate()
	if d.Height() != 2 || d.Spacing() != 1 {
		t.Fatalf("height %d spacing %d, want 2 and 1", d.Height(), d.Spacing())
	}
	lines := strings.Split(plainRows(d, rowFixture(), 78, 1)[1], "\n")
	if len(lines) != 2 {
		t.Fatalf("detailed row: %q", lines)
	}
	if !strings.HasPrefix(lines[0], "❯ ● checkout │ myapp-checkout/ ··") {
		t.Errorf("line one: %q", lines[0])
	}
	if !strings.Contains(lines[0], "·· ") || !strings.Contains(lines[0], " 2d ⬡") {
		t.Errorf("line one should lead to the age, then the tail: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "    ⎇ feat/checkout-redesign   ~13") {
		t.Errorf("line two: %q", lines[1])
	}
	for _, l := range lines {
		if strings.Contains(l, "redesign the checkout") {
			t.Errorf("detailed row shows the commit subject: %q", l)
		}
	}
}

// The name is whole at every width; the folder and branch are cut in the
// middle instead.
func TestDetailedRow_NameNeverCut(t *testing.T) {
	aware := detailedDelegate()
	aware.AgentAware, aware.Pins = true, true
	for w := 42; w <= 120; w++ {
		if lines := strings.Split(plainRows(aware, rowFixture(), w, 0)[4], "\n"); !strings.Contains(lines[0], "payments-reconciliation-audit") {
			t.Fatalf("width %d with the pin and agent columns: name cut: %q", w, lines[0])
		}
		lines := strings.Split(plainRows(detailedDelegate(), rowFixture(), w, 0)[4], "\n")
		if !strings.Contains(lines[0], "payments-reconciliation-audit") {
			t.Fatalf("width %d: name cut: %q", w, lines[0])
		}
		if i := strings.Index(lines[0], "│ "); i >= 0 {
			folder := strings.Fields(lines[0][i+len("│ "):])[0]
			if lipgloss.Width(folder) < rowFolderMin+1 || !strings.HasPrefix(folder, "mya") || !strings.HasSuffix(folder, "t/") {
				t.Errorf("width %d: folder %q should keep its start and end", w, folder)
			}
		}
		branch := strings.Fields(lines[1])[1]
		if !strings.HasPrefix(branch, "agent/") || !strings.HasSuffix(branch, "x7k2p") {
			t.Errorf("width %d: branch %q should keep its start and end", w, branch)
		}
	}
}

// With no room for line one's tail, it ends line two instead.
func TestDetailedRow_TailMovesWhenTheNameFillsLineOne(t *testing.T) {
	d := detailedDelegate()
	d.AgentAware, d.Pins = true, true
	items := rowFixture()
	items[4].TmuxStatus = string(mux.StatusDetached)
	lines := strings.Split(plainRows(d, items, 42, 0)[4], "\n")
	if strings.Contains(lines[0], "⬡") || !strings.HasSuffix(strings.TrimRight(lines[1], " "), "⬡") {
		t.Errorf("the tail should end line two:\n%s", strings.Join(lines, "\n"))
	}
}

// When the folder has no room, ≠ still marks a folder off the pattern.
func TestDetailedRow_MismatchMarkWithoutFolder(t *testing.T) {
	d := detailedDelegate()
	d.AgentAware, d.Pins = true, true
	for w := 42; w <= 60; w++ {
		one := strings.Split(plainRows(d, rowFixture(), w, 0)[3], "\n")[0]
		if !strings.Contains(one, "legacy-api │ ") && !strings.Contains(one, "legacy-api ≠") {
			t.Errorf("width %d: neither the folder nor ≠: %q", w, one)
		}
	}
}

func TestDetailedRow_PinColumn(t *testing.T) {
	d := detailedDelegate()
	d.Pins = true
	d.PinOf = map[string]string{"checkout": "2"}
	rows := plainRows(d, rowFixture(), 78, 1)
	if !strings.HasPrefix(rows[1], "2 ❯ ● checkout") {
		t.Errorf("pinned row: %q", rows[1])
	}
	if !strings.HasPrefix(rows[0], "    ● root") {
		t.Errorf("unpinned row keeps the pin column blank: %q", rows[0])
	}
	if two := strings.Split(rows[1], "\n")[1]; !strings.HasPrefix(two, "      ⎇ feat/") {
		t.Errorf("line two sits under the name: %q", two)
	}
}

// --- both modes ---

func eachMode(t *testing.T, fn func(t *testing.T, d rowDelegate)) {
	t.Helper()
	for _, detailed := range []bool{false, true} {
		d := testRowDelegate()
		d.Detailed = detailed
		name := "compact"
		if detailed {
			name = "detailed"
		}
		t.Run(name, func(t *testing.T) { fn(t, d) })
	}
}

func TestRow_WidthIsExact(t *testing.T) {
	eachMode(t, func(t *testing.T, d rowDelegate) {
		for _, aware := range []bool{false, true} {
			d.AgentAware = aware
			d.Pins = aware
			d.PinOf = map[string]string{"checkout": "9"}
			for w := 42; w <= 200; w++ {
				for i, r := range renderRows(d, rowFixture(), w, 0) {
					for _, line := range strings.Split(r, "\n") {
						if got := lipgloss.Width(line); got != w {
							t.Fatalf("aware=%v width %d row %d: line is %d cells: %q", aware, w, i, got, ansi.Strip(line))
						}
					}
				}
			}
		}
	})
}

func TestRow_SelectedKeepsStatus(t *testing.T) {
	eachMode(t, func(t *testing.T, d rowDelegate) {
		rows := plainRows(d, rowFixture(), 78, 2)
		if !strings.HasPrefix(rows[2], "❯ ○ fix-login") {
			t.Errorf("selected row should show cursor and status: %q", rows[2])
		}
		if !strings.HasPrefix(rows[0], "  ● root") {
			t.Errorf("unselected row: %q", rows[0])
		}
	})
}

// Every cell of the selected row's text lines carries the selection
// background: after a style reset, text never continues unstyled.
func TestRow_SelectedRowHasNoBareCells(t *testing.T) {
	if Colors.SelectionBg == nil {
		t.Skip("no colors")
	}
	eachMode(t, func(t *testing.T, d rowDelegate) {
		for _, raw := range strings.Split(renderRows(d, rowFixture(), 78, 3)[3], "\n") {
			parts := strings.Split(raw, "\x1b[m")
			for i, p := range parts[:len(parts)-1] {
				if p == "" {
					continue
				}
				lead := p[:len(p)-len(strings.TrimLeft(p, "\x1b[0123456789;:m"))]
				if !strings.Contains(lead, "48;") && !strings.Contains(lead, "48:") {
					t.Fatalf("segment %d has no selection background: %q", i, p)
				}
			}
			if last := parts[len(parts)-1]; last != "" {
				t.Errorf("row ends with unstyled text: %q", last)
			}
		}
	})
}

// Beside the docked form the list has no focus: no cursor, no bar, and
// the selected name stays bold.
func TestRow_Unfocused(t *testing.T) {
	eachMode(t, func(t *testing.T, d rowDelegate) {
		d.Unfocused = true
		raw := renderRows(d, rowFixture(), 78, 1)[1]
		if strings.Contains(raw, "❯") {
			t.Errorf("unfocused row has a cursor: %q", ansi.Strip(raw))
		}
		if Colors.SelectionBg != nil && strings.Contains(raw, "48;") {
			t.Errorf("unfocused row has the selection bar: %q", raw)
		}
		if !strings.Contains(raw, Styles.SelectedItem.Render("checkout")) {
			t.Errorf("unfocused selected name is not bold: %q", raw)
		}
	})
}

func TestRow_AgentColumnOnlyWhenAware(t *testing.T) {
	eachMode(t, func(t *testing.T, d rowDelegate) {
		items := rowFixture()
		items[2].AgentStatus = mux.AgentBlocked
		if r := plainRows(d, items, 78, 0)[2]; strings.HasPrefix(r, "!") {
			t.Errorf("agent glyph drawn without agentAware: %q", r)
		}
		d.AgentAware = true
		rows := plainRows(d, items, 78, 0)
		if !strings.HasPrefix(rows[2], "!   ○ fix-login") {
			t.Errorf("agent lead column: %q", rows[2])
		}
		if !strings.HasPrefix(rows[0], "  ❯ ● root") {
			t.Errorf("rows without an agent keep the lead column blank: %q", rows[0])
		}
	})
}

func TestRow_AgentGlyphs(t *testing.T) {
	eachMode(t, func(t *testing.T, d rowDelegate) {
		d.AgentAware = true
		for status, glyph := range map[mux.AgentStatus]string{
			mux.AgentBlocked: "!", mux.AgentWorking: "▸", mux.AgentDone: "✓",
		} {
			items := rowFixture()
			items[2].AgentStatus = status
			if l := plainRows(d, items, 78, 0)[2]; !strings.HasPrefix(l, glyph+" ") {
				t.Errorf("%s: row = %q, want lead %q", status, l, glyph)
			}
		}
	})
}

func TestRow_LastActiveMarker(t *testing.T) {
	eachMode(t, func(t *testing.T, d rowDelegate) {
		d.LastActive = "fix-login"
		if l := plainRows(d, rowFixture(), 78, 0)[2]; !strings.HasPrefix(l, "↩ ○ fix-login") {
			t.Errorf("last-active row: %q", l)
		}
		if l := plainRows(d, rowFixture(), 78, 2)[2]; !strings.HasPrefix(l, "❯ ○ fix-login") {
			t.Errorf("selected last-active row shows the cursor: %q", l)
		}
		d.LastActive = "root"
		if l := plainRows(d, rowFixture(), 78, 1)[0]; strings.Contains(l, "↩") {
			t.Errorf("current worktree shows ↩: %q", l)
		}
	})
}

// The session and container glyphs end line one; the cell after them is
// kept blank for a future pull request mark.
func TestRow_TailAndReservedCell(t *testing.T) {
	eachMode(t, func(t *testing.T, d rowDelegate) {
		first := strings.Split(plainRows(d, rowFixture(), 78, 1)[0], "\n")[0]
		if !strings.HasSuffix(first, "⬢ ◆  ") {
			t.Errorf("line one should end with the session, the container, and the blank pull request cell: %q", first)
		}
	})
}

func TestRow_StaleStatus(t *testing.T) {
	eachMode(t, func(t *testing.T, d rowDelegate) {
		items := rowFixture()
		items[2].IsPrunable = true
		if l := plainRows(d, items, 78, 0)[2]; !strings.HasPrefix(l, "  ✗ fix-login") {
			t.Errorf("stale row: %q", l)
		}
	})
}

// --- color ---

// themed sets the default palette for one test.
func themed(t *testing.T) {
	t.Helper()
	goldenMu.Lock()
	Colors = theme.DefaultColorScheme()
	Styles = NewStyleSet(Colors)
	t.Cleanup(func() {
		Colors = NewColorScheme()
		Styles = NewStyleSet(Colors)
		goldenMu.Unlock()
	})
}

// Each element is drawn in its token: the name brighter than line two, the
// folder muted, rules dim, the branch in the accent.
func TestRow_Tokens(t *testing.T) {
	themed(t)
	rule := lipgloss.NewStyle().Foreground(Colors.SurfaceDim)
	dots := strings.TrimSuffix(rule.Render("·"), "\x1b[m") // the rule color, then a dot
	d := detailedDelegate()
	rows := renderRows(d, rowFixture(), 78, 0)
	check := func(what, raw, want string) {
		t.Helper()
		if !strings.Contains(raw, want) {
			t.Errorf("%s: want %q in\n%q", what, want, raw)
		}
	}
	check("detailed name", rows[1], Styles.TextBright.Render("checkout"))
	check("divider", rows[1], rule.Render("│"))
	check("folder", rows[1], Styles.TextMuted.Render("myapp-checkout/"))
	check("leader", rows[1], dots+"··")
	check("branch", rows[1], lipgloss.NewStyle().Foreground(Colors.Primary).Render("⎇ feat/checkout-redesign"))
	check("off-pattern folder", rows[3], Styles.StatusWarning.Render("legacy-api/"))
	check("dirty count", rows[1], Styles.StatusWarning.Render("~13"))
	check("age", rows[1], Styles.TextMuted.Render("2d"))

	c := renderRows(testRowDelegate(), rowFixture(), 78, 0)
	check("compact name", c[1], Styles.NormalItem.Render("checkout"))
	check("compact branch", c[1], lipgloss.NewStyle().Foreground(Colors.Primary).Render("feat/checkout-redesign"))
	check("compact leader", c[1], dots)
	check("≠", c[3], Styles.StatusWarning.Render("≠"))
	cd := testRowDelegate()
	check("heading", cd.heading(rowList(cd, rowFixture(), 78)), Styles.DimmedItem.Render("NAME"))
}

// --- the model ---

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

// The first fetch decides agentAware even when no backend was available.
func TestAgentAware_SetByFirstFetchWithoutBackend(t *testing.T) {
	m := newTestModel(withSize(120, 40))
	next, _ := m.Update(worktreesFetchedMsg{items: makeTestItems(2), backend: ""})
	next, _ = next.(Model).Update(worktreesFetchedMsg{items: makeTestItems(2), backend: "herdr"})
	if next.(Model).agentAware {
		t.Error("a later fetch turned agentAware on after a first fetch with no backend")
	}
}

func TestStartupListChoice(t *testing.T) {
	yes, no := true, false
	for _, c := range []struct {
		name         string
		config, pref *bool
		want         *bool
	}{
		{"nothing saved: the layout decides", nil, nil, nil},
		{"saved compact", nil, &yes, &yes},
		{"saved detailed", nil, &no, &no},
		{"config detailed", &no, nil, &no},
		{"config compact", &yes, nil, &yes},
		{"saved choice beats config", &no, &yes, &yes},
	} {
		got := startupListChoice(c.config, c.pref)
		if (got == nil) != (c.want == nil) || (got != nil && *got != *c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

// With no saved choice the list is compact beside the pane and detailed
// alone; a saved choice holds at every width.
func TestListMode_DefaultByLayout(t *testing.T) {
	yes, no := true, false
	for _, c := range []struct {
		width    int
		choice   *bool
		detailed bool
	}{
		{160, nil, false}, {120, nil, false}, {110, nil, false},
		{109, nil, true}, {80, nil, true}, {44, nil, true},
		{80, &yes, false}, {120, &no, true},
	} {
		m := newTestModel(withItems(3))
		m.compactChoice = c.choice
		m = withSizeModel(m, c.width, 24)
		if got := m.rowDelegate().Detailed; got != c.detailed {
			t.Errorf("width %d choice %v: detailed %v, want %v", c.width, c.choice, got, c.detailed)
		}
	}
}

func withSizeModel(m Model, w, h int) Model {
	withSize(w, h)(&m)
	return m
}

// v switches the mode, saves the choice with the key the old toggle used,
// and the footer names the mode it switches to.
func TestListMode_VTogglesAndSaves(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m.projectRoot = t.TempDir()
	if err := os.MkdirAll(filepath.Join(m.projectRoot, ".grove"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := saveUIPrefs(m.projectRoot, &UIPrefs{Pins: map[string]string{"1": "root"}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stripAnsi(m.viewString()), "v detailed") {
		t.Fatalf("compact footer should offer v detailed:\n%s", stripAnsi(m.viewString()))
	}
	m = sendKey(m, "v")
	if !m.rowDelegate().Detailed {
		t.Fatal("after v: still compact")
	}
	prefs := loadUIPrefs(m.projectRoot)
	if prefs == nil || prefs.CompactMode == nil || *prefs.CompactMode {
		t.Fatalf("after v: saved %+v, want compact_mode false", prefs)
	}
	if prefs.Pins["1"] != "root" {
		t.Error("saving the mode dropped the pins")
	}
	if !strings.Contains(stripAnsi(m.viewString()), "v compact") {
		t.Errorf("detailed footer should offer v compact:\n%s", stripAnsi(m.viewString()))
	}
	m = sendKey(m, "v")
	if m.rowDelegate().Detailed {
		t.Error("after v again: still detailed")
	}
	if p := loadUIPrefs(m.projectRoot); p.CompactMode == nil || !*p.CompactMode {
		t.Errorf("after v again: saved %+v, want compact_mode true", p.CompactMode)
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

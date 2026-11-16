package tui

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/exp/golden"

	"github.com/lost-in-the/grove/internal/config"
)

// ruleRow is the row of the rule between the list and the context panel.
func ruleRow(t *testing.T, view string) int {
	t.Helper()
	for i, l := range strings.Split(stripAnsi(view), "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "────") {
			return i
		}
	}
	t.Fatalf("no rule in:\n%s", stripAnsi(view))
	return -1
}

// dividerCol is the column of the divider between the list and the context
// panel in the two-column layout, or -1 when the frame is stacked. It checks
// the divider runs unbroken from the first body row to the footer.
func dividerCol(t *testing.T, m Model) int {
	t.Helper()
	lines := strings.Split(stripAnsi(m.viewString()), "\n")
	top := 3 // header, tabs, blank
	col := slices.Index([]rune(lines[top]), '│')
	if col < 0 {
		return -1
	}
	bottom := m.height - m.configLayout().footerH
	for i := top; i < bottom; i++ {
		if r := []rune(lines[i]); col >= len(r) || r[col] != '│' {
			t.Errorf("divider breaks at row %d:\n%s", i, stripAnsi(m.viewString()))
			break
		}
	}
	return col
}

// bodyMark is where the list ends: the rule's row when stacked, the
// divider's column when side by side.
func bodyMark(t *testing.T, m Model) string {
	t.Helper()
	if col := dividerCol(t, m); col >= 0 {
		return fmt.Sprintf("divider at column %d", col)
	}
	return fmt.Sprintf("rule on row %d", ruleRow(t, m.viewString()))
}

// checkConfigFrame checks the frame fills the terminal exactly and no line
// is wider than it.
func checkConfigFrame(t *testing.T, m Model, w, h int, label string) {
	t.Helper()
	lines := strings.Split(m.viewString(), "\n")
	if len(lines) != h {
		t.Errorf("%s: frame has %d lines, want %d", label, len(lines), h)
	}
	for i, l := range lines {
		if lw := lipgloss.Width(l); lw > w {
			t.Errorf("%s: line %d is %d cells, wider than %d", label, i, lw, w)
		}
	}
}

// One layout at a given size: every section, every focused field, and an
// edit in progress put the rule on the same row (or the divider on the same
// column) and fill the terminal.
func TestConfig_LayoutStableAcrossSectionsAndFields(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}, {44, 24}, {160, 50}, {270, 80}} {
		w, h := size[0], size[1]
		m := openConfigWith(t, w, h, nil)
		mark := bodyMark(t, m)
		for tab := ConfigTab(0); tab < ConfigTabCount; tab++ {
			m = sendConfigKeys(t, m, string(rune('1'+tab)))
			for i, f := range m.configState.Fields[tab] {
				label := fmt.Sprintf("%dx%d %s", w, h, f.Key)
				checkConfigFrame(t, m, w, h, label)
				if got := bodyMark(t, m); got != mark {
					t.Errorf("%s: %s, want %s", label, got, mark)
				}
				if f.Type == ConfigString || f.Type == ConfigList {
					edit := sendConfigKeys(t, m, "enter")
					checkConfigFrame(t, edit, w, h, label+" editing")
					if got := bodyMark(t, edit); got != mark {
						t.Errorf("%s editing: %s, want %s", label, got, mark)
					}
				}
				if i < len(m.configState.Fields[tab])-1 {
					m = sendConfigKeys(t, m, "down")
				}
			}
		}
	}
}

// At 80x24 nothing is cut: every field's whole context fits, with every
// option and its meaning.
func TestConfig_NothingClippedAt80x24(t *testing.T) {
	m := openConfigWith(t, 80, 24, func(c *config.Config) { c.ProjectsDir = "~/projects" })
	for tab := ConfigTab(0); tab < ConfigTabCount; tab++ {
		m = sendConfigKeys(t, m, string(rune('1'+tab)))
		for i, f := range m.configState.Fields[tab] {
			view := stripAnsi(m.viewString())
			if strings.Contains(view, "…") {
				t.Errorf("%s: something is truncated:\n%s", f.Key, view)
			}
			if !strings.Contains(view, f.Description[:min(len(f.Description), 30)]) {
				t.Errorf("%s: description missing:\n%s", f.Key, view)
			}
			for _, o := range f.Options {
				if !strings.Contains(view, o.Meaning) {
					t.Errorf("%s: option %s's meaning missing:\n%s", f.Key, o.Value, view)
				}
			}
			for _, other := range m.configState.Fields[tab] {
				if !strings.Contains(view, other.Label) {
					t.Errorf("%s: row %s missing:\n%s", f.Key, other.Label, view)
				}
			}
			if i < len(m.configState.Fields[tab])-1 {
				m = sendConfigKeys(t, m, "down")
			}
		}
	}
}

// The footer names the keys that work on the focused field.
func TestConfig_FooterMatchesTheField(t *testing.T) {
	m := openConfigWith(t, 80, 24, nil)
	footer := func(m Model) string { return stripAnsi(m.configFooter()) }

	m = focusField(t, m, "switch.dirty_handling")
	if f := footer(m); !strings.Contains(f, "←→/hl change") || strings.Contains(f, "enter edit") || strings.Contains(f, "u undo") {
		t.Errorf("on a choice, nothing changed yet: %q", f)
	}
	m = sendConfigKeys(t, m, "l")
	if f := footer(m); !strings.Contains(f, "u undo") {
		t.Errorf("after a change: %q, want u undo", f)
	}
	m = focusField(t, m, "naming.pattern")
	if f := footer(m); !strings.Contains(f, "enter edit") || strings.Contains(f, "←→") {
		t.Errorf("on a text field: %q", f)
	}
	m = sendConfigKeys(t, m, "enter")
	if f := footer(m); !strings.Contains(f, "enter save") || !strings.Contains(f, "esc cancel") || strings.Contains(f, "↑↓") {
		t.Errorf("while editing: %q", f)
	}
}

// j/k and h/l work as well as the arrows, so the footer names them, joined
// with a slash as the other footers join a key pair. While a field is being
// typed in, they type, so they are not named then, and the mode word says
// type.
func TestConfig_FooterShowsVimKeys(t *testing.T) {
	m := openConfigWith(t, 80, 24, nil)
	footer := func(m Model) string { return stripAnsi(m.configFooter()) }

	m = focusField(t, m, "switch.dirty_handling")
	if f := footer(m); !strings.HasPrefix(strings.TrimSpace(f), "move · ↑↓/jk move") || !strings.Contains(f, "←→/hl change") {
		t.Errorf("on a choice: %q", f)
	}
	m = focusField(t, m, "naming.pattern")
	if f := footer(m); !strings.Contains(f, "↑↓/jk move") || !strings.Contains(f, "enter edit") || strings.Contains(f, "hl") {
		t.Errorf("on a text field: %q", f)
	}
	m = sendConfigKeys(t, m, "enter")
	if f := footer(m); strings.Contains(f, "jk") || strings.Contains(f, "hl") || !strings.HasPrefix(strings.TrimSpace(f), "type ·") {
		t.Errorf("while editing, j and k type: %q", f)
	}
}

// At the narrowest width every hint still shows, in the footer's reserved
// lines, on every kind of field and after a change.
func TestConfig_FooterFitsAt44(t *testing.T) {
	m := openConfigWith(t, 44, 24, nil)
	m = writeIgnoringDisk(t, focusField(t, m, "switch.dirty_handling"), "l")
	for _, key := range []string{"switch.dirty_handling", "naming.pattern"} {
		m = focusField(t, m, key)
		f := m.configFooter()
		if got, want := lipgloss.Height(f), m.configLayout().footerH; got != want {
			t.Errorf("%s: footer is %d lines, reserved %d", key, got, want)
		}
		plain := stripAnsi(f)
		for _, hint := range []string{"move · ", "↑↓/jk move", "saves as you go", "u undo", "esc close"} {
			if !strings.Contains(plain, hint) {
				t.Errorf("%s: footer lacks %q:\n%s", key, hint, plain)
			}
		}
	}
}

// contextTitle is the context panel's first line and the one after it,
// without styling or the list column.
func contextTitle(t *testing.T, m Model) (string, string) {
	t.Helper()
	lines := strings.Split(stripAnsi(m.viewString()), "\n")
	first := 3
	if col := dividerCol(t, m); col >= 0 {
		cut := func(l string) string { return strings.TrimSpace(string([]rune(l)[col+1:])) }
		return cut(lines[first]), cut(lines[first+1])
	}
	first = ruleRow(t, m.viewString()) + 1
	return strings.TrimSpace(lines[first]), strings.TrimSpace(lines[first+1])
}

// The config key sits right after the title, two spaces on, wherever the
// panel is; when both do not fit, the key takes the next line.
func TestConfig_KeyFollowsTitle(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}, {270, 80}, {44, 24}} {
		m := openConfigWith(t, size[0], size[1], nil)
		m = focusField(t, m, "switch.dirty_handling")
		if got, _ := contextTitle(t, m); got != "Uncommitted files  switch.dirty_handling" {
			t.Errorf("%dx%d: title line %q", size[0], size[1], got)
		}
	}
	m := openConfigWith(t, 44, 24, nil)
	m = focusField(t, m, "plugins.docker.auto_stop")
	if title, next := contextTitle(t, m); title != "Stack down on leaving" || next != "plugins.docker.auto_stop" {
		t.Errorf("44x24: want the key under the title, got %q then %q", title, next)
	}
}

// The panel goes side by side at the dashboard's split width; below it the
// list, rule, and context stack. 80 columns stacks.
func TestConfig_LayoutByWidth(t *testing.T) {
	for _, c := range []struct {
		w, h int
		wide bool
	}{
		{44, 24, false}, {80, 24, false}, {splitWidth - 1, 40, false},
		{splitWidth, 40, true}, {120, 40, true}, {160, 50, true}, {270, 80, true},
	} {
		m := openConfigWith(t, c.w, c.h, nil)
		if got := dividerCol(t, m) >= 0; got != c.wide {
			t.Errorf("%dx%d: two columns %v, want %v:\n%s", c.w, c.h, got, c.wide, stripAnsi(m.viewString()))
		}
		if !c.wide {
			ruleRow(t, m.viewString())
		}
	}
}

// When stacked, the rule sits right under the tallest section's list,
// however tall the terminal.
func TestConfig_StackedRuleUnderTallestList(t *testing.T) {
	for _, h := range []int{24, 60} {
		m := openConfigWith(t, 100, h, nil)
		tallest := 0
		for _, fields := range m.configState.Fields {
			tallest = max(tallest, len(configListShape(fields)))
		}
		if got, want := ruleRow(t, m.viewString()), 3+tallest; got != want {
			t.Errorf("100x%d: rule on row %d, want %d", h, got, want)
		}
	}
}

// Side by side, the list is a modest column whatever the width: the
// selection bar spans it only, a long value ends in an ellipsis there, and
// the context wraps at a readable measure.
func TestConfig_WideColumns(t *testing.T) {
	long := "~/projects/a/very/long/folder/that/does/not/fit/in/the/list"
	for _, w := range []int{120, 270} {
		m := openConfigWith(t, w, 50, func(c *config.Config) { c.ProjectsDir = long })
		m = focusField(t, m, "projects_dir")
		col := dividerCol(t, m)
		if col < 44 || col > 58 {
			t.Errorf("%d wide: divider at column %d, want the list 44 to 56 wide", w, col)
		}
		l := m.configLayout()
		sel := m.configState.listRow(*m.configState.focused(), true, l)
		if got := lipgloss.Width(sel); got != l.listW {
			t.Errorf("%d wide: selected row is %d cells, want the list's %d", w, got, l.listW)
		}
		lines := strings.Split(stripAnsi(m.viewString()), "\n")
		var row string
		for _, line := range lines {
			if strings.Contains(line, "❯") {
				row = string([]rune(line)[:col])
			}
		}
		if !strings.Contains(row, "…") {
			t.Errorf("%d wide: long value not cut with an ellipsis: %q", w, row)
		}
		var ctx strings.Builder
		for i, line := range lines[3 : m.height-l.footerH] {
			right := strings.TrimRight(string([]rune(line)[col+2:]), " ") // past the divider's padding
			if got := lipgloss.Width(right); got > configMeasure {
				t.Errorf("%d wide: context row %d is %d cells, past the measure: %q", w, i, got, right)
			}
			ctx.WriteString(strings.ReplaceAll(right, " ", ""))
		}
		if !strings.Contains(ctx.String(), long) {
			t.Errorf("%d wide: context lacks the full value:\n%s", w, stripAnsi(m.viewString()))
		}
	}
}

// The "saved" note goes when its timer fires, unless a newer write
// restarted it.
func TestConfig_SavedNoteExpires(t *testing.T) {
	m := openConfigWith(t, 80, 24, nil)
	m = focusField(t, m, "switch.dirty_handling")
	m = sendConfigKeys(t, m, "l")
	old := m.configState.flashGen
	m = sendConfigKeys(t, m, "l")
	m = sendMsg(m, configFlashDoneMsg{gen: old})
	if h := configHeaderLine(m); !strings.Contains(h, "saved") {
		t.Errorf("an old timer cleared the note: %q", h)
	}
	m = sendMsg(m, configFlashDoneMsg{gen: m.configState.flashGen})
	if h := configHeaderLine(m); strings.Contains(h, "saved") {
		t.Errorf("the note should be gone: %q", h)
	}
}

func TestConfig_HeaderAndTabs(t *testing.T) {
	m := openConfigWith(t, 100, 30, nil)
	lines := strings.Split(stripAnsi(m.viewString()), "\n")
	if !strings.Contains(lines[0], "test-project ▸ settings") {
		t.Errorf("header: %q", lines[0])
	}
	for _, tab := range []string{"[1] general", "[2] behavior", "[3] plugins", "[4] protection"} {
		if !strings.Contains(lines[1], tab) {
			t.Errorf("tab row lacks %q: %q", tab, lines[1])
		}
	}

	m = openConfigWith(t, 44, 24, nil)
	m = sendConfigKeys(t, m, "3")
	row := strings.Split(stripAnsi(m.viewString()), "\n")[1]
	if !strings.Contains(row, "[3] plugins") || strings.Contains(row, "general") || !strings.Contains(row, "[1]") {
		t.Errorf("narrow tab row should name only the active tab: %q", row)
	}
}

// A resize keeps the frame filling the terminal.
func TestConfig_Resize(t *testing.T) {
	m := openConfigWith(t, 80, 24, nil)
	for _, size := range [][2]int{{120, 40}, {60, 20}, {44, 24}} {
		m = sendMsg(m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		for tab := ConfigTab(0); tab < ConfigTabCount; tab++ {
			m = sendConfigKeys(t, m, string(rune('1'+tab)))
			checkConfigFrame(t, m, size[0], size[1], fmt.Sprintf("%dx%d %s", size[0], size[1], tabName(tab)))
		}
	}
}

func TestRenderConfig_Loading(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	m = sendKey(m, "c")
	if v := stripAnsi(m.viewString()); !strings.Contains(v, "Loading settings") {
		t.Errorf("expected a loading message:\n%s", v)
	}
	checkConfigFrame(t, m, 80, 24, "loading")
}

// goldenConfig opens the panel on the golden fixture at size, focused on
// key, over a dashboard of root alone, so no field warns.
func goldenConfig(t *testing.T, size termSize, key string) Model {
	t.Helper()
	return goldenConfigOver(t, size, key, 1)
}

// goldenConfigOver is goldenConfig over a dashboard of n worktrees, root
// included.
func goldenConfigOver(t *testing.T, size termSize, key string, n int) Model {
	t.Helper()
	m := goldenModel(t, size, withItems(n), withConfigOverlay())
	return focusField(t, m, key)
}

func TestGolden_Config(t *testing.T) {
	wide := termSize{"wide_120x40", 120, 40}
	wider := termSize{"wide_160x50", 160, 50}
	narrow := termSize{"narrow_44x24", 44, 24}
	cases := []struct {
		name string
		size termSize
		key  string
	}{
		{"general_80x24", sizeStandard, "project_name"},
		{"behavior_80x24", sizeStandard, "switch.dirty_handling"},
		{"plugins_80x24", sizeStandard, "plugins.docker.auto_start"},
		{"protection_80x24", sizeStandard, "protection.protected"},
		{"behavior_44x24", narrow, "switch.dirty_handling"},
		{"behavior_120x40", wide, "switch.dirty_handling"},
		{"text_field_80x24", sizeStandard, "naming.pattern"},
		{"text_field_120x40", wide, "naming.pattern"},
		{"behavior_160x50", wider, "switch.dirty_handling"},
		{"text_field_160x50", wider, "naming.pattern"},
		{"protection_44x24", narrow, "protection.protected"},
		{"plugins_44x24", narrow, "plugins.docker.auto_stop"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := goldenConfig(t, c.size, c.key)
			golden.RequireEqual(t, []byte(m.viewString()))
		})
	}
}

// States reached through keys: an edit, a write, a failed write, and a
// rejected value.
func TestGolden_Config_States(t *testing.T) {
	t.Run("text_edit_80x24", func(t *testing.T) {
		m := goldenConfig(t, sizeStandard, "naming.pattern")
		m = sendConfigKeys(t, m, "enter")
		m = typeConfig(t, m, "-wt")
		golden.RequireEqual(t, []byte(m.viewString()))
	})
	t.Run("text_invalid_80x24", func(t *testing.T) {
		m := goldenConfig(t, sizeStandard, "naming.pattern")
		m = sendConfigKeys(t, m, "enter", "ctrl+u")
		m = typeConfig(t, m, "{name}")
		m = sendConfigKeys(t, m, "enter")
		golden.RequireEqual(t, []byte(m.viewString()))
	})
	t.Run("saved_80x24", func(t *testing.T) {
		m := goldenConfig(t, sizeStandard, "switch.dirty_handling")
		m = writeIgnoringDisk(t, m, "l")
		golden.RequireEqual(t, []byte(m.viewString()))
	})
	t.Run("error_80x24", func(t *testing.T) {
		m := goldenConfig(t, sizeStandard, "switch.dirty_handling")
		failWrites(t, errors.New("open .grove/config.toml: permission denied"))
		m = sendConfigKeys(t, m, "l")
		golden.RequireEqual(t, []byte(m.viewString()))
	})
	t.Run("error_44x24", func(t *testing.T) {
		m := goldenConfig(t, termSize{"narrow_44x24", 44, 24}, "switch.dirty_handling")
		failWrites(t, errors.New("open .grove/config.toml: permission denied"))
		m = sendConfigKeys(t, m, "l")
		golden.RequireEqual(t, []byte(m.viewString()))
	})
}

// The project name and folder pattern with two worktrees besides root: the
// warning, and the confirm step after a changed value.
func TestGolden_Config_Rename(t *testing.T) {
	wide := termSize{"wide_120x40", 120, 40}
	narrow := termSize{"narrow_44x24", 44, 24}
	for _, c := range []struct {
		name    string
		size    termSize
		key     string
		confirm string
	}{
		{"warning_80x24", sizeStandard, "project_name", ""},
		{"warning_120x40", wide, "project_name", ""},
		{"warning_44x24", narrow, "project_name", ""},
		{"pattern_warning_80x24", sizeStandard, "naming.pattern", ""},
		{"confirm_80x24", sizeStandard, "project_name", "shop"},
		{"confirm_120x40", wide, "project_name", "shop"},
		{"pattern_confirm_80x24", sizeStandard, "naming.pattern", "{project}.{name}"},
		{"pattern_confirm_44x24", narrow, "naming.pattern", "{project}.{name}"},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := goldenConfigOver(t, c.size, c.key, 3)
			if c.confirm != "" {
				m = editTo(t, m, c.confirm)
			}
			golden.RequireEqual(t, []byte(m.viewString()))
		})
	}
}

// writeIgnoringDisk sends keys with writes that succeed without touching a
// file, for goldens of the state after a write. The reload after a write
// reads an empty temp project, not this repository's config.
func writeIgnoringDisk(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	write := saveProjectConfig
	saveProjectConfig = func(map[string]string) error { return nil }
	defer func() {
		saveProjectConfig = write
		_ = os.Chdir(cwd) // back to the package, where the goldens live
	}()
	return sendConfigKeys(t, m, keys...)
}

// This one opens the panel the way the dashboard does, on the default
// config, so the Behavior section shows every field it really has.
func TestGolden_Config_BehaviorDefaults(t *testing.T) {
	m := goldenModel(t, sizeStandard, withItems(3))
	m = sendKey(m, "c")
	m = sendConfigMsg(t, m, configLoadedMsg{cfg: config.LoadDefaults()})
	m = sendConfigKeys(t, m, "2")
	golden.RequireEqual(t, []byte(m.viewString()))
}

// The raised tab and the selected row are color only, so they need a
// themed golden (spec 18).
func TestGolden_Themed_ConfigTabs(t *testing.T) {
	m := goldenModelThemed(t, sizeStandard, withItems(3), withConfigOverlay())
	m = focusField(t, m, "plugins.docker.auto_start")
	golden.RequireEqual(t, []byte(m.viewString()))
}

// flatView is the context panel's text with its line breaks and padding
// folded to single spaces, so wrapped sentences read whole.
func flatView(t *testing.T, m Model) string {
	t.Helper()
	lines := strings.Split(stripAnsi(m.viewString()), "\n")
	var ctx []string
	if col := dividerCol(t, m); col >= 0 {
		for _, l := range lines[3 : m.height-m.configLayout().footerH] {
			ctx = append(ctx, string([]rune(l)[col+1:]))
		}
	} else {
		ctx = lines[ruleRow(t, m.viewString())+1 : m.height-m.configLayout().footerH]
	}
	return strings.Join(strings.Fields(strings.Join(ctx, "\n")), " ")
}

const (
	warnTwo = "⚠ This project has 2 worktrees besides root. Changing this does not rename their folders. Grove will stop recognizing them by their short names."
	warnOne = "⚠ This project has 1 worktree besides root. Changing this does not rename its folder. Grove will stop recognizing it by its short name."
)

// Project name and Folder pattern warn, under their description, when other
// worktrees would lose their short names; no other field does, and neither
// does with root alone.
func TestConfig_RenameWarning(t *testing.T) {
	for _, c := range []struct {
		items int
		want  string
	}{{1, ""}, {2, warnOne}, {3, warnTwo}} {
		m, _ := openConfigOver(t, 80, 24, "", c.items)
		for _, tab := range m.configState.Fields {
			for _, f := range tab {
				m = focusField(t, m, f.Key)
				view := flatView(t, m)
				renames := f.Key == "project_name" || f.Key == "naming.pattern"
				switch {
				case !renames || c.want == "":
					if strings.Contains(view, "⚠") {
						t.Errorf("%d items, %s: unexpected warning:\n%s", c.items, f.Key, stripAnsi(m.viewString()))
					}
				case !strings.Contains(view, c.want):
					t.Errorf("%d items, %s: want %q in:\n%s", c.items, f.Key, c.want, stripAnsi(m.viewString()))
				case strings.Index(view, c.want) < strings.Index(view, strings.Fields(f.Description)[0]):
					t.Errorf("%d items, %s: the warning should follow the description:\n%s", c.items, f.Key, stripAnsi(m.viewString()))
				}
			}
		}
	}
}

// The confirm step says what the second enter does and what esc keeps, and
// the footer names only those two keys.
func TestConfig_ConfirmStepText(t *testing.T) {
	for _, c := range []struct{ key, file, typed, want string }{
		{"project_name", "project_name = \"myapp\"\n", "shop", `Press enter again to change. esc keeps "myapp".`},
		{"project_name", "", "shop", "Press enter again to change. esc leaves it not set."},
		{"naming.pattern", "", "{project}.{name}", `Press enter again to change. esc keeps "{project}-{name}".`},
	} {
		m, _ := openConfigIn(t, 80, 24, c.file)
		m = editTo(t, focusField(t, m, c.key), c.typed)
		if view := flatView(t, m); !strings.Contains(view, c.want) || !strings.Contains(view, warnTwo) {
			t.Errorf("%s: want %q and the warning in:\n%s", c.key, c.want, stripAnsi(m.viewString()))
		}
		if f := strings.TrimSpace(stripAnsi(m.configFooter())); f != "type · enter change · esc cancel" {
			t.Errorf("%s: footer %q, want type · enter change · esc cancel", c.key, f)
		}
	}
}

// The warning and the confirm step fit the frame every other field uses:
// the same rule row or divider, the terminal filled, the warning whole.
func TestConfig_ConfirmStepKeepsTheLayout(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {44, 24}, {120, 40}, {160, 50}} {
		w, h := size[0], size[1]
		for _, c := range renamingFields {
			m, _ := openConfigIn(t, w, h, c.file)
			mark := bodyMark(t, m)
			m = focusField(t, m, c.key)
			for _, state := range []string{"focused", "confirm"} {
				if state == "confirm" {
					m = editTo(t, m, c.typed)
				}
				label := fmt.Sprintf("%dx%d %s %s", w, h, c.key, state)
				checkConfigFrame(t, m, w, h, label)
				if got := bodyMark(t, m); got != mark {
					t.Errorf("%s: %s, want %s", label, got, mark)
				}
				if view := flatView(t, m); !strings.Contains(view, warnTwo) {
					t.Errorf("%s: the warning is cut:\n%s", label, stripAnsi(m.viewString()))
				}
				if state == "confirm" && !strings.Contains(flatView(t, m), "Press enter again to change.") {
					t.Errorf("%s: the confirm line is missing:\n%s", label, stripAnsi(m.viewString()))
				}
				if w == 80 && strings.Contains(flatView(t, m), "…") {
					t.Errorf("%s: something is cut:\n%s", label, stripAnsi(m.viewString()))
				}
			}
		}
	}
}

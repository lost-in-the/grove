package tui

import (
	"errors"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/BurntSushi/toml"

	"github.com/lost-in-the/grove/internal/config"
)

func TestConfigOverlay_OpenClose(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m = sendKey(m, "c")
	if m.activeView() != ViewConfig || m.configState == nil {
		t.Fatalf("c: view %v, want the settings panel", m.activeView())
	}
	m = sendKey(m, "esc")
	if m.activeView() != ViewDashboard || m.configState != nil {
		t.Errorf("esc: view %v, want the dashboard", m.activeView())
	}
}

func TestConfigOverlay_NilState(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.setViewForTest(ViewConfig)
	m.configState = nil
	if m = sendKey(m, "enter"); m.activeView() != ViewDashboard {
		t.Errorf("a panel with no state should close: view %v", m.activeView())
	}
}

func TestConfigLoadedMsg_Error(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m = sendKey(m, "c")
	m = sendMsg(m, configLoadedMsg{err: errTest})
	if m.configState.Err == nil {
		t.Fatal("the load error should be kept")
	}
	if f := m.configState.focused(); f != nil {
		t.Errorf("no field should have focus after a failed load, got %s", f.Key)
	}
	view := stripAnsi(m.viewString())
	if !strings.Contains(view, "test error") {
		t.Errorf("the load error should show:\n%s", view)
	}
	if m = sendKey(m, "esc"); m.activeView() != ViewDashboard {
		t.Errorf("esc after a failed load: view %v, want closed", m.activeView())
	}
}

// ↑↓ and j/k stop at the ends; tab and shift+tab wrap.
func TestConfig_MoveBetweenFields(t *testing.T) {
	m := openConfigWith(t, 80, 24, nil)
	m = sendConfigKeys(t, m, "2")
	behavior := m.configState.Fields[ConfigTabBehavior]
	last := len(behavior) - 1

	steps := []struct {
		key  string
		want int
	}{
		{"down", 1}, {"j", 2}, {"up", 1}, {"k", 0}, {"k", 0},
		{"shift+tab", last}, {"tab", 0}, {"tab", 1},
	}
	for _, s := range steps {
		m = sendConfigKeys(t, m, s.key)
		if got := m.configState.cursor[ConfigTabBehavior]; got != s.want {
			t.Errorf("after %s: cursor %d, want %d", s.key, got, s.want)
		}
	}
	for range last + 2 {
		m = sendConfigKeys(t, m, "down")
	}
	if got := m.configState.cursor[ConfigTabBehavior]; got != last {
		t.Errorf("down past the end: cursor %d, want %d", got, last)
	}
}

// Changing a choice writes that one key at once, in its own section.
func TestConfig_ChangeWritesTheKeyAtOnce(t *testing.T) {
	m, path := openConfigIn(t, 80, 24, "project_name = \"myapp\"\n")
	m = focusField(t, m, "switch.dirty_handling")

	m = sendConfigKeys(t, m, "l")
	if got := m.configState.focused().Value; got != "ignore" {
		t.Errorf("after l: value %q, want ignore", got)
	}
	text := readConfigFile(t, path)
	var onDisk struct {
		ProjectName string `toml:"project_name"`
		Switch      config.SwitchConfig
	}
	if _, err := toml.Decode(text, &onDisk); err != nil {
		t.Fatalf("written file is not TOML: %v\n%s", err, text)
	}
	if onDisk.Switch.DirtyHandling != "ignore" || onDisk.ProjectName != "myapp" {
		t.Errorf("file should gain dirty_handling = ignore and keep the rest:\n%s", text)
	}
	if h := configHeaderLine(m); !strings.Contains(h, "✓ saved") {
		t.Errorf("header after a write: %q, want ✓ saved", h)
	}

	m = sendConfigKeys(t, m, "right", "left", "h")
	if got := m.configState.focused().Value; got != "warn" {
		t.Errorf("after right, left, h: value %q, want warn", got)
	}
	if !strings.Contains(readConfigFile(t, path), `dirty_handling = "warn"`) {
		t.Errorf("file should hold the last value:\n%s", readConfigFile(t, path))
	}

	// ← from the first option wraps to the last, as the branch form's do.
	m = sendConfigKeys(t, m, "h")
	if got := m.configState.focused().Value; got != config.DirtyHandlingValues[len(config.DirtyHandlingValues)-1] {
		t.Errorf("h from the first option: value %q, want the last", got)
	}
}

func TestConfig_BoolTogglesWithSpaceAndArrows(t *testing.T) {
	m, path := openConfigIn(t, 80, 24, "[plugins.docker]\nenabled = true\n")
	m = focusField(t, m, "plugins.docker.enabled")

	m = sendConfigKeys(t, m, "space")
	if got := m.configState.focused().Value; got != boolFalse {
		t.Errorf("after space: %q, want false", got)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("file no longer loads: %v\n%s", err, readConfigFile(t, path))
	}
	if e := cfg.Plugins.Docker.Enabled; e == nil || *e {
		t.Errorf("enabled should load back false:\n%s", readConfigFile(t, path))
	}
	if m = sendConfigKeys(t, m, "right"); m.configState.focused().Value != boolTrue {
		t.Errorf("after right: %q, want true", m.configState.focused().Value)
	}
}

// space changes only a true/false field; on a choice it does nothing.
func TestConfig_SpaceLeavesChoices(t *testing.T) {
	m, path := openConfigIn(t, 80, 24, "")
	m = focusField(t, m, "switch.dirty_handling")
	if m = sendConfigKeys(t, m, "space"); m.configState.focused().Value != "warn" {
		t.Errorf("space on a choice changed it to %q", m.configState.focused().Value)
	}
	if text := readConfigFile(t, path); text != "" {
		t.Errorf("nothing should be written:\n%s", text)
	}
}

// u writes back the value before the last change, newest first, and moves
// to the field it changed.
func TestConfig_UndoWritesThePreviousValue(t *testing.T) {
	m, path := openConfigIn(t, 80, 24, "")
	m = focusField(t, m, "switch.dirty_handling")
	m = sendConfigKeys(t, m, "l") // ignore
	m = focusField(t, m, "plugins.docker.auto_stop")
	m = sendConfigKeys(t, m, "space") // true

	m = sendConfigKeys(t, m, "u")
	if !strings.Contains(readConfigFile(t, path), "auto_stop = false") {
		t.Errorf("first undo should write auto_stop back to false:\n%s", readConfigFile(t, path))
	}
	m = sendConfigKeys(t, m, "1", "u")
	if !strings.Contains(readConfigFile(t, path), `dirty_handling = "warn"`) {
		t.Errorf("second undo should write dirty_handling back to warn:\n%s", readConfigFile(t, path))
	}
	if m.configState.Tab != ConfigTabBehavior || focusedKey(m) != "switch.dirty_handling" {
		t.Errorf("undo should move to the field it changed: section %s, focus %s", tabName(m.configState.Tab), focusedKey(m))
	}
	if h := configHeaderLine(m); !strings.Contains(h, "undone") {
		t.Errorf("header after undo: %q, want undone", h)
	}

	before := readConfigFile(t, path)
	m = sendConfigKeys(t, m, "u")
	if readConfigFile(t, path) != before {
		t.Error("u with nothing to undo should write nothing")
	}
	if h := configHeaderLine(m); !strings.Contains(h, "nothing to undo") {
		t.Errorf("header after an empty undo: %q, want nothing to undo", h)
	}
}

// A failed write keeps the value on disk, shows the error, and leaves
// nothing to undo.
func TestConfig_FailedWriteKeepsTheValue(t *testing.T) {
	m, _ := openConfigIn(t, 80, 24, "")
	m = focusField(t, m, "switch.dirty_handling")
	failWrites(t, errors.New("disk full"))

	m = sendConfigKeys(t, m, "l")
	if got := m.configState.focused().Value; got != "warn" {
		t.Errorf("after a failed write: value %q, want warn", got)
	}
	view := stripAnsi(m.viewString())
	if h := configHeaderLine(m); !strings.Contains(h, "not saved") {
		t.Errorf("header: %q, want not saved", h)
	}
	if !strings.Contains(view, "disk full") {
		t.Errorf("the error should be shown:\n%s", view)
	}
	if len(m.configState.undo) != 0 {
		t.Errorf("a failed write left %d undo steps", len(m.configState.undo))
	}
}

// failWrites makes every config write fail with err until the test ends.
func failWrites(t *testing.T, err error) {
	t.Helper()
	write := saveProjectConfig
	saveProjectConfig = func(map[string]string) error { return err }
	t.Cleanup(func() { saveProjectConfig = write })
}

// A failed undo keeps its step, so u can try again.
func TestConfig_FailedUndoKeepsTheStep(t *testing.T) {
	m, _ := openConfigIn(t, 80, 24, "")
	m = focusField(t, m, "switch.dirty_handling")
	m = sendConfigKeys(t, m, "l")
	failWrites(t, errors.New("disk full"))
	m = sendConfigKeys(t, m, "u")
	if len(m.configState.undo) != 1 || m.configState.focused().Value != "ignore" {
		t.Errorf("after a failed undo: %d steps, value %q; want 1, ignore", len(m.configState.undo), m.configState.focused().Value)
	}
}

func TestConfig_EscClosesAtOnceAfterChanges(t *testing.T) {
	m := openConfigWith(t, 80, 24, nil)
	m = focusField(t, m, "switch.dirty_handling")
	m = sendConfigKeys(t, m, "l", "esc")
	if m.activeView() != ViewDashboard || m.configState != nil {
		t.Errorf("esc: view %v, want closed with no prompt", m.activeView())
	}
}

// enter edits a text field in place; enter again saves, and keys type.
func TestConfig_TextEditCommits(t *testing.T) {
	m, path := openConfigOver(t, 80, 24, "", 1)
	m = focusField(t, m, "project_name")
	m = sendConfigKeys(t, m, "enter")
	if !m.configState.editing {
		t.Fatal("enter on a text field should start editing")
	}
	// Each of these would move, change section, undo, or open help.
	m = typeConfig(t, m, "jk1u?hl q")
	if m.activeView() != ViewConfig || m.configState.Tab != ConfigTabGeneral || focusedKey(m) != "project_name" {
		t.Fatalf("typing navigated: view %v, section %s, focus %s", m.activeView(), tabName(m.configState.Tab), focusedKey(m))
	}
	if text := readConfigFile(t, path); text != "" {
		t.Errorf("typing should write nothing before enter:\n%s", text)
	}
	m = sendConfigKeys(t, m, "enter")
	if m.configState.editing {
		t.Error("enter should finish the edit")
	}
	if got := m.configState.focused().Value; got != "jk1u?hl q" {
		t.Errorf("value %q, want what was typed", got)
	}
	if !strings.Contains(readConfigFile(t, path), `project_name = "jk1u?hl q"`) {
		t.Errorf("file:\n%s", readConfigFile(t, path))
	}
}

// esc during an edit cancels only the edit.
func TestConfig_TextEditCancels(t *testing.T) {
	m, path := openConfigIn(t, 80, 24, "")
	m = focusField(t, m, "default_base_branch")
	m = sendConfigKeys(t, m, "enter")
	m = typeConfig(t, m, "-next")
	m = sendConfigKeys(t, m, "esc")
	if m.activeView() != ViewConfig || m.configState.editing {
		t.Fatalf("esc during an edit: view %v, editing %v; want the panel, not editing", m.activeView(), m.configState.editing)
	}
	if got := m.configState.focused().Value; got != "main" {
		t.Errorf("value %q, want main", got)
	}
	if text := readConfigFile(t, path); text != "" {
		t.Errorf("a canceled edit wrote:\n%s", text)
	}
	if m = sendConfigKeys(t, m, "esc"); m.activeView() != ViewDashboard {
		t.Errorf("a second esc should close: view %v", m.activeView())
	}
}

// An invalid value is never written: the edit stays open with the reason,
// which goes away once you type again.
func TestConfig_TextEditValidates(t *testing.T) {
	cases := []struct {
		key, typed, want string
	}{
		{"naming.pattern", "{name}", "{project} exactly once"},
		{"naming.pattern", "{project}/{name}", "literal characters"},
		{"default_base_branch", "", "cannot be empty"},
	}
	for _, c := range cases {
		t.Run(c.key+" "+c.typed, func(t *testing.T) {
			m, path := openConfigIn(t, 80, 24, "")
			m = focusField(t, m, c.key)
			m = sendConfigKeys(t, m, "enter", "ctrl+u")
			m = typeConfig(t, m, c.typed)
			m = sendConfigKeys(t, m, "enter")
			if !m.configState.editing {
				t.Fatal("an invalid value should keep the edit open")
			}
			if text := readConfigFile(t, path); text != "" {
				t.Errorf("an invalid value was written:\n%s", text)
			}
			if view := stripAnsi(m.viewString()); !strings.Contains(view, c.want) {
				t.Errorf("the reason %q should show:\n%s", c.want, view)
			}
			m = typeConfig(t, m, "x")
			if view := stripAnsi(m.viewString()); strings.Contains(view, c.want) {
				t.Errorf("typing again should clear the reason:\n%s", view)
			}
		})
	}
}

// enter with the value unchanged ends the edit and writes nothing.
func TestConfig_TextEditUnchangedWritesNothing(t *testing.T) {
	m, path := openConfigIn(t, 80, 24, "")
	m = focusField(t, m, "naming.pattern")
	m = sendConfigKeys(t, m, "enter", "enter")
	if m.configState.editing || readConfigFile(t, path) != "" || len(m.configState.undo) != 0 {
		t.Errorf("editing %v, file %q, %d undo steps; want none", m.configState.editing, readConfigFile(t, path), len(m.configState.undo))
	}
}

// A list is edited as names separated by commas and written as a TOML
// array that loads back.
func TestConfig_ListEditWritesAnArray(t *testing.T) {
	m, path := openConfigIn(t, 80, 24, "")
	m = focusField(t, m, "protection.protected")
	m = sendConfigKeys(t, m, "enter")
	m = typeConfig(t, m, " main,staging , ")
	m = sendConfigKeys(t, m, "enter")
	if got := m.configState.focused().Value; got != "main, staging" {
		t.Errorf("value %q, want main, staging", got)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("file no longer loads: %v\n%s", err, readConfigFile(t, path))
	}
	if !slices.Equal(cfg.Protection.Protected, []string{"main", "staging"}) {
		t.Errorf("protected %v:\n%s", cfg.Protection.Protected, readConfigFile(t, path))
	}
}

// A text edit can be undone like any change.
func TestConfig_UndoATextEdit(t *testing.T) {
	m, path := openConfigIn(t, 80, 24, "")
	m = focusField(t, m, "default_base_branch")
	m = sendConfigKeys(t, m, "enter", "ctrl+u")
	m = typeConfig(t, m, "develop")
	m = sendConfigKeys(t, m, "enter", "u")
	if got := m.configState.focused().Value; got != "main" {
		t.Errorf("after undo: %q, want main", got)
	}
	if !strings.Contains(readConfigFile(t, path), `default_base_branch = "main"`) {
		t.Errorf("file:\n%s", readConfigFile(t, path))
	}
}

// ? opens help, except while typing.
func TestConfig_QuestionMarkOpensHelp(t *testing.T) {
	m := openConfigWith(t, 80, 24, nil)
	if m = sendConfigKeys(t, m, "?"); m.activeView() != ViewHelp {
		t.Errorf("? : view %v, want help", m.activeView())
	}
}

func TestConfig_SectionKeys(t *testing.T) {
	m := openConfigWith(t, 80, 24, nil)
	m = sendConfigKeys(t, m, "L")
	if m.configState.Tab != ConfigTabBehavior {
		t.Errorf("L: section %s, want Behavior", tabName(m.configState.Tab))
	}
	m = sendConfigKeys(t, m, "H", "H")
	if m.configState.Tab != ConfigTabProtection {
		t.Errorf("H twice from Behavior: section %s, want Protection", tabName(m.configState.Tab))
	}
	m = sendConfigKeys(t, m, "L")
	if m.configState.Tab != ConfigTabGeneral {
		t.Errorf("L from Protection: section %s, want General", tabName(m.configState.Tab))
	}
	for k, want := range map[string]ConfigTab{"3": ConfigTabPlugins, "4": ConfigTabProtection, "2": ConfigTabBehavior, "1": ConfigTabGeneral} {
		if m = sendConfigKeys(t, m, k); m.configState.Tab != want {
			t.Errorf("%s: section %s, want %s", k, tabName(m.configState.Tab), tabName(want))
		}
	}
}

// Each section keeps its own cursor, so coming back lands on the field you
// left.
func TestConfig_SectionsKeepTheirCursor(t *testing.T) {
	m := openConfigWith(t, 80, 24, nil)
	m = sendConfigKeys(t, m, "2", "down", "down")
	m = sendConfigKeys(t, m, "3")
	if focusedKey(m) != "plugins.docker.enabled" {
		t.Errorf("a section not visited yet should start at its first field, got %s", focusedKey(m))
	}
	m = sendConfigKeys(t, m, "H")
	if focusedKey(m) != m.configState.Fields[ConfigTabBehavior][2].Key {
		t.Errorf("back in Behavior: focus %s, want its third field", focusedKey(m))
	}
}

// The undo step and the shown value use the value the panel shows, so an
// option grove fills in when the file says nothing is undone to that.
func TestPopulateConfigFields_Values(t *testing.T) {
	cfg := config.LoadDefaults()
	cfg.Switch.ContainerSwitch = ""
	cfg.Tmux.Mode = ""
	cfg.Session.OpenIn = "current"
	cfg.Protection.Protected = []string{"main", "staging"}
	off := false
	cfg.Plugins.Docker.Enabled = &off

	want := map[string]string{
		"switch.container_switch":  "off",
		"tmux.mode":                "auto",
		"session.open_in":          "here",
		"protection.protected":     "main, staging",
		"protection.immutable":     "",
		"plugins.docker.enabled":   "false",
		"plugins.docker.auto_stop": "false",
	}
	for _, fields := range populateConfigFields(cfg) {
		for _, f := range fields {
			if w, ok := want[f.Key]; ok && f.Value != w {
				t.Errorf("%s = %q, want %q", f.Key, f.Value, w)
			}
		}
	}
}

func TestTabName(t *testing.T) {
	names := map[ConfigTab]string{
		ConfigTabGeneral:    "General",
		ConfigTabBehavior:   "Behavior",
		ConfigTabPlugins:    "Plugins",
		ConfigTabProtection: "Protection",
	}
	for tab, expected := range names {
		if got := tabName(tab); got != expected {
			t.Errorf("tabName(%d) = %q, want %q", tab, got, expected)
		}
	}
}

// On a text field that is not being edited, the section keys work as on
// any other field.
func TestConfig_SectionKeysWorkOnATextField(t *testing.T) {
	m := openConfigWith(t, 80, 24, nil)
	if f := m.configState.focused(); f.Type != ConfigString {
		t.Fatalf("General should open on a text field, got %s", f.Key)
	}
	steps := []struct {
		key  string
		want ConfigTab
	}{{"2", ConfigTabBehavior}, {"1", ConfigTabGeneral}, {"L", ConfigTabBehavior}, {"H", ConfigTabGeneral}, {"4", ConfigTabProtection}}
	for _, s := range steps {
		if m = sendConfigKeys(t, m, s.key); m.configState.Tab != s.want {
			t.Errorf("%s on a text field: section %s, want %s", s.key, tabName(m.configState.Tab), tabName(s.want))
		}
	}
}

// While a field is being typed in, a digit types and the tabs stop
// offering their number keys.
func TestConfig_DigitMidEditTypes(t *testing.T) {
	m := openConfigWith(t, 80, 24, nil)
	m = focusField(t, m, "default_base_branch")
	m = sendConfigKeys(t, m, "enter", "ctrl+u")
	m = typeConfig(t, m, "3")
	if m.configState.Tab != ConfigTabGeneral || m.configState.input.Value() != "3" {
		t.Errorf("3 mid-edit: section %s, typed %q; want General and 3", tabName(m.configState.Tab), m.configState.input.Value())
	}
	m = sendConfigKeys(t, m, "ctrl+n")
	if m.configState.Tab != ConfigTabGeneral || !m.configState.editing {
		t.Errorf("ctrl+n mid-edit: section %s, editing %v; want the edit kept", tabName(m.configState.Tab), m.configState.editing)
	}
	tabs := strings.Split(stripAnsi(m.viewString()), "\n")[1]
	if strings.Contains(tabs, "[") || !strings.Contains(tabs, "general") {
		t.Errorf("tabs mid-edit should drop their numbers: %q", tabs)
	}
}

// Pasted text lands in the field being edited.
func TestConfig_PasteIntoAnEdit(t *testing.T) {
	m := openConfigWith(t, 80, 24, nil)
	m = focusField(t, m, "default_base_branch")
	m = sendConfigKeys(t, m, "enter", "ctrl+u")
	m = sendConfigMsg(t, m, tea.PasteMsg{Content: "develop"})
	if got := m.configState.input.Value(); got != "develop" {
		t.Errorf("after paste: %q, want develop", got)
	}
}

// recordWrites counts the config writes until the test ends, passing each
// through to the real writer.
func recordWrites(t *testing.T) *[]map[string]string {
	t.Helper()
	var writes []map[string]string
	write := saveProjectConfig
	saveProjectConfig = func(updates map[string]string) error {
		writes = append(writes, updates)
		return write(updates)
	}
	t.Cleanup(func() { saveProjectConfig = write })
	return &writes
}

// The two fields that name worktree folders, with a value that passes
// validation and differs from the one in the file.
var renamingFields = []struct {
	key, file, old, typed, toml string
}{
	{"project_name", "project_name = \"myapp\"\n", "myapp", "shop", `"shop"`},
	{"naming.pattern", "[naming]\npattern = \"{project}-{name}\"\n", "{project}-{name}", "{project}.{name}", `"{project}.{name}"`},
}

// editTo opens the focused field, replaces its text with typed, and presses
// enter once.
func editTo(t *testing.T, m Model, typed string) Model {
	t.Helper()
	m = sendConfigKeys(t, m, "enter", "ctrl+u")
	m = typeConfig(t, m, typed)
	return sendConfigKeys(t, m, "enter")
}

// With other worktrees, the first enter on a changed project name or folder
// pattern writes nothing and asks again; the second writes that one key once.
func TestConfig_RenamingFieldAsksBeforeWriting(t *testing.T) {
	for _, c := range renamingFields {
		t.Run(c.key, func(t *testing.T) {
			m, path := openConfigIn(t, 80, 24, c.file)
			writes := recordWrites(t)
			m = focusField(t, m, c.key)
			m = editTo(t, m, c.typed)
			if !m.configState.editing || !m.configState.confirming {
				t.Fatalf("first enter: editing %v, confirming %v; want both", m.configState.editing, m.configState.confirming)
			}
			if len(*writes) != 0 || readConfigFile(t, path) != c.file {
				t.Fatalf("first enter wrote %v:\n%s", *writes, readConfigFile(t, path))
			}
			if got := m.configState.focused().Value; got != c.old {
				t.Errorf("first enter: value %q, want %q", got, c.old)
			}

			m = sendConfigKeys(t, m, "enter")
			if m.configState.editing || m.configState.confirming {
				t.Errorf("second enter: editing %v, confirming %v; want neither", m.configState.editing, m.configState.confirming)
			}
			if len(*writes) != 1 || len((*writes)[0]) != 1 || (*writes)[0][c.key] != c.toml {
				t.Fatalf("second enter: writes %v, want one of %s = %s", *writes, c.key, c.toml)
			}
			if got := m.configState.focused().Value; got != c.typed {
				t.Errorf("second enter: value %q, want %q", got, c.typed)
			}
			if h := configHeaderLine(m); !strings.Contains(h, "✓ saved") || !strings.Contains(h, "u undo") {
				t.Errorf("header after the write: %q, want saved and u undo", h)
			}

			m = sendConfigKeys(t, m, "u")
			if m.configState.confirming || m.configState.editing || len(*writes) != 2 {
				t.Errorf("undo should write at once: confirming %v, editing %v, %d writes", m.configState.confirming, m.configState.editing, len(*writes))
			}
			if got := m.configState.focused().Value; got != c.old {
				t.Errorf("after undo: value %q, want %q", got, c.old)
			}
		})
	}
}

// esc from the confirm step goes back to the edit with the typed text; a
// second esc drops the edit. Nothing is written.
func TestConfig_RenamingFieldEscKeepsTheTypedText(t *testing.T) {
	for _, c := range renamingFields {
		t.Run(c.key, func(t *testing.T) {
			m, path := openConfigIn(t, 80, 24, c.file)
			m = editTo(t, focusField(t, m, c.key), c.typed)
			m = sendConfigKeys(t, m, "esc")
			s := m.configState
			if !s.editing || s.confirming || s.input.Value() != c.typed {
				t.Fatalf("esc from confirm: editing %v, confirming %v, text %q; want the edit with %q", s.editing, s.confirming, s.input.Value(), c.typed)
			}
			m = sendConfigKeys(t, m, "esc")
			if m.configState.editing || m.activeView() != ViewConfig {
				t.Errorf("second esc: editing %v, view %v; want the panel, not editing", m.configState.editing, m.activeView())
			}
			if readConfigFile(t, path) != c.file || m.configState.focused().Value != c.old {
				t.Errorf("esc wrote: value %q, file:\n%s", m.configState.focused().Value, readConfigFile(t, path))
			}
		})
	}
}

// Any other key in the confirm step goes back to the edit and types.
func TestConfig_RenamingFieldTypingReturnsToTheEdit(t *testing.T) {
	m, path := openConfigIn(t, 80, 24, renamingFields[0].file)
	m = editTo(t, focusField(t, m, "project_name"), "shop")
	m = typeConfig(t, m, "s")
	s := m.configState
	if !s.editing || s.confirming || s.input.Value() != "shops" {
		t.Fatalf("typing from confirm: editing %v, confirming %v, text %q; want the edit with shops", s.editing, s.confirming, s.input.Value())
	}
	if readConfigFile(t, path) != renamingFields[0].file {
		t.Errorf("typing wrote:\n%s", readConfigFile(t, path))
	}
	if m = sendConfigKeys(t, m, "enter"); !m.configState.confirming {
		t.Error("enter on the new text should ask again")
	}
}

// An invalid value shows its reason and never reaches the confirm step; an
// unchanged one ends the edit without asking.
func TestConfig_RenamingFieldChecksFirst(t *testing.T) {
	m, path := openConfigIn(t, 80, 24, renamingFields[1].file)
	m = editTo(t, focusField(t, m, "naming.pattern"), "{name}")
	if m.configState.confirming || m.configState.editErr == "" {
		t.Errorf("invalid value: confirming %v, reason %q; want the reason only", m.configState.confirming, m.configState.editErr)
	}
	m = sendConfigKeys(t, m, "esc")

	m = sendConfigKeys(t, m, "enter", "enter")
	if m.configState.confirming || m.configState.editing {
		t.Errorf("unchanged value: confirming %v, editing %v; want neither", m.configState.confirming, m.configState.editing)
	}
	if readConfigFile(t, path) != renamingFields[1].file {
		t.Errorf("nothing should be written:\n%s", readConfigFile(t, path))
	}
}

// With no worktree besides root, nothing can lose its name, so one enter
// writes.
func TestConfig_RenamingFieldWritesAtOnceWithoutOtherWorktrees(t *testing.T) {
	for _, c := range renamingFields {
		t.Run(c.key, func(t *testing.T) {
			m, path := openConfigOver(t, 80, 24, c.file, 1)
			m = editTo(t, focusField(t, m, c.key), c.typed)
			if m.configState.confirming || m.configState.editing {
				t.Errorf("confirming %v, editing %v; want the write done", m.configState.confirming, m.configState.editing)
			}
			if !strings.Contains(readConfigFile(t, path), c.toml) {
				t.Errorf("file:\n%s", readConfigFile(t, path))
			}
		})
	}
}

// Every other text field writes on the first enter, other worktrees or not.
func TestConfig_OtherTextFieldsWriteAtOnce(t *testing.T) {
	m, path := openConfigIn(t, 80, 24, "")
	m = editTo(t, focusField(t, m, "projects_dir"), "~/src")
	if m.configState.confirming || !strings.Contains(readConfigFile(t, path), `projects_dir = "~/src"`) {
		t.Errorf("confirming %v, file:\n%s", m.configState.confirming, readConfigFile(t, path))
	}
}

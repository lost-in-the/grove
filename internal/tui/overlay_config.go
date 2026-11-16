package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/tuilog"
)

// ConfigTab identifies which section of the settings panel is shown.
type ConfigTab int

const (
	ConfigTabGeneral ConfigTab = iota
	ConfigTabBehavior
	ConfigTabPlugins
	ConfigTabProtection
	ConfigTabCount // sentinel for tab wrapping
)

// ConfigFieldType describes the type of a config field.
type ConfigFieldType int

const (
	ConfigString ConfigFieldType = iota
	ConfigBool
	ConfigEnum
	ConfigList
)

// configUndoLimit is how many changes u can step back through.
const configUndoLimit = 20

// configChange is one write u can take back: the field and the value it
// had before.
type configChange struct {
	tab   ConfigTab
	index int
	prev  string
}

// ConfigState is the settings panel: a list of fields per section, the
// focused one explained below it. Every change is written at once, so there
// is nothing unsaved.
type ConfigState struct {
	Tab    ConfigTab
	Fields [][]ConfigField
	Err    error          // the config did not load
	Config *config.Config // the loaded config, kept current with each write

	cursor [ConfigTabCount]int

	editing bool // a text field's value is being typed
	input   textinput.Model
	editErr string // why the typed value was not written

	saveErr    error  // the last write failed
	saveErrKey string // the field it failed for
	flash      string // "saved", "undone", "nothing to undo"
	flashGen   int    // matches the timer that clears flash

	undo []configChange
}

// NewConfigState creates an empty ConfigState.
func NewConfigState() *ConfigState {
	return &ConfigState{
		Tab:    ConfigTabGeneral,
		Fields: make([][]ConfigField, ConfigTabCount),
	}
}

// load fills the panel from cfg.
func (s *ConfigState) load(cfg *config.Config) {
	s.Config = cfg
	s.Fields = populateConfigFields(cfg)
	s.cursor = [ConfigTabCount]int{}
	s.editing = false
}

// focused is the field under the cursor, or nil before the config loads.
func (s *ConfigState) focused() *ConfigField {
	fields := s.Fields[s.Tab]
	if len(fields) == 0 {
		return nil
	}
	return &fields[s.cursor[s.Tab]]
}

// configLoadedMsg is sent after loading config.
type configLoadedMsg struct {
	cfg *config.Config
	err error
}

// configFlashDoneMsg clears the header note gen set, unless a later write
// replaced it.
type configFlashDoneMsg struct{ gen int }

// loadConfigCmd loads the configuration.
func loadConfigCmd() tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load()
		return configLoadedMsg{cfg: cfg, err: err}
	}
}

// saveProjectConfig writes keys to the project's .grove/config.toml, leaving
// the rest of the file as it was. Tests point it at a temp project.
var saveProjectConfig = config.SetProjectConfigValues

// tomlString quotes s as a TOML basic string. strconv.Quote is close, but
// its \a, \v, and \x escapes are not TOML: other control characters get
// \uXXXX instead.
func tomlString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\b':
			b.WriteString(`\b`)
		case '\t':
			b.WriteString(`\t`)
		case '\n':
			b.WriteString(`\n`)
		case '\f':
			b.WriteString(`\f`)
		case '\r':
			b.WriteString(`\r`)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, `\u%04X`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// tomlList turns a comma-separated list into a TOML array of strings,
// dropping empty items.
func tomlList(list string) string {
	items := splitList(list)
	for i, item := range items {
		items[i] = tomlString(item)
	}
	return "[" + strings.Join(items, ", ") + "]"
}

// tabName returns the display name for a config tab.
func tabName(tab ConfigTab) string {
	switch tab {
	case ConfigTabGeneral:
		return "General"
	case ConfigTabBehavior:
		return "Behavior"
	case ConfigTabPlugins:
		return "Plugins"
	case ConfigTabProtection:
		return "Protection"
	default:
		return ""
	}
}

// handleConfigLoaded fills the panel, or keeps the load error to show.
func (m Model) handleConfigLoaded(msg configLoadedMsg) (tea.Model, tea.Cmd) {
	if m.configState == nil {
		return m, nil
	}
	if msg.err != nil {
		m.configState.Err = msg.err
		return m, nil
	}
	m.configState.Err = nil
	m.configState.load(msg.cfg)
	return m, nil
}

func (m Model) handleConfigFlashDone(msg configFlashDoneMsg) (tea.Model, tea.Cmd) {
	if s := m.configState; s != nil && msg.gen == s.flashGen {
		s.flash = ""
	}
	return m, nil
}

// handleConfigKey handles key input for the settings panel.
func (m Model) handleConfigKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := m.configState
	if s == nil {
		m.popView()
		return m, nil
	}
	if s.editing {
		return m.handleConfigEditKey(msg)
	}
	if key.Matches(msg, m.keys.Escape) {
		m.popView()
		m.configState = nil
		return m, nil
	}
	f := s.focused()
	if f == nil {
		return m, nil
	}
	n := len(s.Fields[s.Tab])
	switch k := msg.String(); k {
	case "ctrl+n":
		s.Tab = (s.Tab + 1) % ConfigTabCount
	case "ctrl+p":
		s.Tab = (s.Tab + ConfigTabCount - 1) % ConfigTabCount
	case "1", "2", "3", "4":
		s.Tab = ConfigTab(k[0] - '1')
	case "down", "j":
		s.cursor[s.Tab] = min(n-1, s.cursor[s.Tab]+1)
	case "up", "k":
		s.cursor[s.Tab] = max(0, s.cursor[s.Tab]-1)
	case "tab":
		s.cursor[s.Tab] = (s.cursor[s.Tab] + 1) % n
	case "shift+tab":
		s.cursor[s.Tab] = (s.cursor[s.Tab] + n - 1) % n
	case "right", "l":
		return m, m.stepConfigChoice(1)
	case "left", "h":
		return m, m.stepConfigChoice(-1)
	case "space":
		if f.Type == ConfigBool {
			return m, m.stepConfigChoice(1)
		}
	case "enter":
		if f.Type == ConfigString || f.Type == ConfigList {
			return m, s.startEdit()
		}
	case "u":
		return m, m.undoConfigChange()
	}
	return m, nil
}

// stepConfigChoice moves a choice or true/false field by delta options,
// wrapping, and writes it.
func (m *Model) stepConfigChoice(delta int) tea.Cmd {
	s := m.configState
	f := s.focused()
	if f.Type != ConfigEnum && f.Type != ConfigBool {
		return nil
	}
	cur := 0
	for i, o := range f.Options {
		if o.Value == f.Value {
			cur = i
		}
	}
	n := len(f.Options)
	next := f.Options[(cur+delta+n)%n].Value
	return m.changeConfigField(s.Tab, s.cursor[s.Tab], next, "saved")
}

// changeConfigField writes value to one field and, when that works,
// records the old value for undo.
func (m *Model) changeConfigField(tab ConfigTab, index int, value, note string) tea.Cmd {
	s := m.configState
	prev := s.Fields[tab][index].Value
	if err := m.writeConfigField(tab, index, value); err != nil {
		return nil
	}
	s.undo = append(s.undo, configChange{tab: tab, index: index, prev: prev})
	if len(s.undo) > configUndoLimit {
		s.undo = s.undo[1:]
	}
	return s.setFlash(note)
}

// writeConfigField validates value as config loading would, writes that one
// key, and only then shows it. Like the pins, the write happens here rather
// than in a command, so writes land in the order they were made.
func (m *Model) writeConfigField(tab ConfigTab, index int, value string) error {
	s := m.configState
	f := &s.Fields[tab][index]
	if err := f.validate(s.Config, value); err != nil {
		return err
	}
	if err := saveProjectConfig(map[string]string{f.Key: f.tomlValue(value)}); err != nil {
		s.saveErr, s.saveErrKey = err, f.Key
		s.flash = ""
		return err
	}
	s.saveErr, s.saveErrKey = nil, ""
	f.Value = value
	if s.Config != nil && f.set != nil {
		next := *s.Config
		f.set(&next, value)
		s.Config = &next
	}
	m.reloadConfig()
	return nil
}

// reloadConfig picks up the written file for the rest of the TUI, such as
// the row layout.
func (m *Model) reloadConfig() {
	cfg, err := config.Load()
	if err != nil {
		tuilog.Printf("error: config reload after save failed: %v", err)
		return
	}
	m.cfg = cfg
	m.refreshDelegate()
}

// undoConfigChange writes back the value before the newest change and
// moves to its field. A failed write keeps the step.
func (m *Model) undoConfigChange() tea.Cmd {
	s := m.configState
	if len(s.undo) == 0 {
		return s.setFlash("nothing to undo")
	}
	c := s.undo[len(s.undo)-1]
	s.Tab, s.cursor[c.tab] = c.tab, c.index
	if err := m.writeConfigField(c.tab, c.index, c.prev); err != nil {
		return nil
	}
	s.undo = s.undo[:len(s.undo)-1]
	return s.setFlash("undone")
}

// setFlash shows note in the header until the toast timeout passes.
func (s *ConfigState) setFlash(note string) tea.Cmd {
	s.flashGen++
	s.flash = note
	gen := s.flashGen
	return tea.Tick(DefaultToastDuration, func(time.Time) tea.Msg { return configFlashDoneMsg{gen: gen} })
}

// startEdit opens the focused text field for typing, holding its value.
func (s *ConfigState) startEdit() tea.Cmd {
	in := textinput.New()
	in.Prompt = "▏"
	in.CharLimit = 500
	in.SetValue(s.focused().Value)
	in.CursorEnd()
	s.input, s.editing, s.editErr = in, true, ""
	return s.input.Focus()
}

// handleConfigEditKey sends keys to the field being typed in: enter writes
// the value, esc drops the edit, and every other key types.
func (m Model) handleConfigEditKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := m.configState
	switch {
	case key.Matches(msg, m.keys.Escape):
		s.editing, s.editErr = false, ""
		return m, nil
	case key.Matches(msg, m.keys.Enter):
		return m, m.commitConfigEdit()
	}
	s.editErr = ""
	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)
	return m, cmd
}

// commitConfigEdit writes the typed value. An invalid one, or a failed
// write, keeps the edit open with the reason.
func (m *Model) commitConfigEdit() tea.Cmd {
	s := m.configState
	f := s.focused()
	value := strings.TrimSpace(s.input.Value())
	if f.Type == ConfigList {
		value = strings.Join(splitList(value), ", ")
	}
	if value == f.Value {
		s.editing = false
		return nil
	}
	if err := f.validate(s.Config, value); err != nil {
		s.editErr = err.Error()
		return nil
	}
	cmd := m.changeConfigField(s.Tab, s.cursor[s.Tab], value, "saved")
	if s.saveErr != nil && s.saveErrKey == f.Key {
		return nil
	}
	s.editing = false
	return cmd
}

// handleConfigInputMsg passes the cursor blink and pasted text to the field
// being typed in.
func (m Model) handleConfigInputMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	s := m.configState
	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)
	return m, cmd
}

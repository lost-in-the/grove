package tui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/lost-in-the/grove/internal/config"
)

// ConfigTab identifies which tab is active in the config overlay.
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

// ConfigField represents a single editable config field.
type ConfigField struct {
	Key         string // TOML key path, e.g. "tmux.mode"
	Label       string // display name
	Value       string // current value as string
	Loaded      string // value loaded from disk; Value differs when changed
	Type        ConfigFieldType
	Options     []string // for Enum type
	Description string   // help text
	Placeholder string   // shown when value is empty (defaults to "(empty)")
}

// ConfigState holds the state for the config editor: one huh form per
// section, of which grove shows the active one (spec 11).
type ConfigState struct {
	Tab        ConfigTab       // active section
	Fields     [][]ConfigField // fields per section; the forms bind to their values
	Err        error
	Confirming bool           // save confirmation prompt active
	Config     *config.Config // loaded config

	Forms      [ConfigTabCount]*huh.Form // nil until the config loads
	FormValues *configFormValues         // bool bindings shared by the forms
	formW      int                       // size the forms are built at
	formH      int
}

// form is the active section's form, or nil before the config loads.
func (s *ConfigState) form() *huh.Form { return s.Forms[s.Tab] }

// focusOnInput reports whether a text input has focus, so digits and ?
// must type rather than act.
func (s *ConfigState) focusOnInput() bool {
	if s.form() == nil {
		return false
	}
	_, ok := s.form().GetFocusedField().(*huh.Input)
	return ok
}

// unsaved counts the fields whose value differs from the one loaded from
// disk. Call syncConfigFormValues first so bools are current.
func (s *ConfigState) unsaved() int {
	n := 0
	for _, fields := range s.Fields {
		for _, f := range fields {
			if f.Value != f.Loaded {
				n++
			}
		}
	}
	return n
}

// NewConfigState creates an empty ConfigState.
func NewConfigState() *ConfigState {
	return &ConfigState{
		Tab:    ConfigTabGeneral,
		Fields: make([][]ConfigField, ConfigTabCount),
	}
}

// configLoadedMsg is sent after loading config.
type configLoadedMsg struct {
	cfg *config.Config
	err error
}

// configSavedMsg is sent after saving config.
type configSavedMsg struct {
	err error
}

// loadConfigCmd loads the configuration.
func loadConfigCmd() tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load()
		return configLoadedMsg{cfg: cfg, err: err}
	}
}

// configUpdates is the TOML value of every changed field, keyed by its
// config key.
func configUpdates(state *ConfigState) map[string]string {
	updates := make(map[string]string)
	for _, tabFields := range state.Fields {
		for _, field := range tabFields {
			if field.Value == field.Loaded {
				continue
			}
			val := field.Value
			// Quote string values for TOML with proper escaping
			switch field.Type {
			case ConfigString, ConfigEnum:
				val = strconv.Quote(val)
			case ConfigList:
				val = tomlList(val)
			}
			updates[field.Key] = val
		}
	}
	return updates
}

// tomlList turns a comma-separated list into a TOML array of strings,
// dropping empty items.
func tomlList(list string) string {
	var items []string
	for _, item := range strings.Split(list, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, strconv.Quote(item))
		}
	}
	return "[" + strings.Join(items, ", ") + "]"
}

// saveConfigCmd saves the changed fields. The updates are worked out here,
// not in the command, which runs on another goroutine.
func saveConfigCmd(state *ConfigState) tea.Cmd {
	updates := configUpdates(state)
	return func() tea.Msg {
		if len(updates) == 0 {
			return configSavedMsg{}
		}
		return configSavedMsg{err: config.SetProjectConfigValues(updates)}
	}
}

// populateConfigFields maps a Config struct to field arrays per tab.
func populateConfigFields(cfg *config.Config) [][]ConfigField {
	fields := make([][]ConfigField, ConfigTabCount)

	// General tab
	fields[ConfigTabGeneral] = []ConfigField{
		{
			Key:         "project_name",
			Label:       "project_name",
			Value:       cfg.ProjectName,
			Loaded:      cfg.ProjectName,
			Type:        ConfigString,
			Description: "Project name",
		},
		{
			Key:         "projects_dir",
			Label:       "projects_dir",
			Value:       cfg.ProjectsDir,
			Loaded:      cfg.ProjectsDir,
			Type:        ConfigString,
			Description: "Where worktrees are created",
		},
		{
			Key:         "default_base_branch",
			Label:       "default_branch",
			Value:       cfg.DefaultBranch,
			Loaded:      cfg.DefaultBranch,
			Type:        ConfigString,
			Description: "Base branch for new worktrees",
		},
	}

	// Behavior tab
	tmuxMode := cfg.Tmux.Mode
	if tmuxMode == "" {
		tmuxMode = "auto"
	}
	fields[ConfigTabBehavior] = []ConfigField{
		{
			Key:         "switch.dirty_handling",
			Label:       "dirty_handling",
			Value:       cfg.Switch.DirtyHandling,
			Loaded:      cfg.Switch.DirtyHandling,
			Type:        ConfigEnum,
			Options:     []string{"auto-stash", "prompt", "refuse"},
			Description: "How to handle dirty worktree on switch",
		},
		{
			Key:         "tmux.mode",
			Label:       "tmux_mode",
			Value:       tmuxMode,
			Loaded:      tmuxMode,
			Type:        ConfigEnum,
			Options:     []string{"auto", "manual", "off"},
			Description: "Tmux session behavior: auto-attach, print instructions, or skip",
		},
		{
			Key:         "naming.pattern",
			Label:       "naming_pattern",
			Value:       cfg.Naming.Pattern,
			Loaded:      cfg.Naming.Pattern,
			Type:        ConfigString,
			Description: "Worktree directory template; {project} and {name} each exactly once",
		},
	}

	// Plugins tab
	dockerEnabled := boolTrue
	dockerAutoStart := boolTrue
	dockerAutoStop := boolFalse
	if cfg.Plugins.Docker.Enabled != nil {
		dockerEnabled = fmt.Sprintf("%v", *cfg.Plugins.Docker.Enabled)
	}
	if cfg.Plugins.Docker.AutoStart != nil {
		dockerAutoStart = fmt.Sprintf("%v", *cfg.Plugins.Docker.AutoStart)
	}
	if cfg.Plugins.Docker.AutoStop != nil {
		dockerAutoStop = fmt.Sprintf("%v", *cfg.Plugins.Docker.AutoStop)
	}
	fields[ConfigTabPlugins] = []ConfigField{
		{
			Key:         "plugins.docker.enabled",
			Label:       "docker_enabled",
			Value:       dockerEnabled,
			Loaded:      dockerEnabled,
			Type:        ConfigBool,
			Description: "Enable Docker plugin",
		},
		{
			Key:         "plugins.docker.auto_start",
			Label:       "docker_auto_start",
			Value:       dockerAutoStart,
			Loaded:      dockerAutoStart,
			Type:        ConfigBool,
			Description: "Auto-start containers on switch",
		},
		{
			Key:         "plugins.docker.auto_stop",
			Label:       "docker_auto_stop",
			Value:       dockerAutoStop,
			Loaded:      dockerAutoStop,
			Type:        ConfigBool,
			Description: "Auto-stop containers on leave",
		},
	}

	// Protection tab
	fields[ConfigTabProtection] = []ConfigField{
		{
			Key:         "protection.protected",
			Label:       "protected",
			Value:       strings.Join(cfg.Protection.Protected, ", "),
			Loaded:      strings.Join(cfg.Protection.Protected, ", "),
			Type:        ConfigList,
			Description: "Protected worktrees (comma-separated)",
			Placeholder: "(none — add worktree names)",
		},
		{
			Key:         "protection.immutable",
			Label:       "immutable",
			Value:       strings.Join(cfg.Protection.Immutable, ", "),
			Loaded:      strings.Join(cfg.Protection.Immutable, ", "),
			Type:        ConfigList,
			Description: "Immutable worktrees (comma-separated)",
			Placeholder: "(none — add worktree names)",
		},
	}

	return fields
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

// handleConfigKey handles key input for the config editor.
func (m Model) handleConfigKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.configState == nil {
		m.popView()
		return m, nil
	}

	s := m.configState

	// Handle confirmation prompt keys
	if s.Confirming {
		switch {
		case key.Matches(msg, m.keys.Enter):
			// Save and close
			m.popView()
			m.configState = nil
			return m, saveConfigCmd(s)
		case key.Matches(msg, m.keys.Escape):
			// Discard and close
			m.popView()
			m.configState = nil
			return m, nil
		}
		return m, nil
	}

	if key.Matches(msg, m.keys.Escape) {
		return m.leaveConfig()
	}

	// Section keys (spec 11): ctrl+n / ctrl+p anywhere, before the form,
	// whose selects would take them as down and up; digits only off text
	// inputs, where they must type.
	//
	// Known gap: enter on a last field finishes the form through two
	// follow-up messages. A section key that lands between them sends the
	// rest to the new section's form, moving its focus. It needs two keys
	// in one event-loop turn, so it is left as is.
	switch k := msg.String(); {
	case k == "ctrl+n":
		return m, s.switchSection((s.Tab + 1) % ConfigTabCount)
	case k == "ctrl+p":
		return m, s.switchSection((s.Tab + ConfigTabCount - 1) % ConfigTabCount)
	case len(k) == 1 && k >= "1" && k <= "4" && !s.focusOnInput():
		return m, s.switchSection(ConfigTab(k[0] - '1'))
	}

	if s.form() != nil {
		return m.updateConfigForm(msg)
	}
	return m, nil
}

// leaveConfig closes the editor, first asking to save when anything
// changed. updateConfigForm syncs the bools after every form message, so
// the fields are already current.
func (m Model) leaveConfig() (tea.Model, tea.Cmd) {
	if m.configState.unsaved() > 0 {
		m.configState.Confirming = true
		return m, nil
	}
	m.popView()
	m.configState = nil
	return m, nil
}

// handleConfigFormMsg forwards non-key messages (focus moves, cursor blink)
// to the active section's form. huh finishes a form through one of these,
// so it goes through the same completion check as keys.
func (m Model) handleConfigFormMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.configState == nil || m.configState.form() == nil {
		return m, nil
	}
	return m.updateConfigForm(msg)
}

// updateConfigForm sends a message to the active form. Finishing a section
// (enter on its last field) moves to the next; finishing the last one
// leaves as esc does.
func (m Model) updateConfigForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	s := m.configState
	model, cmd := s.form().Update(msg)
	if f, ok := model.(*huh.Form); ok {
		s.Forms[s.Tab] = f
	}
	m.syncConfigFormValues() // keeps the unsaved count current

	switch s.form().State {
	case huh.StateCompleted:
		// A completed form ignores input; rebuild it so the section can be
		// revisited with its current values.
		initCmd := s.rebuildForm(s.Tab)
		if s.Tab == ConfigTabCount-1 {
			next, leaveCmd := m.leaveConfig()
			return next, tea.Batch(initCmd, leaveCmd)
		}
		return m, tea.Batch(initCmd, s.switchSection(s.Tab+1))
	case huh.StateAborted:
		m.popView()
		m.configState = nil
		return m, nil
	}
	return m, cmd
}

// switchSection makes tab the active section. Only the active form gets
// messages, so a hidden form's cursor blink stops; focusing the field
// arrived at again restarts it. huh 2.0.3 draws a focused input's cursor
// regardless, so this guards against a huh change rather than a visible bug.
func (s *ConfigState) switchSection(tab ConfigTab) tea.Cmd {
	s.Tab = tab
	if f := s.form(); f != nil {
		return f.GetFocusedField().Focus()
	}
	return nil
}

// buildForms builds every section's form at the editor's size.
func (s *ConfigState) buildForms(width, height int) tea.Cmd {
	s.formW, s.formH = width, height
	s.FormValues = newConfigFormValues()
	var cmds []tea.Cmd
	for tab := ConfigTab(0); tab < ConfigTabCount; tab++ {
		cmds = append(cmds, s.rebuildForm(tab))
	}
	return tea.Batch(cmds...)
}

// rebuildForm builds one section's form from the fields' current values.
func (s *ConfigState) rebuildForm(tab ConfigTab) tea.Cmd {
	s.Forms[tab] = buildSectionForm(s.Fields[tab], s.FormValues, s.formW, s.formH)
	return s.Forms[tab].Init()
}

// syncConfigFormValues reads the bool bindings back into their fields.
func (m *Model) syncConfigFormValues() {
	s := m.configState
	if s == nil || s.FormValues == nil {
		return
	}

	for tabIdx := range s.Fields {
		for fieldIdx := range s.Fields[tabIdx] {
			syncConfigField(&s.Fields[tabIdx][fieldIdx], s)
		}
	}
}

// syncConfigField updates a bool field from its binding. String, enum, and
// list fields bind straight to Value.
func syncConfigField(f *ConfigField, s *ConfigState) {
	if f.Type == ConfigBool {
		if bPtr, ok := s.FormValues.bools[f.Key]; ok {
			if *bPtr {
				f.Value = boolTrue
			} else {
				f.Value = boolFalse
			}
		}
	}
}

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
	Dirty      bool           // unsaved changes exist
	Confirming bool           // save confirmation prompt active
	Config     *config.Config // loaded config

	Forms      [ConfigTabCount]*huh.Form // nil until the config loads
	FormValues *configFormValues         // bool bindings shared by the forms
	formW      int                       // size the forms are built at
	formH      int
}

// form is the active section's form, or nil before the config loads.
func (s *ConfigState) form() *huh.Form { return s.Forms[s.Tab] }

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

// saveConfigCmd saves modified config fields.
func saveConfigCmd(state *ConfigState) tea.Cmd {
	return func() tea.Msg {
		updates := make(map[string]string)
		for _, tabFields := range state.Fields {
			for _, field := range tabFields {
				// Only save changed fields
				if field.Value != field.Loaded {
					val := field.Value
					// Quote string values for TOML with proper escaping
					if field.Type == ConfigString || field.Type == ConfigEnum {
						val = strconv.Quote(val)
					}
					updates[field.Key] = val
				}
			}
		}

		if len(updates) == 0 {
			return configSavedMsg{}
		}

		err := config.SetProjectConfigValues(updates)
		return configSavedMsg{err: err}
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

	if s.form() != nil {
		return m.updateConfigForm(msg)
	}
	return m, nil
}

// leaveConfig closes the editor, first asking to save when anything
// changed. Sync first: string fields bind live, bools only through their
// bindings.
func (m Model) leaveConfig() (tea.Model, tea.Cmd) {
	m.syncConfigFormValues()
	if m.configState.Dirty {
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

	switch s.form().State {
	case huh.StateCompleted:
		m.syncConfigFormValues()
		// A completed form ignores input; rebuild it so the section can be
		// revisited with its current values.
		initCmd := s.rebuildForm(s.Tab)
		if s.Tab == ConfigTabCount-1 {
			next, leaveCmd := m.leaveConfig()
			return next, tea.Batch(initCmd, leaveCmd)
		}
		s.Tab++
		return m, initCmd
	case huh.StateAborted:
		m.popView()
		m.configState = nil
		return m, nil
	}
	return m, cmd
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

// syncConfigFormValues reads values from form bindings back into ConfigFields
// and marks dirty if anything changed from the loaded value.
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

// syncConfigField updates a single config field from form values and marks
// the state dirty when the value differs from the loaded one.
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
	// String/Enum/List fields are bound directly via &f.Value

	if f.Value != f.Loaded {
		s.Dirty = true
	}
}

// renderConfig renders the config overlay.
func renderConfig(s *ConfigState, width int) string {
	overlayWidth := width * 60 / 100
	if overlayWidth < 60 {
		overlayWidth = 60
	}
	if overlayWidth > 80 {
		overlayWidth = 80
	}
	indent := overlayIndent

	var b strings.Builder

	if s.Err != nil {
		b.WriteString(indent + Styles.ErrorText.Render("Error: "+s.Err.Error()) + "\n\n")
	}

	// If confirming save, show the confirmation prompt
	if s.Confirming {
		b.WriteString("\n" + indent + Styles.WarningText.Render("Save changes?") + "\n")
		b.WriteString("\n" + Styles.Footer.Render(indent+"[enter] save  [esc] discard"))
		return Styles.OverlayBorder.Width(overlayWidth).Render(
			Styles.OverlayTitle.Render("Configuration") + "\n\n" + b.String(),
		)
	}

	if s.form() != nil {
		b.WriteString(Styles.OverlayTitle.Render(tabName(s.Tab)) + "\n")
		b.WriteString(s.form().View())
		b.WriteString("\n" + Styles.Footer.Render(indent+"esc close"))
	} else {
		b.WriteString(indent + Styles.DetailDim.Render("Loading configuration...") + "\n")
	}

	return Styles.OverlayBorder.Width(overlayWidth).Render(
		Styles.OverlayTitle.Render("Configuration") + "\n\n" + b.String(),
	)
}

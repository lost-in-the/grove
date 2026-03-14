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
	Key         string // TOML key path, e.g. "tui.skip_branch_notice"
	Label       string // display name
	Value       string // current value as string
	Default     string // default value for display
	Type        ConfigFieldType
	Options     []string // for Enum type
	Description string   // help text
	Placeholder string   // shown when value is empty (defaults to "(empty)")
}

// ConfigState holds the state for the config overlay.
type ConfigState struct {
	Tab               ConfigTab
	Fields            [][]ConfigField // fields per tab
	Cursor            int             // field cursor within current tab
	Editing           bool            // inline edit active
	EditBuffer        string          // current edit value (manual input)
	EditCursorPos     int             // cursor position within EditBuffer
	EditOptionCursor  int             // cursor for enum/bool option selection
	EditOriginalValue string          // value before edit started
	Err               error
	Dirty             bool           // unsaved changes exist
	Confirming        bool           // save confirmation prompt active
	Config            *config.Config // loaded config

	// Huh form integration
	Form       *huh.Form        // the embedded Huh form (nil until fields loaded)
	FormValues *configFormValues // value bindings for the form
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

// saveConfigCmd saves modified config fields.
func saveConfigCmd(state *ConfigState) tea.Cmd {
	return func() tea.Msg {
		updates := make(map[string]string)
		for _, tabFields := range state.Fields {
			for _, field := range tabFields {
				// Only save changed fields
				if field.Value != field.Default {
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
			Default:     cfg.ProjectName,
			Type:        ConfigString,
			Description: "Project name",
		},
		{
			Key:         "alias",
			Label:       "alias",
			Value:       cfg.Alias,
			Default:     cfg.Alias,
			Type:        ConfigString,
			Description: "Short name for display",
		},
		{
			Key:         "projects_dir",
			Label:       "projects_dir",
			Value:       cfg.ProjectsDir,
			Default:     cfg.ProjectsDir,
			Type:        ConfigString,
			Description: "Where worktrees are created",
		},
		{
			Key:         "default_base_branch",
			Label:       "default_branch",
			Value:       cfg.DefaultBranch,
			Default:     cfg.DefaultBranch,
			Type:        ConfigString,
			Description: "Base branch for new worktrees",
		},
	}

	// Behavior tab
	skipNotice := "false"
	if cfg.TUI.SkipBranchNotice != nil && *cfg.TUI.SkipBranchNotice {
		skipNotice = "true"
	}
	tmuxMode := cfg.Tmux.Mode
	if tmuxMode == "" {
		tmuxMode = "auto"
	}
	fields[ConfigTabBehavior] = []ConfigField{
		{
			Key:         "switch.dirty_handling",
			Label:       "dirty_handling",
			Value:       cfg.Switch.DirtyHandling,
			Default:     cfg.Switch.DirtyHandling,
			Type:        ConfigEnum,
			Options:     []string{"auto-stash", "prompt", "refuse"},
			Description: "How to handle dirty worktree on switch",
		},
		{
			Key:         "tmux.mode",
			Label:       "tmux_mode",
			Value:       tmuxMode,
			Default:     tmuxMode,
			Type:        ConfigEnum,
			Options:     []string{"auto", "manual", "off"},
			Description: "Tmux session behavior: auto-attach, print instructions, or skip",
		},
		{
			Key:         "naming.pattern",
			Label:       "naming_pattern",
			Value:       cfg.Naming.Pattern,
			Default:     cfg.Naming.Pattern,
			Type:        ConfigString,
			Description: "Pattern for naming worktrees",
		},
		{
			Key:         "tui.skip_branch_notice",
			Label:       "skip_branch_notice",
			Value:       skipNotice,
			Default:     skipNotice,
			Type:        ConfigBool,
			Description: "Skip branch-exists notice",
		},
		{
			Key:         "tui.default_branch_action",
			Label:       "default_branch_action",
			Value:       cfg.TUI.DefaultBranchAction,
			Default:     cfg.TUI.DefaultBranchAction,
			Type:        ConfigEnum,
			Options:     []string{"split", "fork"},
			Description: "Default action when branch exists",
			Placeholder: "(prompt each time)",
		},
	}

	// Plugins tab
	dockerEnabled := "true"
	dockerAutoStart := "true"
	dockerAutoStop := "false"
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
			Default:     dockerEnabled,
			Type:        ConfigBool,
			Description: "Enable Docker plugin",
		},
		{
			Key:         "plugins.docker.auto_start",
			Label:       "docker_auto_start",
			Value:       dockerAutoStart,
			Default:     dockerAutoStart,
			Type:        ConfigBool,
			Description: "Auto-start containers on switch",
		},
		{
			Key:         "plugins.docker.auto_stop",
			Label:       "docker_auto_stop",
			Value:       dockerAutoStop,
			Default:     dockerAutoStop,
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
			Default:     strings.Join(cfg.Protection.Protected, ", "),
			Type:        ConfigList,
			Description: "Protected worktrees (comma-separated)",
			Placeholder: "(none — add worktree names)",
		},
		{
			Key:         "protection.immutable",
			Label:       "immutable",
			Value:       strings.Join(cfg.Protection.Immutable, ", "),
			Default:     strings.Join(cfg.Protection.Immutable, ", "),
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

// handleConfigKey handles key input for the config overlay.
func (m Model) handleConfigKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.configState == nil {
		m.activeView = ViewDashboard
		return m, nil
	}

	s := m.configState

	// Handle confirmation prompt keys
	if s.Confirming {
		switch {
		case key.Matches(msg, m.keys.Enter):
			// Save and close
			m.activeView = ViewDashboard
			m.configState = nil
			return m, saveConfigCmd(s)
		case key.Matches(msg, m.keys.Escape):
			// Discard and close
			m.activeView = ViewDashboard
			m.configState = nil
			return m, nil
		}
		return m, nil
	}

	// Escape closes the overlay (with dirty check)
	if key.Matches(msg, m.keys.Escape) {
		if s.Dirty {
			s.Confirming = true
			return m, nil
		}
		m.activeView = ViewDashboard
		m.configState = nil
		return m, nil
	}

	// Delegate to Huh form if available
	if s.Form != nil {
		return m.updateConfigForm(msg)
	}

	// Legacy fallback: no form yet (fields not loaded)
	return m, nil
}

// handleConfigFormMsg forwards non-key messages to the Huh form.
// Called from the main Update() method for cursor blink, spinner, etc.
func (m Model) handleConfigFormMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.configState == nil || m.configState.Form == nil {
		return m, nil
	}

	model, cmd := m.configState.Form.Update(msg)
	if f, ok := model.(*huh.Form); ok {
		m.configState.Form = f
	}

	return m, cmd
}

// updateConfigForm sends a message to the Huh form and checks completion.
func (m Model) updateConfigForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	s := m.configState

	model, cmd := s.Form.Update(msg)
	if f, ok := model.(*huh.Form); ok {
		s.Form = f
	}

	// Check if form was completed (user navigated through all groups)
	if s.Form.State == huh.StateCompleted {
		// Sync form values back to fields and detect dirty
		m.syncConfigFormValues()
		if s.Dirty {
			m.activeView = ViewDashboard
			m.configState = nil
			return m, saveConfigCmd(s)
		}
		m.activeView = ViewDashboard
		m.configState = nil
		return m, nil
	}

	// Check if form was aborted
	if s.Form.State == huh.StateAborted {
		m.activeView = ViewDashboard
		m.configState = nil
		return m, nil
	}

	return m, cmd
}

// syncConfigFormValues reads values from form bindings back into ConfigFields
// and marks dirty if anything changed from the default.
func (m *Model) syncConfigFormValues() {
	s := m.configState
	if s == nil || s.FormValues == nil {
		return
	}

	for tabIdx := range s.Fields {
		for fieldIdx := range s.Fields[tabIdx] {
			f := &s.Fields[tabIdx][fieldIdx]

			// Bool fields are stored in FormValues.bools
			if f.Type == ConfigBool {
				if bPtr, ok := s.FormValues.bools[f.Key]; ok {
					if *bPtr {
						f.Value = "true"
					} else {
						f.Value = "false"
					}
				}
			}
			// String/Enum/List fields are bound directly via &f.Value

			if f.Value != f.Default {
				s.Dirty = true
			}
		}
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

	// Render Huh form if available
	if s.Form != nil {
		formView := s.Form.View()
		b.WriteString(formView)
		b.WriteString("\n" + Styles.Footer.Render(indent+"esc close"))
	} else {
		b.WriteString(indent + Styles.DetailDim.Render("Loading configuration...") + "\n")
	}

	return Styles.OverlayBorder.Width(overlayWidth).Render(
		Styles.OverlayTitle.Render("Configuration") + "\n\n" + b.String(),
	)
}

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"github.com/LeahArmstrong/grove-cli/internal/config"
)

// ConfigTab identifies which tab is active in the config overlay.
type ConfigTab int

const (
	ConfigTabGeneral    ConfigTab = iota
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
	Key         string          // TOML key path, e.g. "tui.skip_branch_notice"
	Label       string          // display name
	Value       string          // current value as string
	Default     string          // default value for display
	Type        ConfigFieldType
	Options     []string        // for Enum type
	Description string          // help text
}

// ConfigState holds the state for the config overlay.
type ConfigState struct {
	Tab      ConfigTab
	Fields   [][]ConfigField // fields per tab
	Cursor   int             // field cursor within current tab
	Editing  bool            // inline edit active
	EditForm *huh.Form       // active Huh form for editing
	Err      error
	Dirty    bool           // unsaved changes exist
	Config   *config.Config // loaded config
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
					// Quote string values for TOML
					if field.Type == ConfigString || field.Type == ConfigEnum {
						val = `"` + val + `"`
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
		},
		{
			Key:         "protection.immutable",
			Label:       "immutable",
			Value:       strings.Join(cfg.Protection.Immutable, ", "),
			Default:     strings.Join(cfg.Protection.Immutable, ", "),
			Type:        ConfigList,
			Description: "Immutable worktrees (comma-separated)",
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
func (m Model) handleConfigKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.configState == nil {
		m.activeView = ViewDashboard
		return m, nil
	}

	s := m.configState

	// If editing a field, delegate to the edit form
	if s.Editing {
		return m.handleConfigEditKey(msg)
	}

	switch {
	case key.Matches(msg, m.keys.Escape):
		if s.Dirty {
			// Save on close if dirty
			m.activeView = ViewDashboard
			m.configState = nil
			return m, saveConfigCmd(s)
		}
		m.activeView = ViewDashboard
		m.configState = nil
		return m, nil

	case key.Matches(msg, m.keys.Tab):
		s.Tab = ConfigTab((int(s.Tab) + 1) % int(ConfigTabCount))
		s.Cursor = 0
		return m, nil

	case key.Matches(msg, m.keys.Up):
		if s.Cursor > 0 {
			s.Cursor--
		}
		return m, nil

	case key.Matches(msg, m.keys.Down):
		tabFields := s.Fields[s.Tab]
		if s.Cursor < len(tabFields)-1 {
			s.Cursor++
		}
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		tabFields := s.Fields[s.Tab]
		if s.Cursor < len(tabFields) {
			field := &tabFields[s.Cursor]
			s.Editing = true
			s.EditForm = newConfigEditForm(field)
			return m, s.EditForm.Init()
		}
		return m, nil
	}

	return m, nil
}

// handleConfigEditKey handles key input while editing a config field.
func (m Model) handleConfigEditKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := m.configState
	if s.EditForm == nil {
		s.Editing = false
		return m, nil
	}

	model, cmd := s.EditForm.Update(msg)
	s.EditForm = model.(*huh.Form)

	if s.EditForm.State == huh.StateAborted {
		s.Editing = false
		s.EditForm = nil
		return m, nil
	}

	if s.EditForm.State == huh.StateCompleted {
		// Extract value from the form
		tabFields := s.Fields[s.Tab]
		if s.Cursor < len(tabFields) {
			field := &tabFields[s.Cursor]
			newVal := s.EditForm.GetString("value")
			if newVal != field.Value {
				field.Value = newVal
				s.Dirty = true
			}
		}
		s.Editing = false
		s.EditForm = nil
		return m, nil
	}

	return m, cmd
}

// forwardToConfigHuhForm forwards non-key messages to the active config edit form.
func (m Model) forwardToConfigHuhForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	s := m.configState
	if s.EditForm == nil {
		return m, nil
	}
	model, cmd := s.EditForm.Update(msg)
	s.EditForm = model.(*huh.Form)
	return m, cmd
}

// newConfigEditForm creates a Huh form for editing a config field.
func newConfigEditForm(field *ConfigField) *huh.Form {
	switch field.Type {
	case ConfigBool:
		return huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(field.Label).
					Description(field.Description).
					Options(
						huh.NewOption("true", "true"),
						huh.NewOption("false", "false"),
					).
					Value(&field.Value).
					Key("value"),
			),
		).WithTheme(huh.ThemeCharm()).WithShowHelp(false).WithAccessible(isHighContrast())

	case ConfigEnum:
		opts := make([]huh.Option[string], len(field.Options))
		for i, o := range field.Options {
			opts[i] = huh.NewOption(o, o)
		}
		return huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(field.Label).
					Description(field.Description).
					Options(opts...).
					Value(&field.Value).
					Key("value"),
			),
		).WithTheme(huh.ThemeCharm()).WithShowHelp(false).WithAccessible(isHighContrast())

	default: // ConfigString, ConfigList
		return huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title(field.Label).
					Placeholder(field.Default).
					Description(field.Description).
					Value(&field.Value).
					Key("value"),
			),
		).WithTheme(huh.ThemeCharm()).WithShowHelp(false).WithAccessible(isHighContrast())
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
	contentWidth := overlayWidth - 6
	indent := huhOverlayIndent

	var b strings.Builder

	// Tab bar
	var tabs []string
	for i := ConfigTab(0); i < ConfigTabCount; i++ {
		name := tabName(i)
		if i == s.Tab {
			tabs = append(tabs, Styles.Header.Render("["+name+"]"))
		} else {
			tabs = append(tabs, Styles.TextMuted.Render(" "+name+" "))
		}
	}
	b.WriteString(indent + strings.Join(tabs, "  ") + "\n\n")

	if s.Err != nil {
		b.WriteString(indent + Styles.ErrorText.Render("Error: "+s.Err.Error()) + "\n\n")
	}

	// If editing, show the edit form
	if s.Editing && s.EditForm != nil {
		b.WriteString(s.EditForm.View())
		b.WriteString("\n" + Styles.Footer.Render(indent+"[enter] save  [esc] cancel"))
		return Styles.OverlayBorder.Width(overlayWidth).Render(
			Styles.OverlayTitle.Render("Configuration") + "\n\n" + b.String(),
		)
	}

	// Field list for current tab
	tabFields := s.Fields[s.Tab]
	if len(tabFields) == 0 {
		b.WriteString(indent + Styles.DetailDim.Render("No config fields available.") + "\n")
	} else {
		// Calculate label width for alignment
		labelWidth := 0
		for _, f := range tabFields {
			if len(f.Label) > labelWidth {
				labelWidth = len(f.Label)
			}
		}

		for i, field := range tabFields {
			cursor := "  "
			if i == s.Cursor {
				cursor = Styles.ListCursor.String()
			}

			label := padRight(field.Label, labelWidth)
			value := field.Value
			if value == "" {
				value = Styles.DetailDim.Render("(empty)")
			}

			// Truncate value to fit
			maxValWidth := contentWidth - labelWidth - 8
			if maxValWidth < 10 {
				maxValWidth = 10
			}
			if len(value) > maxValWidth {
				value = value[:maxValWidth-3] + "..."
			}

			line := indent + cursor + Styles.DetailLabel.Render(label) + "  " + Styles.DetailValue.Render(value)
			b.WriteString(line + "\n")

			// Show description for selected field
			if i == s.Cursor && field.Description != "" {
				b.WriteString(indent + "    " + Styles.DetailDim.Render(field.Description) + "\n")
			}
		}
	}

	if s.Dirty {
		b.WriteString("\n" + indent + Styles.WarningText.Render("● unsaved changes") + "\n")
	}

	b.WriteString("\n" + Styles.Footer.Render(indent+"tab next section  ↑↓ navigate  enter edit  esc close"))

	return Styles.OverlayBorder.Width(overlayWidth).Render(
		Styles.OverlayTitle.Render("Configuration") + "\n\n" + b.String(),
	)
}

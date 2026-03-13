package tui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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

	// If editing a field, delegate to the edit form
	if s.Editing {
		return m.handleConfigEditKey(msg)
	}

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

	switch {
	case key.Matches(msg, m.keys.Escape):
		if s.Dirty {
			s.Confirming = true
			return m, nil
		}
		m.activeView = ViewDashboard
		m.configState = nil
		return m, nil

	case key.Matches(msg, m.keys.Tab):
		s.Tab = ConfigTab((int(s.Tab) + 1) % int(ConfigTabCount))
		s.Cursor = 0
		return m, nil

	case key.Matches(msg, m.keys.ShiftTab):
		s.Tab = ConfigTab((int(s.Tab) - 1 + int(ConfigTabCount)) % int(ConfigTabCount))
		s.Cursor = 0
		return m, nil

	case key.Matches(msg, m.keys.Up):
		if s.Cursor > 0 {
			s.Cursor--
		}
		return m, nil

	case key.Matches(msg, m.keys.Down):
		tabFields := s.Fields[s.Tab]
		if len(tabFields) > 0 && s.Cursor < len(tabFields)-1 {
			s.Cursor++
		}
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		tabFields := s.Fields[s.Tab]
		if s.Cursor < len(tabFields) {
			field := &tabFields[s.Cursor]
			s.EditOriginalValue = field.Value
			s.EditBuffer = field.Value
			s.EditCursorPos = len(field.Value)
			s.EditOptionCursor = 0
			s.Editing = true

			// For enum/bool, set cursor to current value
			if field.Type == ConfigBool {
				if field.Value == "false" {
					s.EditOptionCursor = 1
				}
			} else if field.Type == ConfigEnum {
				for i, opt := range field.Options {
					if opt == field.Value {
						s.EditOptionCursor = i
						break
					}
				}
			}
		}
		return m, nil
	}

	return m, nil
}

// handleConfigEditKey handles key input while editing a config field.
func (m Model) handleConfigEditKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := m.configState
	tabFields := s.Fields[s.Tab]
	if s.Cursor >= len(tabFields) {
		s.Editing = false
		return m, nil
	}
	field := &tabFields[s.Cursor]

	if key.Matches(msg, m.keys.Escape) {
		// Restore original value
		field.Value = s.EditOriginalValue
		s.Editing = false
		return m, nil
	}

	switch field.Type {
	case ConfigBool:
		switch {
		case key.Matches(msg, m.keys.Up):
			if s.EditOptionCursor > 0 {
				s.EditOptionCursor--
			}
		case key.Matches(msg, m.keys.Down):
			if s.EditOptionCursor < 1 {
				s.EditOptionCursor++
			}
		case key.Matches(msg, m.keys.Enter):
			if s.EditOptionCursor == 0 {
				field.Value = "true"
			} else {
				field.Value = "false"
			}
			if field.Value != s.EditOriginalValue {
				s.Dirty = true
			}
			s.Editing = false
		}
		return m, nil

	case ConfigEnum:
		switch {
		case key.Matches(msg, m.keys.Up):
			if s.EditOptionCursor > 0 {
				s.EditOptionCursor--
			}
		case key.Matches(msg, m.keys.Down):
			if s.EditOptionCursor < len(field.Options)-1 {
				s.EditOptionCursor++
			}
		case key.Matches(msg, m.keys.Enter):
			if len(field.Options) == 0 {
				s.Editing = false
				return m, nil
			}
			field.Value = field.Options[s.EditOptionCursor]
			if field.Value != s.EditOriginalValue {
				s.Dirty = true
			}
			s.Editing = false
		}
		return m, nil

	default: // ConfigString, ConfigList
		switch {
		case key.Matches(msg, m.keys.Enter):
			field.Value = s.EditBuffer
			if field.Value != s.EditOriginalValue {
				s.Dirty = true
			}
			s.Editing = false
		case msg.Code == tea.KeyBackspace:
			if len(s.EditBuffer) > 0 {
				s.EditBuffer = s.EditBuffer[:len(s.EditBuffer)-1]
			}
		case isPrintableText(msg.Text):
			s.EditBuffer += msg.Text
		}
		return m, nil
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
	indent := overlayIndent

	var b strings.Builder

	// Tab bar
	var tabs []string
	for i := ConfigTab(0); i < ConfigTabCount; i++ {
		name := tabName(i)
		if i == s.Tab {
			tabs = append(tabs, Styles.Header.Bold(true).Underline(true).Render(name))
		} else {
			tabs = append(tabs, Styles.TextMuted.Render(name))
		}
	}
	tabBar := indent + strings.Join(tabs, "  ")
	tabRule := indent + Styles.TextMuted.Render(strings.Repeat("─", contentWidth))
	b.WriteString(tabBar + "\n" + tabRule + "\n\n")

	if s.Err != nil {
		b.WriteString(indent + Styles.ErrorText.Render("Error: "+s.Err.Error()) + "\n\n")
	}

	// If editing, show the manual edit UI
	if s.Editing {
		tabFields := s.Fields[s.Tab]
		if s.Cursor < len(tabFields) {
			field := tabFields[s.Cursor]
			b.WriteString(indent + Styles.DetailLabel.Render(field.Label) + "\n")
			if field.Description != "" {
				b.WriteString(indent + Styles.DetailDim.Render(field.Description) + "\n")
			}
			b.WriteString("\n")

			switch field.Type {
			case ConfigBool:
				options := []string{"true", "false"}
				for i, opt := range options {
					cursor := "  "
					if i == s.EditOptionCursor {
						cursor = Styles.ListCursor.Render("❯ ")
					}
					b.WriteString(indent + cursor + opt + "\n")
				}
			case ConfigEnum:
				for i, opt := range field.Options {
					cursor := "  "
					if i == s.EditOptionCursor {
						cursor = Styles.ListCursor.Render("❯ ")
					}
					b.WriteString(indent + cursor + opt + "\n")
				}
			default: // ConfigString, ConfigList
				b.WriteString(indent + fmt.Sprintf("%s█\n", s.EditBuffer))
			}
		}
		b.WriteString("\n" + Styles.Footer.Render(indent+"[enter] save  [esc] cancel"))
		return Styles.OverlayBorder.Width(overlayWidth).Render(
			Styles.OverlayTitle.Render("Configuration") + "\n\n" + b.String(),
		)
	}

	// If confirming save, show the confirmation prompt
	if s.Confirming {
		b.WriteString("\n" + indent + Styles.WarningText.Render("Save changes?") + "\n")
		b.WriteString("\n" + Styles.Footer.Render(indent+"[enter] save  [esc] discard"))
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
				cursor = Styles.ListCursor.Render("❯ ")
			}

			label := padRight(field.Label, labelWidth)
			value := field.Value
			if value == "" {
				placeholder := "(empty)"
				if field.Placeholder != "" {
					placeholder = field.Placeholder
				}
				value = Styles.DetailDim.Render(placeholder)
			}

			// Truncate value to fit
			maxValWidth := contentWidth - labelWidth - 8
			if maxValWidth < 10 {
				maxValWidth = 10
			}
			if lipgloss.Width(value) > maxValWidth {
				value = truncate(value, maxValWidth)
			}

			// Use warning color for changed fields
			valueStyle := Styles.DetailValue
			if field.Value != field.Default {
				valueStyle = Styles.WarningText
			}

			line := indent + cursor + Styles.DetailLabel.Render(label) + "  " + valueStyle.Render(value)
			b.WriteString(line + "\n")

			// Show description for selected field
			if i == s.Cursor && field.Description != "" {
				b.WriteString(indent + "    " + Styles.DetailDim.Render(field.Description) + "\n")
			}
		}
	}

	if s.Dirty {
		b.WriteString("\n" + Styles.Footer.Render(indent+"tab/shift+tab sections  ↑↓ navigate  enter edit  esc save & close"))
	} else {
		b.WriteString("\n" + Styles.Footer.Render(indent+"tab/shift+tab sections  ↑↓ navigate  enter edit  esc close"))
	}

	return Styles.OverlayBorder.Width(overlayWidth).Render(
		Styles.OverlayTitle.Render("Configuration") + "\n\n" + b.String(),
	)
}

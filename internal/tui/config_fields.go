package tui

import (
	"strconv"
	"strings"

	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/worktree"
)

// configOption is one value a choice or true/false field can take, with
// what it does in plain words.
type configOption struct {
	Value   string
	Meaning string
}

// ConfigField is one setting in the panel: a plain label for the list, the
// key as the file spells it, and everything the context panel explains.
type ConfigField struct {
	Key         string // TOML key path, e.g. "switch.dirty_handling"
	Label       string // plain words, for the list
	Group       string // heading above the field; empty in a one-group section
	Value       string // as the file spells it; lists join their entries with ", "
	Type        ConfigFieldType
	Options     []configOption // choice and true/false fields, in order
	Default     string         // the value grove uses when the file sets none
	Description string         // one or two plain sentences
	Notes       []string       // text and list fields: what to type
	Empty       string         // shown in the list when Value is empty

	set   func(*config.Config, string) // applies a value, for validation
	check func(string) error           // rules config.Validate does not cover

	// renamesWorktrees: existing worktrees' short names are read from their
	// folder names with this value, so changing it leaves them unmatched.
	renamesWorktrees bool
}

// validate reports whether value is one config loading would accept, by
// applying it to a copy of base and running the same checks.
func (f ConfigField) validate(base *config.Config, value string) error {
	if f.check != nil {
		if err := f.check(value); err != nil {
			return err
		}
	}
	if base == nil || f.set == nil {
		return nil
	}
	candidate := *base
	f.set(&candidate, value)
	return config.Validate(&candidate)
}

// tomlValue is value as the file writes it.
func (f ConfigField) tomlValue(value string) string {
	switch f.Type {
	case ConfigBool:
		return value
	case ConfigList:
		return tomlList(value)
	default:
		return tomlString(value)
	}
}

// splitList is a list field's text as entries, trimmed, empty ones dropped.
func splitList(list string) []string {
	var items []string
	for _, item := range strings.Split(list, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}

func boolString(p *bool, fallback bool) string {
	if p != nil {
		return strconv.FormatBool(*p)
	}
	return strconv.FormatBool(fallback)
}

func boolPtr(v string) *bool {
	b := v == boolTrue
	return &b
}

// orDefault is v, or def when the file leaves v empty.
func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

var dirtyHandlingMeanings = map[string]string{
	"warn":       "Tell you, then move",
	"ignore":     "Move, say nothing",
	"prompt":     "Ask first",
	"refuse":     "Do not move",
	"auto-stash": "Stash the files, then move",
}

func dirtyHandlingOptions() []configOption {
	out := make([]configOption, len(config.DirtyHandlingValues))
	for i, v := range config.DirtyHandlingValues {
		out[i] = configOption{v, dirtyHandlingMeanings[v]}
	}
	return out
}

// populateConfigFields maps a config to the panel's fields, per section.
// Meanings follow docs/CONFIGURATION_REFERENCE.md and what the code does.
func populateConfigFields(cfg *config.Config) [][]ConfigField {
	fields := make([][]ConfigField, ConfigTabCount)
	d := &cfg.Plugins.Docker

	fields[ConfigTabGeneral] = []ConfigField{
		{
			Key: "project_name", Label: "Project name", Type: ConfigString,
			Value:            cfg.ProjectName,
			Default:          "the repo name",
			Description:      "The {project} part of worktree folder and session names.",
			Notes:            []string{"Empty: the repo name from the git remote, else the folder name."},
			Empty:            "not set",
			set:              func(c *config.Config, v string) { c.ProjectName = v },
			renamesWorktrees: true,
		},
		{
			Key: "projects_dir", Label: "Projects folder", Type: ConfigString,
			Value:       cfg.ProjectsDir,
			Default:     "~/projects",
			Description: "The folder your projects live in. The Docker plugin looks here for a worktree by name.",
			Notes:       []string{"A path; ~ means your home folder."},
			set:         func(c *config.Config, v string) { c.ProjectsDir = v },
		},
		{
			Key: "default_base_branch", Label: "Default branch", Type: ConfigString,
			Value:       cfg.DefaultBranch,
			Default:     "main",
			Description: "New branches start from it in the new worktree form. Ahead and behind counts compare with it.",
			Notes:       []string{"A branch name, such as main or develop."},
			set:         func(c *config.Config, v string) { c.DefaultBranch = v },
		},
	}

	const moving, names = "Moving between worktrees", "Names"
	fields[ConfigTabBehavior] = []ConfigField{
		{
			Key: "switch.dirty_handling", Label: "Uncommitted files", Group: moving, Type: ConfigEnum,
			Value:       cfg.Switch.DirtyHandling,
			Options:     dirtyHandlingOptions(),
			Default:     config.DirtyHandlingValues[0],
			Description: "What grove does when the worktree you leave has uncommitted files.",
			set:         func(c *config.Config, v string) { c.Switch.DirtyHandling = v },
		},
		{
			Key: "switch.container_switch", Label: "Docker stack", Group: moving, Type: ConfigEnum,
			Value: orDefault(cfg.Switch.ContainerSwitch, "off"),
			Options: []configOption{
				{"off", "The stack stays where it is"},
				{"prompt", "Ask before the stack follows"},
				{"auto", "The stack follows every move"},
			},
			Default:     "off",
			Description: "Whether the stack follows you to the worktree you move to. grove up or --up still bring it.",
			set:         func(c *config.Config, v string) { c.Switch.ContainerSwitch = v },
		},
		{
			Key: "session.open_in", Label: "Where you land", Group: moving, Type: ConfigEnum,
			Value: cfg.EffectiveOpenIn(),
			Options: []configOption{
				{config.OpenInSession, "The worktree's own session"},
				{config.OpenInHere, "This shell or pane; only the folder changes"},
			},
			Default:     config.OpenInSession,
			Description: "Where you end up when you open a worktree. A session is a tmux session or a herdr workspace.",
			set:         func(c *config.Config, v string) { c.Session.OpenIn = v },
		},
		{
			Key: "tmux.mode", Label: "Sessions", Group: moving, Type: ConfigEnum,
			Value: orDefault(cfg.Tmux.Mode, "auto"),
			Options: []configOption{
				{"auto", "Create sessions and take you there"},
				{"manual", "Create sessions; you join with grove join"},
				{"off", "No sessions at all"},
			},
			Default:     "auto",
			Description: "Whether grove creates sessions for worktrees and moves you into them.",
			set:         func(c *config.Config, v string) { c.Tmux.Mode = v },
		},
		{
			Key: "naming.pattern", Label: "Folder pattern", Group: names, Type: ConfigString,
			Value:       cfg.Naming.Pattern,
			Default:     worktree.DefaultNamePattern,
			Description: "How grove names a new worktree's folder. Session names always use {project}-{name}.",
			Notes: []string{
				"{project} is the project name, {name} the worktree's short name.",
				"Each appears once. Other characters: letters, digits, . _ -",
			},
			set:              func(c *config.Config, v string) { c.Naming.Pattern = v },
			check:            worktree.ValidateNamePattern,
			renamesWorktrees: true,
		},
	}

	const followsOnly = " Only when the stack follows a move: Docker stack is prompt or auto, or --up."
	fields[ConfigTabPlugins] = []ConfigField{
		{
			Key: "plugins.docker.enabled", Label: "Docker plugin", Type: ConfigBool,
			Value: boolString(d.Enabled, true),
			Options: []configOption{
				{boolTrue, "Grove can bring the stack up and down"},
				{boolFalse, "Grove never touches Docker"},
			},
			Default:     boolTrue,
			Description: "Whether grove manages the Docker stack at all.",
			set:         func(c *config.Config, v string) { c.Plugins.Docker.Enabled = boolPtr(v) },
		},
		{
			Key: "plugins.docker.auto_start", Label: "Stack up on arrival", Type: ConfigBool,
			Value: boolString(d.AutoStart, true),
			Options: []configOption{
				{boolTrue, "Bring the stack up on the worktree you move to"},
				{boolFalse, "Leave it down"},
			},
			Default:     boolTrue,
			Description: "Whether the stack comes up on the worktree you arrive at." + followsOnly,
			set:         func(c *config.Config, v string) { c.Plugins.Docker.AutoStart = boolPtr(v) },
		},
		{
			Key: "plugins.docker.auto_stop", Label: "Stack down on leaving", Type: ConfigBool,
			Value: boolString(d.AutoStop, false),
			Options: []configOption{
				{boolTrue, "Take the stack down on the worktree you leave"},
				{boolFalse, "Leave it up"},
			},
			Default:     boolFalse,
			Description: "Whether the stack goes down on the worktree you leave." + followsOnly,
			set:         func(c *config.Config, v string) { c.Plugins.Docker.AutoStop = boolPtr(v) },
		},
	}

	const listNote = "Short worktree names, separated by commas: main, staging"
	fields[ConfigTabProtection] = []ConfigField{
		{
			Key: "protection.protected", Label: "Protected worktrees", Type: ConfigList,
			Value:       strings.Join(cfg.Protection.Protected, ", "),
			Default:     "none",
			Description: "Grove will not delete or rename these, or switch their branch. grove rm needs --force --unprotect.",
			Notes:       []string{listNote},
			Empty:       "none",
			set:         func(c *config.Config, v string) { c.Protection.Protected = splitList(v) },
		},
		{
			Key: "protection.immutable", Label: "No incoming changes", Type: ConfigList,
			Value:       strings.Join(cfg.Protection.Immutable, ", "),
			Default:     "none",
			Description: "grove graft will not bring another worktree's changes into these. They can still be deleted.",
			Notes:       []string{listNote},
			Empty:       "none",
			set:         func(c *config.Config, v string) { c.Protection.Immutable = splitList(v) },
		},
	}

	return fields
}

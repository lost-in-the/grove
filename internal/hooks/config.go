package hooks

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// FindOverride returns the first matching override for a branch/worktree name, or nil
func (c *HooksConfig) FindOverride(branch, worktree string) *Override {
	for i, o := range c.Overrides {
		matched, _ := filepath.Match(o.Match, branch)
		if !matched {
			matched, _ = filepath.Match(o.Match, worktree)
		}
		if matched {
			return &c.Overrides[i]
		}
	}
	return nil
}

// ApplyOverride filters actions based on an override and adds extra actions
func ApplyOverride(actions []HookAction, override *Override, mainPath string) []HookAction {
	if override == nil {
		return actions
	}

	if override.SkipHooks {
		return nil
	}

	// Build skip set
	skipSet := make(map[string]bool)
	for _, s := range override.Skip {
		skipSet[s] = true
	}

	// Filter out skipped action types
	var filtered []HookAction
	for _, a := range actions {
		if !skipSet[a.Type] {
			filtered = append(filtered, a)
		}
	}

	// Add extra copy actions
	for _, f := range override.ExtraCopy {
		filtered = append(filtered, HookAction{
			Type:      "copy",
			From:      f,
			To:        f,
			OnFailure: "warn",
			Timeout:   60,
		})
	}

	// Add extra run actions
	for _, cmd := range override.ExtraRun {
		filtered = append(filtered, HookAction{
			Type:       "command",
			Command:    cmd,
			WorkingDir: "new",
			Timeout:    300,
			OnFailure:  "warn",
		})
	}

	return filtered
}

// HooksConfig represents the hooks configuration from hooks.toml
type HooksConfig struct {
	Hooks     EventHooks `toml:"hooks"`
	Overrides []Override `toml:"overrides"`
}

// Override defines per-branch/worktree overrides for hook behavior
type Override struct {
	Match     string   `toml:"match"`      // glob pattern on branch or worktree name
	SkipHooks bool     `toml:"skip_hooks"` // skip all hooks for matching branches
	Skip      []string `toml:"skip"`       // skip specific action types: "copy", "symlink", "command"
	ExtraCopy []string `toml:"extra_copy"` // additional files to copy
	ExtraRun  []string `toml:"extra_run"`  // additional commands to run
}

// EventHooks maps event names to lists of hook actions
type EventHooks struct {
	PreCreate  []HookAction `toml:"pre_create"`
	PostCreate []HookAction `toml:"post_create"`
	PreSwitch  []HookAction `toml:"pre_switch"`
	PostSwitch []HookAction `toml:"post_switch"`
	PreRemove  []HookAction `toml:"pre_remove"`
	PostRemove []HookAction `toml:"post_remove"`

	// Override flags - if true, clears global hooks for this event
	OverridePreCreate  bool `toml:"override_pre_create"`
	OverridePostCreate bool `toml:"override_post_create"`
	OverridePreSwitch  bool `toml:"override_pre_switch"`
	OverridePostSwitch bool `toml:"override_post_switch"`
	OverridePreRemove  bool `toml:"override_pre_remove"`
	OverridePostRemove bool `toml:"override_post_remove"`
}

// HookAction represents a single hook action configuration
type HookAction struct {
	// Type of action: copy, symlink, command, template
	Type string `toml:"type"`

	// For copy, symlink, template actions
	From string `toml:"from"` // Source path (relative to main worktree or absolute)
	To   string `toml:"to"`   // Destination path (relative to new worktree or absolute)

	// For command action
	Command    string `toml:"command"`     // Shell command to execute
	WorkingDir string `toml:"working_dir"` // "new" (default), "main", or absolute path
	Timeout    int    `toml:"timeout"`     // Timeout in seconds (default: 60)

	// For template action
	Vars map[string]string `toml:"vars"` // Additional template variables

	// Error handling
	Required  bool   `toml:"required"`   // If true, failure aborts the operation
	OnFailure string `toml:"on_failure"` // "warn" (default), "fail", "ignore"
}

// GetHooksConfigPaths returns the paths for hooks configuration files.
// Returns (globalPath, projectPath, error).
// If groveDir is provided, it is used as the .grove directory path for project
// hooks instead of discovering from cwd. This fixes lookups in secondary worktrees.
func GetHooksConfigPaths(groveDir ...string) (string, string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}

	globalHooks := filepath.Join(homeDir, ".config", "grove", "hooks.toml")

	var projectHooks string
	if len(groveDir) > 0 && groveDir[0] != "" {
		projectHooks = filepath.Join(groveDir[0], "hooks.toml")
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return globalHooks, "", err
		}
		projectHooks = filepath.Join(cwd, ".grove", "hooks.toml")
	}

	return globalHooks, projectHooks, nil
}

// LoadHooksConfig loads hooks configuration from global and project paths.
// Project config is merged over global config (or overrides if override flags are set).
// If groveDir is provided, it is forwarded to GetHooksConfigPaths.
func LoadHooksConfig(groveDir ...string) (*HooksConfig, error) {
	globalPath, projectPath, err := GetHooksConfigPaths(groveDir...)
	if err != nil {
		return nil, err
	}

	var globalCfg, projectCfg *HooksConfig

	// Load global config if it exists
	if _, err := os.Stat(globalPath); err == nil {
		globalCfg, err = loadHooksConfigFromPath(globalPath)
		if err != nil {
			return nil, err
		}
	}

	// Load project config if it exists
	if _, err := os.Stat(projectPath); err == nil {
		projectCfg, err = loadHooksConfigFromPath(projectPath)
		if err != nil {
			return nil, err
		}
	}

	// If neither exists, return empty config
	if globalCfg == nil && projectCfg == nil {
		return &HooksConfig{}, nil
	}

	// If only one exists, return it
	if globalCfg == nil {
		return projectCfg, nil
	}
	if projectCfg == nil {
		return globalCfg, nil
	}

	// Merge configs
	return mergeHooksConfigs(globalCfg, projectCfg), nil
}

// loadHooksConfigFromPath loads hooks configuration from a specific file
func loadHooksConfigFromPath(path string) (*HooksConfig, error) {
	cfg := &HooksConfig{}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Set defaults for hook actions
	setActionDefaults(&cfg.Hooks)

	return cfg, nil
}

// setActionDefaults fills in default values for hook actions
func setActionDefaults(hooks *EventHooks) {
	setDefaultsForActions(hooks.PreCreate)
	setDefaultsForActions(hooks.PostCreate)
	setDefaultsForActions(hooks.PreSwitch)
	setDefaultsForActions(hooks.PostSwitch)
	setDefaultsForActions(hooks.PreRemove)
	setDefaultsForActions(hooks.PostRemove)
}

// setDefaultsForActions sets defaults for a slice of hook actions
func setDefaultsForActions(actions []HookAction) {
	for i := range actions {
		if actions[i].Timeout == 0 {
			actions[i].Timeout = 60 // Default 60 seconds
		}
		if actions[i].WorkingDir == "" {
			actions[i].WorkingDir = "new" // Default to new worktree
		}
		if actions[i].OnFailure == "" {
			actions[i].OnFailure = "warn" // Default to warn
		}
	}
}

// mergeHooksConfigs merges project config over global config
// Project hooks append to global hooks unless override flag is set
func mergeHooksConfigs(global, project *HooksConfig) *HooksConfig {
	result := &HooksConfig{}

	// Pre-create hooks
	if project.Hooks.OverridePreCreate {
		result.Hooks.PreCreate = project.Hooks.PreCreate
	} else {
		result.Hooks.PreCreate = append(global.Hooks.PreCreate, project.Hooks.PreCreate...)
	}

	// Post-create hooks
	if project.Hooks.OverridePostCreate {
		result.Hooks.PostCreate = project.Hooks.PostCreate
	} else {
		result.Hooks.PostCreate = append(global.Hooks.PostCreate, project.Hooks.PostCreate...)
	}

	// Pre-switch hooks
	if project.Hooks.OverridePreSwitch {
		result.Hooks.PreSwitch = project.Hooks.PreSwitch
	} else {
		result.Hooks.PreSwitch = append(global.Hooks.PreSwitch, project.Hooks.PreSwitch...)
	}

	// Post-switch hooks
	if project.Hooks.OverridePostSwitch {
		result.Hooks.PostSwitch = project.Hooks.PostSwitch
	} else {
		result.Hooks.PostSwitch = append(global.Hooks.PostSwitch, project.Hooks.PostSwitch...)
	}

	// Pre-remove hooks
	if project.Hooks.OverridePreRemove {
		result.Hooks.PreRemove = project.Hooks.PreRemove
	} else {
		result.Hooks.PreRemove = append(global.Hooks.PreRemove, project.Hooks.PreRemove...)
	}

	// Post-remove hooks
	if project.Hooks.OverridePostRemove {
		result.Hooks.PostRemove = project.Hooks.PostRemove
	} else {
		result.Hooks.PostRemove = append(global.Hooks.PostRemove, project.Hooks.PostRemove...)
	}

	// Merge overrides: project overrides take precedence (prepended)
	result.Overrides = append(project.Overrides, global.Overrides...)

	return result
}

// GetActionsForEvent returns the hook actions for a specific event
func (c *HooksConfig) GetActionsForEvent(event string) []HookAction {
	switch event {
	case EventPreCreate:
		return c.Hooks.PreCreate
	case EventPostCreate:
		return c.Hooks.PostCreate
	case EventPreSwitch:
		return c.Hooks.PreSwitch
	case EventPostSwitch:
		return c.Hooks.PostSwitch
	case EventPreRemove:
		return c.Hooks.PreRemove
	case EventPostRemove:
		return c.Hooks.PostRemove
	default:
		return nil
	}
}

// HasActionsForEvent returns true if there are any actions configured for the event
func (c *HooksConfig) HasActionsForEvent(event string) bool {
	return len(c.GetActionsForEvent(event)) > 0
}

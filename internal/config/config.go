package config

import (
	"os"
	"path/filepath"
)

// Config represents the complete grove configuration
type Config struct {
	ProjectName   string           `toml:"project_name"`
	Alias         string           `toml:"alias"`
	ProjectsDir   string           `toml:"projects_dir"`
	DefaultBranch string           `toml:"default_base_branch"`
	Switch        SwitchConfig     `toml:"switch"`
	Naming        NamingConfig     `toml:"naming"`
	Tmux          TmuxConfig       `toml:"tmux"`
	Plugins       PluginsConfig    `toml:"plugins"`
	Protection    ProtectionConfig `toml:"protection"`

	// Runtime settings (from env vars, not persisted)
	NoColor        bool `toml:"-"` // GROVE_NO_COLOR - disable colored output
	Debug          bool `toml:"-"` // GROVE_DEBUG - enable debug logging
	NonInteractive bool `toml:"-"` // GROVE_NONINTERACTIVE - disable prompts
}

// ProtectionConfig controls worktree protection settings
type ProtectionConfig struct {
	Protected []string `toml:"protected"` // Cannot rm without --force --unprotect
	Immutable []string `toml:"immutable"` // Cannot apply changes to
}

// SwitchConfig controls worktree switching behavior
type SwitchConfig struct {
	DirtyHandling string `toml:"dirty_handling"` // auto-stash, prompt, refuse
}

// NamingConfig controls worktree naming conventions
type NamingConfig struct {
	Pattern string `toml:"pattern"` // Pattern for naming worktrees
}

// TmuxConfig controls tmux session behavior
type TmuxConfig struct {
	Prefix string `toml:"prefix"` // Prefix for tmux session names
}

// PluginsConfig controls plugin behavior
type PluginsConfig struct {
	Docker DockerPluginConfig `toml:"docker"`
}

// DockerPluginConfig controls docker plugin behavior
type DockerPluginConfig struct {
	Enabled   *bool `toml:"enabled"`
	AutoStart *bool `toml:"auto_start"`
	AutoStop  *bool `toml:"auto_stop"`
}

// GetConfigPaths returns the paths to check for config files
// Returns global config path and project config path
func GetConfigPaths() (string, string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}

	globalConfig := filepath.Join(homeDir, ".config", "grove", "config.toml")

	// Get current working directory for project config
	cwd, err := os.Getwd()
	if err != nil {
		return globalConfig, "", err
	}
	projectConfig := filepath.Join(cwd, ".grove", "config.toml")

	return globalConfig, projectConfig, nil
}

// Load loads the configuration from the default locations
// It starts with defaults, then loads global config, then project config,
// and finally applies environment variable overrides.
func Load() (*Config, error) {
	cfg := LoadDefaults()

	globalPath, projectPath, err := GetConfigPaths()
	if err != nil {
		return cfg, err
	}

	// GROVE_CONFIG overrides the global config path
	if envConfig := os.Getenv("GROVE_CONFIG"); envConfig != "" {
		globalPath = envConfig
	}

	// Load global config if it exists
	if _, err := os.Stat(globalPath); err == nil {
		globalCfg, err := LoadConfigFromPath(globalPath)
		if err != nil {
			return nil, err
		}
		cfg = mergeConfigs(cfg, globalCfg)
	}

	// Load project config if it exists (overrides global)
	if _, err := os.Stat(projectPath); err == nil {
		projectCfg, err := LoadConfigFromPath(projectPath)
		if err != nil {
			return nil, err
		}
		cfg = mergeConfigs(cfg, projectCfg)
	}

	// Apply environment variable overrides for runtime settings
	cfg.NoColor = envBool("GROVE_NO_COLOR")
	cfg.Debug = envBool("GROVE_DEBUG")
	cfg.NonInteractive = envBool("GROVE_NONINTERACTIVE")

	// Validate the final configuration
	if err := Validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// envBool returns true if the environment variable is set and non-empty.
// Common truthy values like "1", "true", "yes" all return true.
// An empty string or unset variable returns false.
func envBool(name string) bool {
	val := os.Getenv(name)
	if val == "" {
		return false
	}
	// Any non-empty value is considered true
	// This matches common Unix conventions (e.g., TERM, NO_COLOR)
	return true
}

// mergeConfigs merges two configs, with the second overriding the first
func mergeConfigs(base, override *Config) *Config {
	result := *base

	if override.ProjectName != "" {
		result.ProjectName = override.ProjectName
	}
	if override.Alias != "" {
		result.Alias = override.Alias
	}
	if override.ProjectsDir != "" {
		result.ProjectsDir = override.ProjectsDir
	}
	if override.DefaultBranch != "" {
		result.DefaultBranch = override.DefaultBranch
	}
	if override.Switch.DirtyHandling != "" {
		result.Switch.DirtyHandling = override.Switch.DirtyHandling
	}
	if override.Naming.Pattern != "" {
		result.Naming.Pattern = override.Naming.Pattern
	}
	if override.Tmux.Prefix != "" {
		result.Tmux.Prefix = override.Tmux.Prefix
	}
	// Merge plugin configs - only override if explicitly set (non-nil)
	if override.Plugins.Docker.Enabled != nil {
		result.Plugins.Docker.Enabled = override.Plugins.Docker.Enabled
	}
	if override.Plugins.Docker.AutoStart != nil {
		result.Plugins.Docker.AutoStart = override.Plugins.Docker.AutoStart
	}
	if override.Plugins.Docker.AutoStop != nil {
		result.Plugins.Docker.AutoStop = override.Plugins.Docker.AutoStop
	}

	// Merge protection config
	if len(override.Protection.Protected) > 0 {
		result.Protection.Protected = override.Protection.Protected
	}
	if len(override.Protection.Immutable) > 0 {
		result.Protection.Immutable = override.Protection.Immutable
	}

	return &result
}

// IsProtected checks if a worktree is in the protected list
func (c *Config) IsProtected(name string) bool {
	for _, p := range c.Protection.Protected {
		if p == name {
			return true
		}
	}
	return false
}

// IsImmutable checks if a worktree is in the immutable list
func (c *Config) IsImmutable(name string) bool {
	for _, p := range c.Protection.Immutable {
		if p == name {
			return true
		}
	}
	return false
}

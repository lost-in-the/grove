package config

import (
	"os"
	"path/filepath"
)

// Config represents the complete grove configuration
type Config struct {
	Alias         string        `toml:"alias"`
	ProjectsDir   string        `toml:"projects_dir"`
	DefaultBranch string        `toml:"default_base_branch"`
	Switch        SwitchConfig  `toml:"switch"`
	Naming        NamingConfig  `toml:"naming"`
	Plugins       PluginsConfig `toml:"plugins"`
}

// SwitchConfig controls worktree switching behavior
type SwitchConfig struct {
	DirtyHandling string `toml:"dirty_handling"` // auto-stash, prompt, refuse
}

// NamingConfig controls worktree naming conventions
type NamingConfig struct {
	Pattern string `toml:"pattern"` // Pattern for naming worktrees
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
// It starts with defaults, then loads global config, then project config
func Load() (*Config, error) {
	cfg := LoadDefaults()

	globalPath, projectPath, err := GetConfigPaths()
	if err != nil {
		return cfg, err
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

	// Validate the final configuration
	if err := Validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// mergeConfigs merges two configs, with the second overriding the first
func mergeConfigs(base, override *Config) *Config {
	result := *base

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

	return &result
}

package config

import (
	"os"
	"path/filepath"
	"strings"
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
	TUI           TUIConfig        `toml:"tui"`

	// Runtime settings (from env vars, not persisted)
	NoColor        bool `toml:"-"` // GROVE_NO_COLOR - disable colored output
	Debug          bool `toml:"-"` // GROVE_DEBUG - enable debug logging
	NonInteractive bool `toml:"-"` // GROVE_NONINTERACTIVE - disable prompts
}

// TUIConfig controls TUI behavior preferences
type TUIConfig struct {
	SkipBranchNotice    *bool  `toml:"skip_branch_notice"`     // Don't show "branch exists" notice
	DefaultBranchAction string `toml:"default_branch_action"`  // "split" or "fork" — used when notice is skipped
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

	// Merge TUI config
	if override.TUI.SkipBranchNotice != nil {
		result.TUI.SkipBranchNotice = override.TUI.SkipBranchNotice
	}
	if override.TUI.DefaultBranchAction != "" {
		result.TUI.DefaultBranchAction = override.TUI.DefaultBranchAction
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

// SetProjectConfigValues updates specific key-value pairs in .grove/config.toml
// without disturbing the rest of the file (comments, ordering, unrelated keys).
// Keys use dotted notation: "tui.skip_branch_notice" updates skip_branch_notice
// under the [tui] section. Keys without a dot update top-level values.
func SetProjectConfigValues(updates map[string]string) error {
	_, projectPath, err := GetConfigPaths()
	if err != nil {
		return err
	}

	dir := filepath.Dir(projectPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	// Read existing file or start empty
	data, err := os.ReadFile(projectPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	lines := []string{}
	if len(data) > 0 {
		lines = strings.Split(string(data), "\n")
	}

	for key, val := range updates {
		section, field := splitKey(key)
		lines = setValueInLines(lines, section, field, val)
	}

	content := strings.Join(lines, "\n")
	// Ensure file ends with a newline
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return os.WriteFile(projectPath, []byte(content), 0o644)
}

// splitKey splits "section.field" into ("section", "field").
// A key without a dot returns ("", key) for top-level fields.
func splitKey(key string) (string, string) {
	if i := strings.IndexByte(key, '.'); i >= 0 {
		return key[:i], key[i+1:]
	}
	return "", key
}

// setValueInLines inserts or replaces a key=value in the given lines.
// If section is empty, the key is placed before any section header (top-level).
func setValueInLines(lines []string, section, field, value string) []string {
	formattedLine := field + " = " + value

	if section == "" {
		// Top-level key: find existing line or insert before first section header
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, field+" =") || strings.HasPrefix(trimmed, field+"=") {
				lines[i] = formattedLine
				return lines
			}
		}
		// Insert before first section header
		for i, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "[") {
				inserted := make([]string, 0, len(lines)+1)
				inserted = append(inserted, lines[:i]...)
				inserted = append(inserted, formattedLine)
				inserted = append(inserted, lines[i:]...)
				return inserted
			}
		}
		// No sections exist, just append
		return append(lines, formattedLine)
	}

	// Find the section header
	sectionHeader := "[" + section + "]"
	sectionStart := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == sectionHeader {
			sectionStart = i
			break
		}
	}

	if sectionStart == -1 {
		// Section doesn't exist — append it
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		lines = append(lines, sectionHeader)
		lines = append(lines, formattedLine)
		return lines
	}

	// Find the key within the section (between sectionStart+1 and next section or EOF)
	sectionEnd := len(lines)
	for i := sectionStart + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "[") && strings.Contains(trimmed, "]") {
			sectionEnd = i
			break
		}
	}

	for i := sectionStart + 1; i < sectionEnd; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, field+" =") || strings.HasPrefix(trimmed, field+"=") {
			lines[i] = formattedLine
			return lines
		}
	}

	// Key not found in section — insert after header
	inserted := make([]string, 0, len(lines)+1)
	inserted = append(inserted, lines[:sectionStart+1]...)
	inserted = append(inserted, formattedLine)
	inserted = append(inserted, lines[sectionStart+1:]...)
	return inserted
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

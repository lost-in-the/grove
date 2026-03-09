package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TestConfig controls test command behavior
type TestConfig struct {
	Command string `toml:"command"`
	Service string `toml:"service"`
}

// SessionConfig controls session command behavior for grove open
type SessionConfig struct {
	Command     string `toml:"command"`
	Popup       *bool  `toml:"popup"`
	PopupWidth  string `toml:"popup_width"`
	PopupHeight string `toml:"popup_height"`
}

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
	Test          TestConfig       `toml:"test"`
	Session       SessionConfig    `toml:"session"`

	// Runtime settings (from env vars, not persisted)
	NoColor        bool `toml:"-"` // GROVE_NO_COLOR - disable colored output
	Debug          bool `toml:"-"` // GROVE_DEBUG - enable debug logging
	NonInteractive bool `toml:"-"` // GROVE_NONINTERACTIVE - disable prompts
	AgentMode      bool `toml:"-"` // GROVE_AGENT_MODE - agent isolation mode
}

// TUIConfig controls TUI behavior preferences
type TUIConfig struct {
	SkipBranchNotice       *bool  `toml:"skip_branch_notice"`        // Don't show "branch exists" notice
	DefaultBranchAction    string `toml:"default_branch_action"`     // "split" or "fork" — used when notice is skipped
	WorktreeNameFromBranch string `toml:"worktree_name_from_branch"` // "last_segment" (default) — how to derive name from branch
	CompactList            *bool  `toml:"compact_list"`              // Use single-line list items (V1 delegate)
}

// ProtectionConfig controls worktree protection settings
type ProtectionConfig struct {
	Protected []string `toml:"protected"` // Cannot rm without --force --unprotect
	Immutable []string `toml:"immutable"` // Cannot apply changes to
}

// SwitchConfig controls worktree switching behavior
type SwitchConfig struct {
	DirtyHandling   string `toml:"dirty_handling"`   // auto-stash, prompt, refuse
	ContainerSwitch string `toml:"container_switch"` // auto, prompt, off
}

// NamingConfig controls worktree naming conventions
type NamingConfig struct {
	Pattern string `toml:"pattern"` // Pattern for naming worktrees
}

// TmuxConfig controls tmux session behavior
type TmuxConfig struct {
	Mode     string `toml:"mode"`      // auto, manual, off
	Prefix   string `toml:"prefix"`    // Prefix for tmux session names
	OnSwitch string `toml:"on_switch"` // reset (default), warn, ignore — directory drift behavior
}

// PluginsConfig controls plugin behavior
type PluginsConfig struct {
	Docker DockerPluginConfig `toml:"docker"`
}

// DockerPluginConfig controls docker plugin behavior
type DockerPluginConfig struct {
	Enabled   *bool                  `toml:"enabled"`
	AutoStart *bool                  `toml:"auto_start"`
	AutoStop  *bool                  `toml:"auto_stop"`
	AutoUp    *bool                  `toml:"auto_up"`
	Mode      string                 `toml:"mode"` // "" or "local" = local compose, "external" = external compose
	External  *ExternalComposeConfig `toml:"external"`
}

// ExternalComposeConfig configures external Docker Compose mode where services
// are defined in a shared compose setup outside the project directory.
type ExternalComposeConfig struct {
	Path         string            `toml:"path"`          // Path to external compose directory
	EnvVar       string            `toml:"env_var"`       // Environment variable name (e.g., "APP_DIR")
	EnvFile      string            `toml:"env_file"`      // File to write env vars to (default: ".env")
	Services     []string          `toml:"services"`      // Service names to manage
	CopyFiles    []string          `toml:"copy_files"`    // Files to copy from main on worktree create
	SymlinkFiles []string          `toml:"symlink_files"` // Files to symlink from main on create
	SymlinkDirs  []string          `toml:"symlink_dirs"`  // Directories to symlink from main on create
	Agent        *AgentStackConfig `toml:"agent"`         // Optional agent stack configuration
}

// EnvFileName returns the configured env file name, defaulting to ".env".
func (e *ExternalComposeConfig) EnvFileName() string {
	if e.EnvFile != "" {
		return e.EnvFile
	}
	return ".env"
}

// AgentStackConfig configures agent stack support for external compose mode.
type AgentStackConfig struct {
	Enabled      *bool    `toml:"enabled"`
	MaxSlots     int      `toml:"max_slots"`
	Services     []string `toml:"services"`
	TemplatePath string   `toml:"template_path"`
	URLPattern   string   `toml:"url_pattern"`
	Network      string   `toml:"network"` // External Docker network that must exist for agent stacks
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

	return loadFromPaths(cfg, globalPath, projectPath)
}

// LoadFromGroveDir loads config using an explicit .grove directory path
// instead of discovering it from cwd. This is needed when running from
// a secondary worktree where .grove/ doesn't exist locally.
func LoadFromGroveDir(groveDir string) (*Config, error) {
	cfg := LoadDefaults()

	// Check for broken config symlink before attempting to load
	projectPath := filepath.Join(groveDir, "config.toml")
	if target, err := os.Readlink(projectPath); err == nil {
		// It's a symlink — verify the target exists
		if _, statErr := os.Stat(projectPath); statErr != nil {
			return nil, fmt.Errorf("config symlink broken: %s → %s (target missing)", projectPath, target)
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return cfg, err
	}
	globalPath := filepath.Join(homeDir, ".config", "grove", "config.toml")

	return loadFromPaths(cfg, globalPath, projectPath)
}

// loadFromPaths is the shared config loading logic.
func loadFromPaths(cfg *Config, globalPath, projectPath string) (*Config, error) {

	// GROVE_CONFIG overrides the global config path
	if envConfig := os.Getenv("GROVE_CONFIG"); envConfig != "" {
		globalPath = envConfig
	}

	// Load global config if it exists
	globalCfg, err := LoadConfigFromPath(globalPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if globalCfg != nil {
		cfg = mergeConfigs(cfg, globalCfg)
	}

	// Load project config if it exists (overrides global)
	projectCfg, err := LoadConfigFromPath(projectPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if projectCfg != nil {
		cfg = mergeConfigs(cfg, projectCfg)
	}

	// Apply environment variable overrides for runtime settings
	cfg.NoColor = envBool("GROVE_NO_COLOR")
	cfg.Debug = envBool("GROVE_DEBUG")
	cfg.NonInteractive = envBool("GROVE_NONINTERACTIVE")
	cfg.AgentMode = envBool("GROVE_AGENT_MODE")

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
	if override.Switch.ContainerSwitch != "" {
		result.Switch.ContainerSwitch = override.Switch.ContainerSwitch
	}
	if override.Naming.Pattern != "" {
		result.Naming.Pattern = override.Naming.Pattern
	}
	if override.Tmux.Mode != "" {
		result.Tmux.Mode = override.Tmux.Mode
	}
	if override.Tmux.Prefix != "" {
		result.Tmux.Prefix = override.Tmux.Prefix
	}
	if override.Tmux.OnSwitch != "" {
		result.Tmux.OnSwitch = override.Tmux.OnSwitch
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
	if override.Plugins.Docker.AutoUp != nil {
		result.Plugins.Docker.AutoUp = override.Plugins.Docker.AutoUp
	}
	if override.Plugins.Docker.Mode != "" {
		result.Plugins.Docker.Mode = override.Plugins.Docker.Mode
	}
	if override.Plugins.Docker.External != nil {
		result.Plugins.Docker.External = override.Plugins.Docker.External
	}

	// Merge TUI config
	if override.TUI.SkipBranchNotice != nil {
		result.TUI.SkipBranchNotice = override.TUI.SkipBranchNotice
	}
	if override.TUI.DefaultBranchAction != "" {
		result.TUI.DefaultBranchAction = override.TUI.DefaultBranchAction
	}
	if override.TUI.WorktreeNameFromBranch != "" {
		result.TUI.WorktreeNameFromBranch = override.TUI.WorktreeNameFromBranch
	}
	if override.TUI.CompactList != nil {
		result.TUI.CompactList = override.TUI.CompactList
	}

	// Merge protection config - union semantics (global protections always apply)
	if len(override.Protection.Protected) > 0 {
		result.Protection.Protected = deduplicatedUnion(
			result.Protection.Protected,
			override.Protection.Protected,
		)
	}
	if len(override.Protection.Immutable) > 0 {
		result.Protection.Immutable = deduplicatedUnion(
			result.Protection.Immutable,
			override.Protection.Immutable,
		)
	}

	// Merge test config
	if override.Test.Command != "" {
		result.Test.Command = override.Test.Command
	}
	if override.Test.Service != "" {
		result.Test.Service = override.Test.Service
	}

	// Merge session config
	if override.Session.Command != "" {
		result.Session.Command = override.Session.Command
	}
	if override.Session.Popup != nil {
		result.Session.Popup = override.Session.Popup
	}
	if override.Session.PopupWidth != "" {
		result.Session.PopupWidth = override.Session.PopupWidth
	}
	if override.Session.PopupHeight != "" {
		result.Session.PopupHeight = override.Session.PopupHeight
	}

	return &result
}

// deduplicatedUnion merges two string slices, preserving order and removing duplicates.
func deduplicatedUnion(base, override []string) []string {
	seen := make(map[string]bool, len(base))
	result := make([]string, 0, len(base)+len(override))
	for _, s := range base {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	for _, s := range override {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
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

// IsExternalDockerMode returns true if the docker plugin is configured for external compose mode.
func (c *Config) IsExternalDockerMode() bool {
	return c.Plugins.Docker.Mode == "external"
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

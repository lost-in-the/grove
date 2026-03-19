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
	Mode        string `toml:"mode"`         // auto, manual, off
	Prefix      string `toml:"prefix"`       // Prefix for tmux session names
	OnSwitch    string `toml:"on_switch"`    // reset (default), warn, ignore — directory drift behavior
	ControlMode *bool  `toml:"control_mode"` // nil/true = auto-detect iTerm2 for tmux -CC, false = disabled
}

// PluginsConfig controls plugin behavior
type PluginsConfig struct {
	Docker DockerPluginConfig `toml:"docker"`
	Claude ClaudePluginConfig `toml:"claude"`
}

// ClaudePluginConfig controls the Claude Code devcontainer plugin behavior
type ClaudePluginConfig struct {
	Enabled            *bool                     `toml:"enabled"`
	AutoStart          *bool                     `toml:"auto_start"`
	SkipPermissions    *bool                     `toml:"skip_permissions"`
	Prompt             string                    `toml:"prompt"`
	InjectGroveContext *bool                     `toml:"inject_grove_context"`
	Devcontainer       *ClaudeDevcontainerConfig `toml:"devcontainer"`
	Permissions        *ClaudePermissionsConfig  `toml:"permissions"`
}

// ClaudeDevcontainerConfig configures the devcontainer sandbox
type ClaudeDevcontainerConfig struct {
	Enabled        *bool    `toml:"enabled"`
	Firewall       *bool    `toml:"firewall"`
	AllowedDomains []string `toml:"allowed_domains"`
}

// ClaudePermissionsConfig defines what the agent can do inside the container
type ClaudePermissionsConfig struct {
	AllowedTools []string `toml:"allowed_tools"`
	AllowedMCPs  []string `toml:"allowed_mcps"`
	MaxTurns     int      `toml:"max_turns"`
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

	mergeTopLevel(&result, override)
	mergeSwitchConfig(&result.Switch, &override.Switch)
	mergeTmuxConfig(&result.Tmux, &override.Tmux)
	mergeDockerConfig(&result.Plugins.Docker, &override.Plugins.Docker)
	mergeClaudeConfig(&result.Plugins.Claude, &override.Plugins.Claude)
	mergeTUIConfig(&result.TUI, &override.TUI)
	mergeProtectionConfig(&result.Protection, &override.Protection)
	mergeTestConfig(&result.Test, &override.Test)
	mergeSessionConfig(&result.Session, &override.Session)

	return &result
}

func mergeTopLevel(result *Config, override *Config) {
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
	if override.Naming.Pattern != "" {
		result.Naming.Pattern = override.Naming.Pattern
	}
}

func mergeSwitchConfig(result, override *SwitchConfig) {
	if override.DirtyHandling != "" {
		result.DirtyHandling = override.DirtyHandling
	}
	if override.ContainerSwitch != "" {
		result.ContainerSwitch = override.ContainerSwitch
	}
}

func mergeTmuxConfig(result, override *TmuxConfig) {
	if override.Mode != "" {
		result.Mode = override.Mode
	}
	if override.Prefix != "" {
		result.Prefix = override.Prefix
	}
	if override.OnSwitch != "" {
		result.OnSwitch = override.OnSwitch
	}
	if override.ControlMode != nil {
		result.ControlMode = override.ControlMode
	}
}

func mergeDockerConfig(result, override *DockerPluginConfig) {
	if override.Enabled != nil {
		result.Enabled = override.Enabled
	}
	if override.AutoStart != nil {
		result.AutoStart = override.AutoStart
	}
	if override.AutoStop != nil {
		result.AutoStop = override.AutoStop
	}
	if override.AutoUp != nil {
		result.AutoUp = override.AutoUp
	}
	if override.Mode != "" {
		result.Mode = override.Mode
	}
	if override.External != nil {
		result.External = override.External
	}
}

func mergeClaudeConfig(result, override *ClaudePluginConfig) {
	if override.Enabled != nil {
		result.Enabled = override.Enabled
	}
	if override.AutoStart != nil {
		result.AutoStart = override.AutoStart
	}
	if override.SkipPermissions != nil {
		result.SkipPermissions = override.SkipPermissions
	}
	if override.Prompt != "" {
		result.Prompt = override.Prompt
	}
	if override.InjectGroveContext != nil {
		result.InjectGroveContext = override.InjectGroveContext
	}
	if override.Devcontainer != nil {
		result.Devcontainer = override.Devcontainer
	}
	if override.Permissions != nil {
		result.Permissions = override.Permissions
	}
}

func mergeTUIConfig(result, override *TUIConfig) {
	if override.SkipBranchNotice != nil {
		result.SkipBranchNotice = override.SkipBranchNotice
	}
	if override.DefaultBranchAction != "" {
		result.DefaultBranchAction = override.DefaultBranchAction
	}
	if override.WorktreeNameFromBranch != "" {
		result.WorktreeNameFromBranch = override.WorktreeNameFromBranch
	}
	if override.CompactList != nil {
		result.CompactList = override.CompactList
	}
}

func mergeProtectionConfig(result, override *ProtectionConfig) {
	if len(override.Protected) > 0 {
		result.Protected = deduplicatedUnion(result.Protected, override.Protected)
	}
	if len(override.Immutable) > 0 {
		result.Immutable = deduplicatedUnion(result.Immutable, override.Immutable)
	}
}

func mergeTestConfig(result, override *TestConfig) {
	if override.Command != "" {
		result.Command = override.Command
	}
	if override.Service != "" {
		result.Service = override.Service
	}
}

func mergeSessionConfig(result, override *SessionConfig) {
	if override.Command != "" {
		result.Command = override.Command
	}
	if override.Popup != nil {
		result.Popup = override.Popup
	}
	if override.PopupWidth != "" {
		result.PopupWidth = override.PopupWidth
	}
	if override.PopupHeight != "" {
		result.PopupHeight = override.PopupHeight
	}
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

// lineMatchesField returns true if the trimmed line is an assignment for field.
func lineMatchesField(trimmed, field string) bool {
	return strings.HasPrefix(trimmed, field+" =") || strings.HasPrefix(trimmed, field+"=")
}

// setTopLevelValue handles the section=="" case for setValueInLines.
func setTopLevelValue(lines []string, field, formattedLine string) []string {
	for i, line := range lines {
		if lineMatchesField(strings.TrimSpace(line), field) {
			lines[i] = formattedLine
			return lines
		}
	}
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "[") {
			inserted := make([]string, 0, len(lines)+1)
			inserted = append(inserted, lines[:i]...)
			inserted = append(inserted, formattedLine)
			inserted = append(inserted, lines[i:]...)
			return inserted
		}
	}
	return append(lines, formattedLine)
}

// findSectionBounds returns the start index of the section header and the end
// index (exclusive) of the section body. Returns -1, -1 if the section is not found.
func findSectionBounds(lines []string, sectionHeader string) (start, end int) {
	start = -1
	for i, line := range lines {
		if strings.TrimSpace(line) == sectionHeader {
			start = i
			break
		}
	}
	if start == -1 {
		return -1, -1
	}
	end = len(lines)
	for i := start + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "[") && strings.Contains(trimmed, "]") {
			end = i
			break
		}
	}
	return start, end
}

// setSectionValue handles the section!="" case for setValueInLines.
func setSectionValue(lines []string, section, field, formattedLine string) []string {
	sectionHeader := "[" + section + "]"
	sectionStart, sectionEnd := findSectionBounds(lines, sectionHeader)

	if sectionStart == -1 {
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		lines = append(lines, sectionHeader)
		lines = append(lines, formattedLine)
		return lines
	}

	for i := sectionStart + 1; i < sectionEnd; i++ {
		if lineMatchesField(strings.TrimSpace(lines[i]), field) {
			lines[i] = formattedLine
			return lines
		}
	}

	inserted := make([]string, 0, len(lines)+1)
	inserted = append(inserted, lines[:sectionStart+1]...)
	inserted = append(inserted, formattedLine)
	inserted = append(inserted, lines[sectionStart+1:]...)
	return inserted
}

// setValueInLines inserts or replaces a key=value in the given lines.
// If section is empty, the key is placed before any section header (top-level).
func setValueInLines(lines []string, section, field, value string) []string {
	formattedLine := field + " = " + value
	if section == "" {
		return setTopLevelValue(lines, field, formattedLine)
	}
	return setSectionValue(lines, section, field, formattedLine)
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

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const dockerModeExternal = "external"

// DirtyHandlingValues lists every [switch] dirty_handling value, default first.
var DirtyHandlingValues = []string{"warn", "ignore", "prompt", "refuse", "auto-stash"}

// Validate checks if the configuration is valid
func Validate(cfg *Config) error {
	if cfg.DefaultBranch == "" {
		return fmt.Errorf("default_base_branch cannot be empty")
	}

	// Validate dirty handling options
	if cfg.Switch.DirtyHandling == "" {
		return fmt.Errorf("dirty_handling cannot be empty")
	}

	if !slices.Contains(DirtyHandlingValues, cfg.Switch.DirtyHandling) {
		return fmt.Errorf("dirty_handling must be one of: %s", strings.Join(DirtyHandlingValues, ", "))
	}

	// Validate container switch behavior
	validContainerSwitch := map[string]bool{
		"auto":   true,
		"prompt": true,
		"off":    true,
	}
	if cfg.Switch.ContainerSwitch != "" && !validContainerSwitch[cfg.Switch.ContainerSwitch] {
		return fmt.Errorf("switch.container_switch must be one of: auto, prompt, off (got %q)", cfg.Switch.ContainerSwitch)
	}

	// Validate multiplexer backend
	validMuxBackend := map[string]bool{
		"":      true,
		"auto":  true,
		"tmux":  true,
		"herdr": true,
		"off":   true,
	}
	if !validMuxBackend[cfg.Mux.Backend] {
		return fmt.Errorf("mux.backend must be one of: auto, tmux, herdr, off (got %q)", cfg.Mux.Backend)
	}

	validListSecondary := map[string]bool{"": true, "branch": true, "dir": true, "off": true}
	if !validListSecondary[cfg.TUI.ListSecondary] {
		return fmt.Errorf("tui.list_secondary must be one of: branch, dir, off (got %q)", cfg.TUI.ListSecondary)
	}

	_, oldOpenIn := cfg.OpenInRename()
	switch {
	case cfg.Session.OpenIn == "", cfg.Session.OpenIn == OpenInSession, cfg.Session.OpenIn == OpenInHere, oldOpenIn:
	default:
		return fmt.Errorf("session.open_in must be one of: session, here (got %q)", cfg.Session.OpenIn)
	}

	// Validate tmux mode
	validTmuxMode := map[string]bool{
		"auto":   true,
		"manual": true,
		"off":    true,
	}
	if cfg.Tmux.Mode != "" && !validTmuxMode[cfg.Tmux.Mode] {
		return fmt.Errorf("tmux.mode must be one of: auto, manual, off (got %q)", cfg.Tmux.Mode)
	}

	// Validate tmux on_switch behavior
	validOnSwitch := map[string]bool{
		"":       true,
		"reset":  true,
		"warn":   true,
		"ignore": true,
	}
	if !validOnSwitch[cfg.Tmux.OnSwitch] {
		return fmt.Errorf("tmux.on_switch must be one of: reset, warn, ignore (got %q)", cfg.Tmux.OnSwitch)
	}

	// Validate docker plugin mode
	if err := validateDockerPlugin(cfg); err != nil {
		return err
	}

	return nil
}

// validateDockerPlugin validates the docker plugin configuration
func validateDockerPlugin(cfg *Config) error {
	mode := cfg.Plugins.Docker.Mode
	if mode != "" && mode != "local" && mode != dockerModeExternal {
		return fmt.Errorf("plugins.docker.mode must be one of: local, external (got %q)", mode)
	}

	// Worktree file setup runs whenever [plugins.docker.external] exists,
	// whatever the mode, so its entries are checked here too.
	if ext := cfg.Plugins.Docker.External; ext != nil {
		if err := validateExternalFileEntries(ext); err != nil {
			return err
		}
	}

	if mode != dockerModeExternal {
		return nil
	}

	ext := cfg.Plugins.Docker.External
	if ext == nil {
		return fmt.Errorf("plugins.docker.external is required when mode is \"external\"")
	}

	if err := validateExternalRequiredFields(ext); err != nil {
		return err
	}

	if err := validateExternalPath(ext); err != nil {
		return err
	}

	return validateExternalAgent(ext)
}

// validateExternalRequiredFields checks that required fields are set for external docker mode
func validateExternalRequiredFields(ext *ExternalComposeConfig) error {
	if ext.Path == "" {
		return fmt.Errorf("plugins.docker.external.path is required for external mode")
	}
	if ext.EnvVar == "" {
		return fmt.Errorf("plugins.docker.external.env_var is required for external mode")
	}
	if len(ext.Services) == 0 {
		return fmt.Errorf("plugins.docker.external.services is required for external mode")
	}
	return nil
}

// validateExternalPath validates the external docker path exists and is a
// directory. In the standard load path ext.Path is already absolute
// (resolveProjectPaths runs before Validate); expandTildePath keeps Validate
// safe for raw, unresolved configs without duplicating the expansion logic.
func validateExternalPath(ext *ExternalComposeConfig) error {
	path, err := expandTildePath(ext.Path)
	if err != nil {
		return fmt.Errorf("plugins.docker.external.path: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("plugins.docker.external.path: %q does not exist", ext.Path)
	}
	if !info.IsDir() {
		return fmt.Errorf("plugins.docker.external.path: %q is not a directory", ext.Path)
	}
	return nil
}

// validateExternalAgent validates agent config when present and enabled
func validateExternalAgent(ext *ExternalComposeConfig) error {
	if ext.Agent == nil || ext.Agent.Enabled == nil || !*ext.Agent.Enabled {
		return nil
	}
	if len(ext.Agent.Services) == 0 {
		return fmt.Errorf("plugins.docker.external.agent.services is required when agent mode is enabled")
	}
	if ext.Agent.TemplatePath == "" {
		return fmt.Errorf("plugins.docker.external.agent.template_path is required when agent mode is enabled")
	}
	for i, overlay := range ext.Agent.TemplateOverlays {
		if overlay == "" {
			return fmt.Errorf("plugins.docker.external.agent.template_overlays[%d] must be a non-empty path", i)
		}
	}
	return nil
}

// validateExternalFileEntries rejects copy_files, symlink_files, and
// symlink_dirs entries that file setup would only fail on later: empty,
// absolute, or escaping paths, and (for the two file lists) glob patterns,
// which are treated as literal names and never expanded.
func validateExternalFileEntries(ext *ExternalComposeConfig) error {
	lists := []struct {
		key     string
		entries []string
		globs   bool
	}{
		{"copy_files", ext.CopyFiles, true},
		{"symlink_files", ext.SymlinkFiles, true},
		{"symlink_dirs", ext.SymlinkDirs, false},
	}
	for _, l := range lists {
		for i, entry := range l.entries {
			where := fmt.Sprintf("plugins.docker.external.%s[%d]", l.key, i)
			switch {
			case entry == "":
				return fmt.Errorf("%s must not be empty", where)
			case filepath.IsAbs(entry):
				return fmt.Errorf("%s: %q must be a relative path", where, entry)
			case strings.HasPrefix(filepath.Clean(entry), ".."):
				return fmt.Errorf("%s: %q must stay inside the worktree", where, entry)
			case l.globs && strings.ContainsAny(entry, "*?["):
				return fmt.Errorf("%s: %q has a glob character; globs are not supported, list each file", where, entry)
			}
		}
	}
	return nil
}

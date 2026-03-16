package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const dockerModeExternal = "external"

// Validate checks if the configuration is valid
func Validate(cfg *Config) error {
	if cfg.Alias == "" {
		return fmt.Errorf("alias cannot be empty")
	}

	if cfg.DefaultBranch == "" {
		return fmt.Errorf("default_base_branch cannot be empty")
	}

	// Validate dirty handling options
	if cfg.Switch.DirtyHandling == "" {
		return fmt.Errorf("dirty_handling cannot be empty")
	}

	validDirtyHandling := map[string]bool{
		"auto-stash": true,
		"prompt":     true,
		"refuse":     true,
	}

	if !validDirtyHandling[cfg.Switch.DirtyHandling] {
		return fmt.Errorf("dirty_handling must be one of: auto-stash, prompt, refuse")
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

// validateExternalPath validates the external docker path exists and is a directory
func validateExternalPath(ext *ExternalComposeConfig) error {
	path := ext.Path
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("plugins.docker.external.path: failed to expand ~: %w", err)
		}
		path = filepath.Join(home, path[2:])
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
	return nil
}

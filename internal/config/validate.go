package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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

	// Validate tmux mode
	validTmuxMode := map[string]bool{
		"auto":   true,
		"manual": true,
		"off":    true,
	}
	if cfg.Tmux.Mode != "" && !validTmuxMode[cfg.Tmux.Mode] {
		return fmt.Errorf("tmux.mode must be one of: auto, manual, off (got %q)", cfg.Tmux.Mode)
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
	if mode != "" && mode != "local" && mode != "external" {
		return fmt.Errorf("plugins.docker.mode must be one of: local, external (got %q)", mode)
	}

	if mode == "external" {
		ext := cfg.Plugins.Docker.External
		if ext == nil {
			return fmt.Errorf("plugins.docker.external is required when mode is \"external\"")
		}
		if ext.Path == "" {
			return fmt.Errorf("plugins.docker.external.path is required for external mode")
		}
		if ext.EnvVar == "" {
			return fmt.Errorf("plugins.docker.external.env_var is required for external mode")
		}
		if len(ext.Services) == 0 {
			return fmt.Errorf("plugins.docker.external.services is required for external mode")
		}

		// Validate path exists (expand ~)
		path := ext.Path
		if strings.HasPrefix(path, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("plugins.docker.external.path: failed to expand ~: %w", err)
			}
			path = filepath.Join(home, path[2:])
		}
		if info, err := os.Stat(path); err != nil {
			return fmt.Errorf("plugins.docker.external.path: %q does not exist", ext.Path)
		} else if !info.IsDir() {
			return fmt.Errorf("plugins.docker.external.path: %q is not a directory", ext.Path)
		}
	}

	return nil
}

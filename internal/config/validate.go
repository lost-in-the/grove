package config

import (
	"fmt"
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
	validDirtyHandling := map[string]bool{
		"auto-stash": true,
		"prompt":     true,
		"refuse":     true,
	}

	if !validDirtyHandling[cfg.Switch.DirtyHandling] {
		return fmt.Errorf("dirty_handling must be one of: auto-stash, prompt, refuse")
	}

	return nil
}

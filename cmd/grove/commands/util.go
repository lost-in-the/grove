package commands

import (
	"github.com/LeahArmstrong/grove-cli/internal/config"
)

// loadConfig loads the configuration with defaults if none exists
func loadConfig() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		// Return default config if loading fails
		return config.LoadDefaults(), nil
	}
	return cfg, nil
}

package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// LoadDefaults returns a Config with sensible default values
func LoadDefaults() *Config {
	homeDir, _ := os.UserHomeDir()
	projectsDir := filepath.Join(homeDir, "projects")

	return &Config{
		Alias:         "w",
		ProjectsDir:   projectsDir,
		DefaultBranch: "main",
		Switch: SwitchConfig{
			DirtyHandling: "prompt",
		},
		Naming: NamingConfig{
			Pattern: "{type}/{description}",
		},
		Tmux: TmuxConfig{
			Prefix: "grove-",
		},
	}
}

// LoadConfigFromPath loads configuration from a specific file path
func LoadConfigFromPath(path string) (*Config, error) {
	cfg := LoadDefaults()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

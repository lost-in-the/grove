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

	// Create default boolean values
	trueVal := true
	falseVal := false

	return &Config{
		ProjectsDir:   projectsDir,
		DefaultBranch: "main",
		Switch: SwitchConfig{
			DirtyHandling:   "prompt",
			ContainerSwitch: "auto",
		},
		Naming: NamingConfig{
			Pattern: "{project}-{name}",
		},
		Tmux: TmuxConfig{
			Mode:        "auto",
			ControlMode: &trueVal,
		},
		Plugins: PluginsConfig{
			Docker: DockerPluginConfig{
				Enabled:   &trueVal,
				AutoStart: &trueVal,
				AutoStop:  &falseVal,
			},
		},
		Session: SessionConfig{
			PopupWidth:  "80%",
			PopupHeight: "80%",
		},
	}
}

// LoadConfigFromPath parses a single config file into a zero-value Config.
//
// Fields the file does not mention stay at their zero values — deliberately
// NOT seeded from LoadDefaults(). mergeConfigs treats non-empty / non-nil
// fields as explicit overrides, so seeding defaults here would make every
// config file appear to set every field, silently resetting lower-priority
// layers' explicit settings back to the defaults. Defaults enter the merge
// chain exactly once, as the base config in Load/LoadFromGroveDir.
func LoadConfigFromPath(path string) (*Config, error) {
	cfg := &Config{}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

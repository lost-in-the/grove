package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/LeahArmstrong/grove-cli/internal/config"
	"github.com/LeahArmstrong/grove-cli/internal/exitcode"
	"github.com/LeahArmstrong/grove-cli/internal/grove"
	"github.com/spf13/cobra"
)

var (
	configEdit   bool
	configGlobal bool
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show or edit configuration",
	Long: `Display or edit grove configuration.

By default, shows merged configuration from global and project configs.
Use --global to work with the global config (~/.config/grove/config.toml).
Use --edit to open the config file in your $EDITOR.

Examples:
  grove config           # Show merged configuration
  grove config --edit    # Edit project config
  grove config --global  # Show global config only
  grove config --global --edit  # Edit global config`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Determine which config file to use
		var configPath string

		if configGlobal {
			// Global config: ~/.config/grove/config.toml
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}
			configPath = filepath.Join(homeDir, ".config", "grove", "config.toml")
		} else {
			// Project config: requires grove context
			groveDir, err := grove.FindRoot("")
			if err != nil || groveDir == "" {
				fmt.Fprintf(os.Stderr, "Error: not a grove project\n")
				fmt.Fprintf(os.Stderr, "Run 'grove setup' to initialize, or use --global for global config\n")
				os.Exit(exitcode.NotGroveProject)
			}
			configPath = filepath.Join(groveDir, "config.toml")
		}

		// Edit mode
		if configEdit {
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}

			// Ensure config file exists
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				// Create directory if needed
				dir := filepath.Dir(configPath)
				if err := os.MkdirAll(dir, 0755); err != nil {
					return fmt.Errorf("failed to create config directory: %w", err)
				}
				// Create empty config file
				if err := os.WriteFile(configPath, []byte("# Grove configuration\n"), 0644); err != nil {
					return fmt.Errorf("failed to create config file: %w", err)
				}
			}

			editorCmd := exec.Command(editor, configPath)
			editorCmd.Stdin = os.Stdin
			editorCmd.Stdout = os.Stdout
			editorCmd.Stderr = os.Stderr
			return editorCmd.Run()
		}

		// Show mode: display current configuration
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		fmt.Printf("Configuration")
		if configGlobal {
			fmt.Printf(" (global)")
		}
		fmt.Printf(":\n")
		fmt.Printf("  Config file: %s\n\n", configPath)

		fmt.Printf("General:\n")
		if cfg.ProjectName != "" {
			fmt.Printf("  project_name: %s\n", cfg.ProjectName)
		}
		fmt.Printf("  alias: %s\n", cfg.Alias)
		fmt.Printf("  projects_dir: %s\n", cfg.ProjectsDir)
		fmt.Printf("  default_base_branch: %s\n", cfg.DefaultBranch)

		fmt.Printf("\n[switch]:\n")
		fmt.Printf("  dirty_handling: %s\n", cfg.Switch.DirtyHandling)

		fmt.Printf("\n[naming]:\n")
		fmt.Printf("  pattern: %s\n", cfg.Naming.Pattern)

		fmt.Printf("\n[tmux]:\n")
		fmt.Printf("  prefix: %s\n", cfg.Tmux.Prefix)

		fmt.Printf("\n[plugins.docker]:\n")
		fmt.Printf("  enabled: %v\n", cfg.Plugins.Docker.Enabled)
		fmt.Printf("  auto_start: %v\n", cfg.Plugins.Docker.AutoStart)
		fmt.Printf("  auto_stop: %v\n", cfg.Plugins.Docker.AutoStop)

		return nil
	},
}

func init() {
	configCmd.Flags().BoolVarP(&configEdit, "edit", "e", false, "Open config file in $EDITOR")
	configCmd.Flags().BoolVarP(&configGlobal, "global", "g", false, "Use global config (~/.config/grove/config.toml)")
	rootCmd.AddCommand(configCmd)
}

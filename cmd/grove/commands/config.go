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
	configHooks  bool
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show or edit configuration",
	Long: `Display or edit grove configuration.

By default, shows merged configuration from global and project configs.
Use --global to work with the global config (~/.config/grove/config.toml).
Use --hooks to work with hooks config instead of main config.
Use --edit to open the config file in your $EDITOR.

Examples:
  grove config                    # Show merged configuration
  grove config --edit             # Edit project config
  grove config --global           # Show global config only
  grove config --global --edit    # Edit global config
  grove config --hooks            # Show hooks configuration
  grove config --hooks --edit     # Edit project hooks config
  grove config --hooks -g -e      # Edit global hooks config`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Determine which config file to use
		var configPath string
		filename := "config.toml"
		if configHooks {
			filename = "hooks.toml"
		}

		if configGlobal {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}
			configPath = filepath.Join(homeDir, ".config", "grove", filename)
		} else {
			groveDir, err := grove.FindRoot("")
			if err != nil || groveDir == "" {
				fmt.Fprintf(os.Stderr, "Error: not a grove project\n")
				fmt.Fprintf(os.Stderr, "Run 'grove init' to initialize, or use --global for global config\n")
				os.Exit(exitcode.NotGroveProject)
			}
			configPath = filepath.Join(groveDir, filename)
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

		// Show mode
		if configHooks {
			return showHooksConfig(configPath)
		}

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

func showHooksConfig(configPath string) error {
	fmt.Printf("Hooks configuration:\n")
	fmt.Printf("  File: %s\n\n", configPath)

	content, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("  No hooks configured.")
			fmt.Println("  Run 'grove init' to auto-detect and generate hooks,")
			fmt.Println("  or 'grove config --hooks -e' to create manually.")
			return nil
		}
		return fmt.Errorf("failed to read hooks config: %w", err)
	}

	fmt.Print(string(content))
	return nil
}

func init() {
	configCmd.Flags().BoolVarP(&configEdit, "edit", "e", false, "Open config file in $EDITOR")
	configCmd.Flags().BoolVarP(&configGlobal, "global", "g", false, "Use global config (~/.config/grove/config.toml)")
	configCmd.Flags().BoolVar(&configHooks, "hooks", false, "Work with hooks config instead of main config")
	rootCmd.AddCommand(configCmd)
}

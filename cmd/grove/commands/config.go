package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/exitcode"
	"github.com/lost-in-the/grove/internal/grove"
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

		w := cli.NewStdout()

		title := "Configuration"
		if configGlobal {
			title += " (global)"
		}
		cli.Header(w, "%s", title)
		cli.Label(w, "  Config file:", configPath)
		_, _ = fmt.Fprintln(w)

		cli.Bold(w, "General:")
		if cfg.ProjectName != "" {
			cli.Label(w, "  project_name:", cfg.ProjectName)
		}
		cli.Label(w, "  alias:", cfg.Alias)
		cli.Label(w, "  projects_dir:", cfg.ProjectsDir)
		cli.Label(w, "  default_base_branch:", cfg.DefaultBranch)

		_, _ = fmt.Fprintln(w)
		cli.Bold(w, "[switch]:")
		cli.Label(w, "  dirty_handling:", cfg.Switch.DirtyHandling)

		_, _ = fmt.Fprintln(w)
		cli.Bold(w, "[naming]:")
		cli.Label(w, "  pattern:", cfg.Naming.Pattern)

		_, _ = fmt.Fprintln(w)
		cli.Bold(w, "[tmux]:")
		cli.Label(w, "  mode:", cfg.Tmux.Mode)
		cli.Label(w, "  prefix:", cfg.Tmux.Prefix)

		_, _ = fmt.Fprintln(w)
		cli.Bold(w, "[plugins.docker]:")
		cli.Label(w, "  enabled:", formatBoolPtr(cfg.Plugins.Docker.Enabled, "true"))
		cli.Label(w, "  auto_start:", formatBoolPtr(cfg.Plugins.Docker.AutoStart, "true"))
		cli.Label(w, "  auto_stop:", formatBoolPtr(cfg.Plugins.Docker.AutoStop, "false"))

		return nil
	},
}

// formatBoolPtr safely formats a *bool, returning the default if nil.
func formatBoolPtr(b *bool, fallback string) string {
	if b == nil {
		return fallback
	}
	return strconv.FormatBool(*b)
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

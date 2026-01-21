package commands

import (
	"fmt"
	"os"

	"github.com/LeahArmstrong/grove-cli/plugins/docker"
	"github.com/spf13/cobra"
)

var (
	upDetach bool
)

func init() {
	upCmd.Flags().BoolVarP(&upDetach, "detach", "d", true, "Run containers in the background")
	rootCmd.AddCommand(upCmd)
}

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start containers for current worktree",
	Long: `Start Docker containers for the current worktree.

This command looks for a docker-compose.yml file in the current directory
and starts all services defined in it. By default, containers run in detached
mode (background).

Examples:
  grove up              # Start containers in detached mode
  grove up --detach=false  # Start containers in foreground
  w up                  # Using alias`,
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		// Get current directory (docker-compose works in cwd)
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		// Create docker plugin
		plugin := docker.New()
		cfg, _ := loadConfig()
		if err := plugin.Init(cfg); err != nil {
			return fmt.Errorf("failed to initialize docker plugin: %w", err)
		}

		// Start containers
		if err := plugin.Up(cwd, upDetach); err != nil {
			return fmt.Errorf("failed to start containers: %w", err)
		}

		if upDetach {
			fmt.Println("Containers started in detached mode")
		}
		return nil
	}),
}

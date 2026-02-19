package commands

import (
	"fmt"
	"os"

	"github.com/LeahArmstrong/grove-cli/plugins/docker"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(downCmd)
}

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop containers for current worktree",
	Long: `Stop Docker containers for the current worktree.

This command looks for a docker-compose.yml file in the current directory
and stops all services defined in it.

Examples:
  grove down   # Stop all containers
  w down       # Using alias`,
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		// Get current directory (docker-compose works in cwd)
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		// Create docker plugin
		plugin := docker.New()
		if err := plugin.Init(ctx.Config); err != nil {
			return fmt.Errorf("failed to initialize docker plugin: %w", err)
		}

		// Stop containers
		if err := plugin.Down(cwd); err != nil {
			return fmt.Errorf("failed to stop containers: %w", err)
		}

		fmt.Println("Containers stopped")
		return nil
	}),
}

package commands

import (
	"fmt"
	"os"

	"github.com/LeahArmstrong/grove-cli/plugins/docker"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(restartCmd)
}

var restartCmd = &cobra.Command{
	Use:   "restart [service]",
	Short: "Restart container services",
	Long: `Restart Docker container services for the current worktree.

If no service is specified, restarts all services.

Examples:
  grove restart        # Restart all services
  grove restart web    # Restart 'web' service only
  w restart db         # Using alias`,
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		// Get current directory (docker-compose works in cwd)
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		// Get service name if provided
		service := ""
		if len(args) > 0 {
			service = args[0]
		}

		// Create docker plugin
		plugin := docker.New()
		if err := plugin.Init(ctx.Config); err != nil {
			return fmt.Errorf("failed to initialize docker plugin: %w", err)
		}

		// Restart service(s)
		if err := plugin.Restart(cwd, service); err != nil {
			return fmt.Errorf("failed to restart: %w", err)
		}

		if service != "" {
			fmt.Printf("Service '%s' restarted\n", service)
		} else {
			fmt.Println("All services restarted")
		}
		return nil
	}),
}

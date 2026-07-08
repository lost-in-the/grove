package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/plugins/docker"
)

var (
	logsFollow bool
)

func init() {
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", true, "Follow log output")
	rootCmd.AddCommand(logsCmd)
}

var logsCmd = &cobra.Command{
	Use:     "logs [service]",
	Aliases: []string{"lo"},
	Short:   "View container logs",
	Long: `View logs from Docker containers for the current worktree.

If no service is specified, shows logs from all services.
By default, follows log output (like tail -f).

If the worktree has an isolated stack running, logs are automatically
shown from that stack's containers.

Examples:
  grove logs           # Show logs from all services
  grove logs web       # Show logs from 'web' service only
  grove logs -f=false  # Show logs without following
  w logs db            # Using alias`,
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		// Resolve the worktree root — slot detection keys on the worktree
		// directory basename, so cwd must be normalized (running from a
		// subdirectory would otherwise tail the shared stack).
		root, err := currentWorktreeRoot(ctx)
		if err != nil {
			return err
		}

		// Get service name if provided
		service := ""
		if len(args) > 0 {
			service = args[0]
		}

		// Create docker plugin — auto-detect isolated stacks
		plugin := docker.New()
		if docker.HasActiveAgentSlot(ctx.Config, root) {
			plugin.SetIsolated(true)
		}
		if err := plugin.Init(ctx.Config); err != nil {
			return fmt.Errorf("failed to initialize docker plugin: %w", err)
		}

		// Show logs
		if err := plugin.Logs(root, service, logsFollow); err != nil {
			return fmt.Errorf("failed to show logs: %w", err)
		}

		return nil
	}),
}

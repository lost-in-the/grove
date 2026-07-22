package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/plugins/docker"
)

func init() {
	rootCmd.AddCommand(restartCmd)
}

var restartCmd = &cobra.Command{
	Use:     "kick [services...]",
	Aliases: []string{"restart", "k"},
	Short:   "Kick (restart) container services",
	Long: `Kick (restart) Docker container services for the current worktree.

If no service is specified, restarts all services. Multiple services may be
listed and are each restarted.

If the worktree has an isolated stack running, the restart targets
that stack's containers automatically.

Examples:
  grove kick           # Restart all services
  grove kick web       # Restart 'web' service only
  grove kick web db    # Restart 'web' and 'db'
  w kick db            # Using alias`,
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		w := cli.NewStdout()

		// Resolve the worktree root — slot detection keys on the worktree
		// directory basename, so cwd must be normalized (running from a
		// subdirectory would otherwise restart the shared stack).
		root, err := currentWorktreeRoot(ctx)
		if err != nil {
			return err
		}

		// Create docker plugin — auto-detect isolated stacks
		plugin := docker.New()
		if docker.HasActiveAgentSlot(ctx.Config, root) {
			plugin.SetIsolated(true)
		}
		if err := plugin.Init(ctx.Config); err != nil {
			return fmt.Errorf("failed to initialize docker plugin: %w", err)
		}

		// Restart service(s) — no spinner: docker compose writes its own progress
		stderr := cli.NewStderr()
		if len(args) == 0 {
			cli.Step(stderr, "Restarting containers...")
			if err := plugin.Restart(root, ""); err != nil {
				return fmt.Errorf("failed to restart: %w", err)
			}
			cli.Success(w, "All services restarted")
			return nil
		}

		// Every listed service is restarted; the first failure aborts so the
		// user isn't told a service restarted when a later one errored.
		for _, service := range args {
			cli.Step(stderr, "Restarting %s...", service)
			if err := plugin.Restart(root, service); err != nil {
				return fmt.Errorf("failed to restart %s: %w", service, err)
			}
			cli.Success(w, "Service '%s' restarted", service)
		}
		return nil
	}),
}

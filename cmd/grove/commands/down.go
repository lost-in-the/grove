package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/plugins/docker"
)

func init() {
	rootCmd.AddCommand(downCmd)
}

var downCmd = &cobra.Command{
	Use:     "down",
	Aliases: []string{"do"},
	Short:   "Stop containers for current worktree",
	Long: `Stop Docker containers for the current worktree.

This command looks for a docker-compose.yml file in the current directory
and stops all services defined in it.

If the worktree has an isolated stack running (started with grove up --isolated),
it is automatically detected and torn down.

Examples:
  grove down   # Stop all containers
  w down       # Using alias`,
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		w := cli.NewStdout()

		// Resolve the worktree root — slot detection keys on the worktree
		// directory basename, so cwd must be normalized (running from a
		// subdirectory would otherwise target the shared stack).
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

		// Stop containers — no spinner: docker compose writes its own progress
		stderr := cli.NewStderr()
		cli.Step(stderr, "Stopping containers...")
		if err := plugin.Down(root); err != nil {
			return fmt.Errorf("failed to stop containers: %w", err)
		}

		cli.Success(w, "Containers stopped")
		return nil
	}),
}

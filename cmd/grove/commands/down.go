package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LeahArmstrong/grove-cli/internal/cli"
	"github.com/LeahArmstrong/grove-cli/plugins/docker"
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

If the worktree has an isolated stack running (started with grove up --isolated),
it is automatically detected and torn down.

Examples:
  grove down   # Stop all containers
  w down       # Using alias`,
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		w := cli.NewStdout()

		// Get current directory (docker-compose works in cwd)
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		// Create docker plugin — auto-detect isolated stacks
		plugin := docker.New()
		if docker.HasActiveAgentSlot(ctx.Config, cwd) {
			plugin.SetIsolated(true)
		}
		if err := plugin.Init(ctx.Config); err != nil {
			return fmt.Errorf("failed to initialize docker plugin: %w", err)
		}

		// Stop containers — no spinner: docker compose writes its own progress
		stderr := cli.NewStderr()
		cli.Step(stderr, "Stopping containers...")
		if err := plugin.Down(cwd); err != nil {
			return fmt.Errorf("failed to stop containers: %w", err)
		}

		cli.Success(w, "Containers stopped")
		return nil
	}),
}

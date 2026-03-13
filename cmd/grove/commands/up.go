package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/plugins/docker"
)

var (
	upDetach   bool
	upIsolated bool
)

func init() {
	upCmd.Flags().BoolVarP(&upDetach, "detach", "d", true, "Run containers in the background")
	upCmd.Flags().BoolVar(&upIsolated, "isolated", false, "Start an independent stack with its own containers and database")
	rootCmd.AddCommand(upCmd)
}

var upCmd = &cobra.Command{
	Use:     "up",
	Aliases: []string{"u"},
	Short:   "Start containers for current worktree",
	Long: `Start Docker containers for the current worktree.

This command looks for a docker-compose.yml file in the current directory
and starts all services defined in it. By default, containers run in detached
mode (background).

Use --isolated to start a fully independent stack with its own containers,
database, and routing. Useful when you need multiple stacks running simultaneously.

Examples:
  grove up                 # Start containers in detached mode
  grove up --detach=false  # Start containers in foreground
  grove up --isolated      # Start an independent stack (allocates a slot)
  w up                     # Using alias`,
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		w := cli.NewStdout()
		stderr := cli.NewStderr()

		// Get current directory (docker-compose works in cwd)
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		// Create docker plugin
		plugin := docker.New()
		if upIsolated {
			plugin.SetIsolated(true)
		}
		if err := plugin.Init(ctx.Config); err != nil {
			return fmt.Errorf("failed to initialize docker plugin: %w", err)
		}

		if upIsolated && !plugin.IsIsolated() {
			return fmt.Errorf("--isolated requires agent stack configuration\n\nAdd to .grove/config.toml:\n\n  [plugins.docker.external.agent]\n  enabled = true\n  services = [\"app\"]\n  template_path = \"agent-stacks/template.yml\"")
		}

		// Start containers — no spinner here: docker compose writes its own
		// progress to stderr, and wrapping it in Bubbletea causes flashing.
		cli.Step(stderr, "Starting containers...")
		if err := plugin.Up(cwd, upDetach); err != nil {
			return fmt.Errorf("failed to start containers: %w", err)
		}
		if upDetach && !upIsolated {
			cli.Success(w, "Containers started")
		}

		return nil
	}),
}

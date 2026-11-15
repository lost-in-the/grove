package commands

import (
	"fmt"

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
	Short:   "Bring the stack to this worktree",
	Long: `Bring the stack to this worktree: the one you are in.

Local mode: looks for a docker-compose.yml file in the current directory and
starts all services defined in it.

External mode: puts the one shared stack on this worktree (it writes the env
file) and starts it. By default the stack runs in the background.

'grove to <name>' only moves you. Use 'grove up' after it, or 'grove to <name> --up'.

Use --isolated to start a fully independent stack with its own containers,
database, and routing. Useful when you need multiple stacks running simultaneously.

Examples:
  grove up                 # Bring the stack up in the background
  grove up --detach=false  # Bring the stack up in the foreground
  grove up --isolated      # Start an independent stack (allocates a slot)
  w up                     # Using alias`,
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		w := cli.NewStdout()
		stderr := cli.NewStderr()

		// Resolve the worktree root — isolated slot allocation keys on the
		// worktree directory basename, so cwd must be normalized (running
		// from a subdirectory would otherwise register a slot under the
		// subdirectory's name, which later lookups would never match).
		root, err := currentWorktreeRoot(ctx)
		if err != nil {
			return err
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
		cli.Step(stderr, "Bringing the stack up...")
		if err := plugin.Up(root, upDetach); err != nil {
			return fmt.Errorf("failed to bring the stack up: %w", err)
		}
		if upDetach && !upIsolated {
			cli.Success(w, "Stack is up")
		}

		return nil
	}),
}

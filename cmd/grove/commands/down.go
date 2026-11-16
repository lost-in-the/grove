package commands

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/plugins/docker"
)

func init() {
	rootCmd.AddCommand(downCmd)
}

// sharedStackDownNotice returns the line grove down prints before stopping the
// shared external stack when it is configured for a worktree other than the
// current one, or "" when no notice is needed. The command still stops the
// stack: there is only one.
func sharedStackDownNotice(shared bool, configured string, configuredOK bool, current string, displayName func(string) string) string {
	if !shared || !configuredOK || filepath.Clean(configured) == filepath.Clean(current) {
		return ""
	}
	return fmt.Sprintf("Stopping the shared stack, currently configured for '%s'", displayName(configured))
}

var downCmd = &cobra.Command{
	Use:     "down",
	Aliases: []string{"do"},
	Short:   "Stop containers for current worktree",
	Long: `Stop Docker containers for the current worktree.

Local mode: looks for a docker-compose.yml file in the current directory
and stops all services defined in it.

External mode: stops the configured services of the one shared stack, whichever
worktree it is configured for. When that is not the current worktree, grove
says so before stopping it.

If the worktree has an isolated stack running (started with grove up --isolated),
it is automatically detected and torn down instead.

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
		isolated := docker.HasActiveAgentSlot(ctx.Config, root)
		if isolated {
			plugin.SetIsolated(true)
		}
		if err := plugin.Init(ctx.Config); err != nil {
			return fmt.Errorf("failed to initialize docker plugin: %w", err)
		}

		// Stop containers — no spinner: docker compose writes its own progress
		stderr := cli.NewStderr()
		if !isolated {
			configured, configuredOK := docker.ConfiguredWorktreePath(ctx.Config)
			mgr, mgrErr := ctx.WorktreeManager()
			if mgrErr == nil {
				if notice := sharedStackDownNotice(usesSharedExternalStack(ctx.Config), configured, configuredOK, root, mgr.DisplayNameForPath); notice != "" {
					cli.Warning(stderr, "%s", notice)
				}
			}
		}
		cli.Step(stderr, "Stopping containers...")
		if err := plugin.Down(root); err != nil {
			return fmt.Errorf("failed to stop containers: %w", err)
		}

		cli.Success(w, "Containers stopped")
		return nil
	}),
}

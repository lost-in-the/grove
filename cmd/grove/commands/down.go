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
// shared external stack when it is on a worktree other than here, or "" when
// no notice is needed. The command still stops the
// stack: there is only one.
func sharedStackDownNotice(shared bool, configured string, configuredOK bool, current string, displayName func(string) string) string {
	if !shared || !configuredOK || filepath.Clean(configured) == filepath.Clean(current) {
		return ""
	}
	return fmt.Sprintf("Stopping the stack. It is on '%s', not here.", displayName(configured))
}

var downCmd = &cobra.Command{
	Use:     "down",
	Aliases: []string{"do"},
	Short:   "Stop the stack",
	Long: `Stop the stack.

Local mode: looks for a docker-compose.yml file in the current directory
and stops all services defined in it.

External mode: stops the one shared stack, whichever worktree it is on. When
that is not here, grove says so before stopping it.

If the worktree has an isolated stack running (started with grove up --isolated),
it is automatically detected and torn down instead.

Examples:
  grove down   # Stop the stack
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
		cli.Step(stderr, "Stopping the stack...")
		if err := plugin.Down(root); err != nil {
			return fmt.Errorf("failed to stop the stack: %w", err)
		}

		cli.Success(w, "Stack stopped")
		return nil
	}),
}

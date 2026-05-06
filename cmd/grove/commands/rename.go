package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/exitcode"
	"github.com/lost-in-the/grove/internal/tmux"
	"github.com/lost-in-the/grove/internal/worktree"
)

var renameCmd = &cobra.Command{
	Use:   "rename [old] [new]",
	Short: "Rename a worktree",
	Long: `Rename a git worktree, updating its directory, state entry, and tmux session.

The worktree directory is moved from {project}-{old} to {project}-{new},
the state entry is re-keyed, and any associated tmux session is renamed.

Protected and main worktrees cannot be renamed.

Examples:
  grove rename feature-auth auth-v2
  grove rename old-name new-name`,
	Args:              cobra.RangeArgs(0, 2),
	ValidArgsFunction: completeWorktreeNamesFirstArg,
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		w := cli.NewStdout()
		stderr := cli.NewStderr()

		var oldName, newName string
		switch len(args) {
		case 0:
			selected, err := selectWorktree(ctx, "Rename which worktree?")
			if err != nil {
				return err
			}
			oldName = selected
			newName, err = cli.ReadLine("New name: ")
			if err != nil {
				return err
			}
		case 1:
			oldName = args[0]
			if !cli.IsInteractive() {
				return fmt.Errorf("new name required: grove rename %s <new-name>", args[0])
			}
			var err error
			newName, err = cli.ReadLine(fmt.Sprintf("New name for '%s': ", oldName))
			if err != nil {
				return err
			}
		case 2:
			oldName = args[0]
			newName = args[1]
		}

		if oldName == "" || newName == "" {
			return fmt.Errorf("both old and new names are required")
		}

		mgr, err := worktree.NewManager(ctx.ProjectRoot)
		if err != nil {
			return fmt.Errorf("failed to initialize worktree manager: %w", err)
		}

		// Find the worktree by old name
		wt, err := mgr.Find(oldName)
		if err != nil {
			return fmt.Errorf("failed to find worktree: %w", err)
		}
		if wt == nil {
			cli.Error(stderr, "worktree '%s' not found", oldName)
			os.Exit(exitcode.ResourceNotFound)
		}

		// Check if new name is already taken
		existing, err := mgr.Find(newName)
		if err != nil {
			return fmt.Errorf("failed to check new name: %w", err)
		}

		// Validate rename preconditions
		currentTree, _ := mgr.GetCurrent()
		if err := validateRename(wt, existing, currentTree, ctx.Config, oldName); err != nil {
			cli.Error(stderr, "%s", err)
			if err == errCurrentWorktree {
				cli.Info(stderr, "Switch to another worktree first: grove to <name>")
			}
			os.Exit(exitcode.CannotRemove)
		}

		// Step 1: Move the git worktree directory
		if err := mgr.Move(oldName, newName); err != nil {
			return fmt.Errorf("failed to move worktree: %w", err)
		}

		// Step 2: Rename in state
		if err := ctx.State.RenameWorktree(oldName, newName); err != nil {
			cli.Warning(w, "Worktree moved but state update failed: %v", err)
		}

		// Step 3: Update the path in state to reflect the new directory
		newFullName := mgr.FullName(newName)
		newWt, findErr := mgr.Find(newName)
		if findErr == nil && newWt != nil {
			if ws, _ := ctx.State.GetWorktree(newName); ws != nil {
				ws.Path = newWt.Path
				// Re-add to save the updated path
				_ = ctx.State.AddWorktree(newName, ws)
			}
		} else {
			cli.Warning(w, "Could not update worktree path for %s", newFullName)
		}

		cli.Success(w, "Renamed worktree '%s' to '%s'", oldName, newName)

		// Step 4: Rename tmux session if it exists
		if tmux.IsTmuxAvailable() {
			projectName := mgr.GetProjectName()
			oldSessionName := worktree.TmuxSessionName(projectName, oldName)
			newSessionName := worktree.TmuxSessionName(projectName, newName)

			exists, err := tmux.SessionExists(oldSessionName)
			if err == nil && exists {
				if err := tmux.RenameSession(oldSessionName, newSessionName); err != nil {
					cli.Warning(w, "Failed to rename tmux session: %v", err)
				} else {
					cli.Success(w, "Renamed tmux session '%s' to '%s'", oldSessionName, newSessionName)
				}
			}
		}

		return nil
	}),
}

var (
	errMainWorktree    = fmt.Errorf("cannot rename the main worktree")
	errCurrentWorktree = fmt.Errorf("cannot rename current worktree")
)

// validateRename checks rename preconditions: not main, not protected,
// not a name collision, and not the current worktree.
func validateRename(wt, existing, current *worktree.Worktree, cfg interface{ IsProtected(string) bool }, oldName string) error {
	if wt.IsMain {
		return errMainWorktree
	}
	if cfg != nil && cfg.IsProtected(oldName) {
		return fmt.Errorf("worktree '%s' is protected", oldName)
	}
	if existing != nil {
		return fmt.Errorf("a worktree named '%s' already exists", existing.ShortName)
	}
	if current != nil && current.Path == wt.Path {
		return errCurrentWorktree
	}
	return nil
}

func init() {
	rootCmd.AddCommand(renameCmd)
}

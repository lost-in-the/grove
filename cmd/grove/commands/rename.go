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

		mgr, err := ctx.WorktreeManager()
		if err != nil {
			return err
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

		// Operate on the resolved short name, not the raw argument. Find
		// accepts a branch or full directory name too (B2); using the typed
		// value for the protection check, state re-key, and tmux rename would
		// half-complete the rename — move the directory but leave state keyed
		// by the old short name and skip the session rename (B15). Mirrors the
		// resolvedName fix in `grove rm`.
		resolvedOld := wt.ShortName

		// Check if new name is already taken
		existing, err := mgr.Find(newName)
		if err != nil {
			return fmt.Errorf("failed to check new name: %w", err)
		}

		// Validate rename preconditions. validateRename only consults
		// current.Path — synthesize a minimal Worktree from CurrentPath.
		var currentTree *worktree.Worktree
		if currentPath, err := mgr.CurrentPath(); err == nil {
			currentTree = &worktree.Worktree{Path: currentPath}
		}
		if err := validateRename(wt, existing, currentTree, ctx.Config, resolvedOld, newName); err != nil {
			cli.Error(stderr, "%s", err)
			if err == errCurrentWorktree {
				cli.Info(stderr, "Switch to another worktree first: grove to <name>")
			}
			os.Exit(exitcode.CannotRemove)
		}

		// Step 1: Move the git worktree directory
		if err := mgr.Move(resolvedOld, newName); err != nil {
			return fmt.Errorf("failed to move worktree: %w", err)
		}

		// Steps 2 + 3: rename in state, then update the path. Batched into a
		// single save — without this the rename + path update would write
		// state.json twice.
		newFullName := mgr.FullName(newName)
		batchErr := ctx.State.Batch(func() error {
			if err := ctx.State.RenameWorktree(resolvedOld, newName); err != nil {
				cli.Warning(w, "Worktree moved but state update failed: %v", err)
			}

			newWt, findErr := mgr.Find(newName)
			if findErr == nil && newWt != nil {
				if ws, _ := ctx.State.GetWorktree(newName); ws != nil {
					ws.Path = newWt.Path
					_ = ctx.State.AddWorktree(newName, ws)
				}
			} else {
				cli.Warning(w, "Could not update worktree path for %s", newFullName)
			}
			return nil
		})
		if batchErr != nil {
			cli.Warning(w, "state save failed: %v", batchErr)
		}

		cli.Success(w, "Renamed worktree '%s' to '%s'", resolvedOld, newName)

		// Step 4: Rename tmux session if it exists
		if tmux.IsTmuxAvailable() {
			projectName := mgr.GetProjectName()
			oldSessionName := worktree.TmuxSessionName(projectName, resolvedOld)
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
// not a name collision, not the current worktree, and that the new name is
// a valid worktree name — the same worktree.ValidateWorktreeName check the
// create paths and TUI overlays enforce. Without it, `grove rename x ../escape`
// or `grove rename x -flag` would reach mgr.Move with an unvalidated path
// component.
func validateRename(wt, existing, current *worktree.Worktree, cfg interface{ IsProtected(string) bool }, oldName, newName string) error {
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
	if errMsg := worktree.ValidateWorktreeName(newName); errMsg != "" {
		return fmt.Errorf("invalid new name '%s': %s", newName, errMsg)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(renameCmd)
}

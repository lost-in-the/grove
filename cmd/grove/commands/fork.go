package commands

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/cmdexec"
	"github.com/lost-in-the/grove/internal/exitcode"
	"github.com/lost-in-the/grove/internal/grove"
	"github.com/lost-in-the/grove/internal/hooks"
	"github.com/lost-in-the/grove/internal/output"
	"github.com/lost-in-the/grove/internal/state"
	"github.com/lost-in-the/grove/internal/tmux"
	"github.com/lost-in-the/grove/internal/worktree"
)

var (
	forkBranchName string
	forkMoveWIP    bool
	forkCopyWIP    bool
	forkNoWIP      bool
	forkNoSwitch   bool
	forkJSON       bool
)

// ForkResult represents the JSON output for grove fork.
type ForkResult struct {
	SwitchTo string `json:"switch_to,omitempty"`
	Name     string `json:"name"`
	Branch   string `json:"branch"`
	Path     string `json:"path"`
	Parent   string `json:"parent"`
	Created  bool   `json:"created"`
}

var forkCmd = &cobra.Command{
	Use:     "fork [name]",
	Aliases: []string{"split", "fo"},
	Short:   "Fork current worktree into a new one",
	Long: `Fork the current worktree, creating a new worktree branching from the current HEAD.

The new branch name will be {current-branch}-{name} unless --branch-name is specified.
By default, prompts to handle uncommitted changes. Use --move-wip, --copy-wip, or --no-wip to skip prompt.

Examples:
  grove fork feature-auth        # Fork into new worktree with branch main-feature-auth
  grove fork hotfix --branch-name emergency-fix  # Use specific branch name
  grove fork experiment --move-wip   # Move uncommitted changes to fork
  grove fork test --no-switch    # Fork but stay in current worktree`,
	Args: cobra.MaximumNArgs(1),
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		w := cli.NewStdout()
		stderr := cli.NewStderr()

		var name string
		if len(args) == 0 {
			if !cli.IsInteractive() {
				return fmt.Errorf("worktree name required: grove fork <name>")
			}
			var err error
			name, err = cli.ReadLine("Fork name: ")
			if err != nil {
				return err
			}
		} else {
			name = args[0]
		}

		if name == "" {
			return fmt.Errorf("worktree name cannot be empty")
		}

		mgr, err := ctx.WorktreeManager()
		if err != nil {
			return err
		}

		// Get current worktree
		currentTree, err := mgr.GetCurrent()
		if err != nil {
			return fmt.Errorf("failed to get current worktree: %w", err)
		}
		if currentTree == nil {
			return fmt.Errorf("could not determine current worktree")
		}

		parentName := currentTree.DisplayName()

		// Determine the base reference (HEAD for normal, mirror for environment)
		baseRef := "HEAD"
		isEnv, _ := ctx.State.IsEnvironment(parentName)
		if isEnv {
			ws, err := ctx.State.GetWorktree(parentName)
			if err == nil && ws != nil && ws.Mirror != "" {
				// For environment worktrees, fork from the mirror's HEAD
				baseRef = ws.Mirror
				if !forkJSON {
					cli.Info(w, "Forking from environment worktree (mirror: %s)", ws.Mirror)
				}
			}
		}

		// Determine new branch name
		newBranchName := forkBranchName
		if newBranchName == "" {
			newBranchName = fmt.Sprintf("%s-%s", currentTree.Branch, name)
		}

		// Check if branch already exists
		if err := cmdexec.Run(context.TODO(), "git", []string{"-C", currentTree.Path, "show-ref", "--verify", "--quiet", "refs/heads/" + newBranchName}, "", cmdexec.GitLocal); err == nil {
			// Branch exists
			cli.Error(stderr, "branch '%s' already exists", newBranchName)
			os.Exit(exitcode.ResourceExists)
		}

		// Handle WIP (work-in-progress)
		wipHandler := worktree.NewWIPHandler(currentTree.Path)
		hasWIP, err := wipHandler.HasWIP()
		if err != nil {
			return fmt.Errorf("failed to check for uncommitted changes: %w", err)
		}

		var wipPatch []byte
		if hasWIP {
			// Determine WIP handling strategy
			if !forkMoveWIP && !forkCopyWIP && !forkNoWIP {
				// Prompt user if interactive
				if !cli.IsInteractive() {
					return fmt.Errorf("uncommitted changes detected; use --move-wip, --copy-wip, or --no-wip")
				}

				files, _ := wipHandler.ListWIPFiles()
				cli.Warning(stderr, "Uncommitted changes detected (%d files):", len(files))
				for i, f := range files {
					if i >= 5 {
						cli.Faint(stderr, "  ... and %d more", len(files)-5)
						break
					}
					cli.Faint(stderr, "  %s", f)
				}

				choice, err := cli.Choose("How do you want to handle uncommitted changes?", []string{
					"Move to fork",
					"Copy to fork",
					"Leave in current",
					"Cancel",
				})
				if err != nil {
					cli.Info(w, "Canceled")
					os.Exit(exitcode.UserCancelled)
				}

				switch choice {
				case "Move to fork":
					forkMoveWIP = true
				case "Copy to fork":
					forkCopyWIP = true
				case "Leave in current":
					forkNoWIP = true
				case "Cancel":
					cli.Info(w, "Canceled")
					os.Exit(exitcode.UserCancelled)
				}
			}

			// Execute WIP handling - create patch only; reset deferred until after fork succeeds
			if forkMoveWIP || forkCopyWIP {
				// Create patch from current changes
				wipPatch, err = wipHandler.CreatePatch()
				if err != nil {
					return fmt.Errorf("failed to capture changes: %w", err)
				}
			}
		}

		// Create branch from base reference
		if output, err := cmdexec.CombinedOutput(context.TODO(), "git", []string{"-C", currentTree.Path, "branch", newBranchName, baseRef}, "", cmdexec.GitLocal); err != nil {
			cli.Error(stderr, "git operation failed: %s", output)
			os.Exit(exitcode.GitOperationFailed)
		}

		// Create worktree
		if err := mgr.CreateFromBranch(name, newBranchName); err != nil {
			// Cleanup: delete the branch we just created
			_ = cmdexec.Run(context.TODO(), "git", []string{"-C", currentTree.Path, "branch", "-D", newBranchName}, "", cmdexec.GitLocal)
			return fmt.Errorf("failed to create worktree: %w", err)
		}

		// Find the created worktree
		newTree, err := mgr.Find(name)
		if err != nil || newTree == nil {
			return fmt.Errorf("failed to find created worktree")
		}

		if !forkJSON {
			cli.Success(w, "Created worktree '%s' with branch '%s'", name, newBranchName)
		}

		// Symlink config.toml from main worktree
		if err := grove.EnsureConfigSymlink(ctx.ProjectRoot, newTree.Path); err != nil {
			if !forkJSON {
				cli.Warning(w, "Failed to symlink config: %v", err)
			}
		}

		// Apply WIP patch to new worktree if needed
		if len(wipPatch) > 0 && (forkMoveWIP || forkCopyWIP) {
			newWipHandler := worktree.NewWIPHandler(newTree.Path)
			if err := newWipHandler.ApplyPatch(wipPatch); err != nil {
				cli.Warning(stderr, "Failed to apply changes to fork: %v", err)
				cli.Warning(stderr, "Changes are preserved in the source worktree")
			} else {
				if forkCopyWIP && !forkJSON {
					cli.Success(w, "Copied uncommitted changes to fork")
				}
				// Reset source worktree only after successful patch application
				if forkMoveWIP {
					if output, err := cmdexec.CombinedOutput(context.TODO(), "git", []string{"-C", currentTree.Path, "checkout", "--", "."}, "", cmdexec.GitLocal); err != nil {
						cli.Warning(stderr, "changes applied to fork but failed to reset source: %v\n%s", err, output)
					} else {
						if output, err := cmdexec.CombinedOutput(context.TODO(), "git", []string{"-C", currentTree.Path, "clean", "-fd"}, "", cmdexec.GitLocal); err != nil {
							cli.Warning(stderr, "failed to clean untracked files in source: %v\n%s", err, output)
						}
						if !forkJSON {
							cli.Success(w, "Moved uncommitted changes to fork")
						}
					}
				}
			}
		}

		// Register in state with parent tracking
		now := time.Now()
		wsState := &state.WorktreeState{
			Path:           newTree.Path,
			Branch:         newBranchName,
			CreatedAt:      now,
			LastAccessedAt: now,
			ParentWorktree: parentName,
		}
		if err := ctx.State.AddWorktree(name, wsState); err != nil {
			cli.Warning(stderr, "worktree created but state tracking failed: %v", err)
			cli.Info(stderr, "run 'grove repair' to fix")
		}

		runFileSetup(ctx.Config, newTree.Path, ctx.ProjectRoot, w, forkJSON)

		// Fire post-create hook
		hookCtx := &hooks.Context{
			Worktree:     name,
			Config:       ctx.Config,
			WorktreePath: newTree.Path,
			MainPath:     ctx.ProjectRoot,
		}
		if err := hooks.Fire(hooks.EventPostCreate, hookCtx); err != nil {
			cli.Warning(stderr, "Post-create hook failed: %v", err)
		}

		projectName := mgr.GetProjectName()

		// Create tmux session
		if tmux.IsTmuxAvailable() {
			sessionName := worktree.TmuxSessionName(projectName, name)
			if err := tmux.CreateSession(sessionName, newTree.Path); err != nil {
				cli.Warning(stderr, "Failed to create tmux session: %v", err)
			} else if !forkJSON {
				cli.Success(w, "Created tmux session '%s'", sessionName)
			}
		}

		// JSON output mode
		if forkJSON {
			result := ForkResult{
				Name:    name,
				Branch:  newBranchName,
				Path:    newTree.Path,
				Parent:  parentName,
				Created: true,
			}
			if !forkNoSwitch {
				result.SwitchTo = newTree.Path
			}
			return output.PrintJSON(result)
		}

		// Switch to new worktree unless --no-switch. Agent mode and tmux mode
		// "off" suppress the client switch (terminal takeover) — same policy
		// as `grove new` and `grove to`.
		if !forkNoSwitch {
			suppressTmux := effectiveTmuxMode(ctx.Config.Tmux.Mode, ctx.Config.AgentMode, false, false) == tmuxModeOff
			sessionName := worktree.TmuxSessionName(projectName, name)
			if !switchToWorktree(ctx, stderr, parentName, name, sessionName, newTree.Path, suppressTmux) {
				emitCdOrExplain(stderr, newTree.Path)
			}
		} else {
			cli.Info(w, "To switch to the new worktree: grove to %s", name)
		}

		return nil
	}),
}

func init() {
	forkCmd.Flags().StringVar(&forkBranchName, "branch-name", "", "Override branch name")
	forkCmd.Flags().BoolVar(&forkMoveWIP, "move-wip", false, "Move uncommitted changes to fork")
	forkCmd.Flags().BoolVar(&forkCopyWIP, "copy-wip", false, "Copy uncommitted changes to both")
	forkCmd.Flags().BoolVar(&forkNoWIP, "no-wip", false, "Fork starts clean (leave changes in current)")
	forkCmd.Flags().BoolVar(&forkNoSwitch, "no-switch", false, "Stay in current worktree")
	forkCmd.Flags().BoolVarP(&forkJSON, "json", "j", false, "Output as JSON")

	// Mark WIP flags as mutually exclusive
	forkCmd.MarkFlagsMutuallyExclusive("move-wip", "copy-wip", "no-wip")

	rootCmd.AddCommand(forkCmd)
}

package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/log"
	"github.com/lost-in-the/grove/internal/output"
	"github.com/lost-in-the/grove/internal/tmux"
	"github.com/lost-in-the/grove/internal/worktree"
)

var lastJSON bool

var lastCmd = &cobra.Command{
	Use:     "last",
	Aliases: []string{"la"},
	Short:   "Switch to the previous worktree",
	Long:    `Switch to the last worktree you were working in.`,
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		stderr := cli.NewStderr()

		mgr, err := ctx.WorktreeManager()
		if err != nil {
			return err
		}

		// Try to get last worktree from state first (V2 approach)
		lastWorktree, err := ctx.State.GetLastWorktree()
		if err != nil || lastWorktree == "" {
			// Fallback to tmux session tracking (legacy approach)
			lastSession, err := tmux.GetLastSession()
			if err != nil {
				return fmt.Errorf("no last worktree found: %w", err)
			}

			projectName := mgr.GetProjectName()
			expectedPrefix := projectName + "-"
			if trimmed, found := strings.CutPrefix(lastSession, expectedPrefix); found {
				lastWorktree = trimmed
			} else {
				lastWorktree = lastSession
			}
		}

		// Find the target worktree
		targetTree, err := mgr.Find(lastWorktree)
		if err != nil {
			return fmt.Errorf("failed to find worktree: %w", err)
		}
		if targetTree == nil {
			return fmt.Errorf("last worktree '%s' not found", lastWorktree)
		}

		// Batch the SetLastWorktree + TouchWorktree pair so the state file
		// is written once instead of twice.
		projectName := mgr.GetProjectName()
		var tmuxSwitched bool
		if batchErr := ctx.State.Batch(func() error {
			if currentPath, err := mgr.CurrentPath(); err == nil {
				prevName := mgr.DisplayNameForPath(currentPath)
				if prevName != "" {
					if err := ctx.State.SetLastWorktree(prevName); err != nil {
						log.Printf("failed to set last worktree %q: %v", prevName, err)
					}
				}
			}

			// Store current session as last if inside tmux
			if tmux.IsInsideTmux() {
				currentSession, err := tmux.GetCurrentSession()
				if err == nil {
					if err := tmux.StoreLastSession(currentSession); err != nil {
						log.Printf("failed to store last session %q: %v", currentSession, err)
					}
				}
			}

			// Switch to session
			if tmux.IsTmuxAvailable() && tmux.IsInsideTmux() {
				sessionName := worktree.TmuxSessionName(projectName, lastWorktree)
				if err := tmux.SwitchSession(sessionName); err != nil {
					return fmt.Errorf("failed to switch session: %w", err)
				}
				tmuxSwitched = true
			}

			// Update last_accessed_at for target worktree
			if err := ctx.State.TouchWorktree(targetTree.DisplayName()); err != nil {
				log.Printf("failed to touch worktree %q: %v", targetTree.DisplayName(), err)
			}
			return nil
		}); batchErr != nil {
			return batchErr
		}

		// JSON output mode
		if lastJSON {
			result := output.SwitchResult{
				SwitchTo: targetTree.Path,
				Name:     targetTree.DisplayName(),
				Branch:   targetTree.Branch,
				Path:     targetTree.Path,
			}
			return output.PrintJSON(result)
		}

		// Skip cd directive when tmux switch already moved the user
		if !tmuxSwitched {
			hasShellIntegration := os.Getenv("GROVE_SHELL") == "1"

			if hasShellIntegration {
				cli.Directive("cd", targetTree.Path)
			} else {
				cli.Faint(stderr, "Note: Directory switching requires shell integration.")
				cli.Faint(stderr, "Add this to your shell config (~/.zshrc or ~/.bashrc):")
				_, _ = fmt.Fprintf(stderr, "\n")
				cli.Faint(stderr, "  eval \"$(grove install zsh)\"   # for zsh")
				cli.Faint(stderr, "  eval \"$(grove install bash)\"  # for bash")
				_, _ = fmt.Fprintf(stderr, "\n")
				cli.Faint(stderr, "To change directory manually:")
				cli.Faint(stderr, "  cd %s", targetTree.Path)
			}
		}

		return nil
	}),
}

func init() {
	lastCmd.Flags().BoolVarP(&lastJSON, "json", "j", false, "Output as JSON with switch_to field")
	rootCmd.AddCommand(lastCmd)
}

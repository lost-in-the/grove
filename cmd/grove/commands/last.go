package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/LeahArmstrong/grove-cli/internal/cli"
	"github.com/LeahArmstrong/grove-cli/internal/log"
	"github.com/LeahArmstrong/grove-cli/internal/output"
	"github.com/LeahArmstrong/grove-cli/internal/tmux"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
)

var lastJSON bool

var lastCmd = &cobra.Command{
	Use:   "last",
	Short: "Switch to the previous worktree",
	Long:  `Switch to the last worktree you were working in.`,
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		stderr := cli.NewStderr()

		// Try to get last worktree from state first (V2 approach)
		lastWorktree, err := ctx.State.GetLastWorktree()
		if err != nil || lastWorktree == "" {
			// Fallback to tmux session tracking (legacy approach)
			lastSession, err := tmux.GetLastSession()
			if err != nil {
				return fmt.Errorf("no last worktree found: %w", err)
			}

			mgr, err := worktree.NewManager(ctx.ProjectRoot)
			if err != nil {
				return fmt.Errorf("failed to initialize worktree manager: %w", err)
			}

			projectName := mgr.GetProjectName()
			expectedPrefix := projectName + "-"
			if trimmed, found := strings.CutPrefix(lastSession, expectedPrefix); found {
				lastWorktree = trimmed
			} else {
				lastWorktree = lastSession
			}
		}

		mgr, err := worktree.NewManager(ctx.ProjectRoot)
		if err != nil {
			return fmt.Errorf("failed to initialize worktree manager: %w", err)
		}

		// Find the target worktree
		targetTree, err := mgr.Find(lastWorktree)
		if err != nil {
			return fmt.Errorf("failed to find worktree: %w", err)
		}
		if targetTree == nil {
			return fmt.Errorf("last worktree '%s' not found", lastWorktree)
		}

		// Get current worktree for state update
		currentTree, _ := mgr.GetCurrent()
		if currentTree != nil {
			// Update last_worktree in state before switching
			if err := ctx.State.SetLastWorktree(currentTree.DisplayName()); err != nil {
				log.Printf("failed to set last worktree %q: %v", currentTree.DisplayName(), err)
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

		projectName := mgr.GetProjectName()

		// Switch to session
		var tmuxSwitched bool
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

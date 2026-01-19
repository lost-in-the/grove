package commands

import (
	"fmt"
	"os"

	"github.com/LeahArmstrong/grove-cli/internal/hooks"
	"github.com/LeahArmstrong/grove-cli/internal/tmux"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
	"github.com/spf13/cobra"
)

var toCmd = &cobra.Command{
	Use:     "to <name>",
	Aliases: []string{"switch"},
	Short:   "Switch to a worktree",
	Long: `Switch to a worktree by name. If a tmux session exists for the worktree, switch to it.
If no tmux session exists, create one.

When using shell integration, this will also change your current directory.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if name == "" {
			return fmt.Errorf("worktree name cannot be empty")
		}

		// Get worktree
		mgr, err := worktree.NewManager("")
		if err != nil {
			return fmt.Errorf("failed to initialize worktree manager: %w", err)
		}

		trees, err := mgr.List()
		if err != nil {
			return fmt.Errorf("failed to list worktrees: %w", err)
		}

		var targetTree *worktree.Worktree
		for _, tree := range trees {
			if tree.Name == name {
				targetTree = tree
				break
			}
		}

		if targetTree == nil {
			return fmt.Errorf("worktree '%s' not found", name)
		}

		// Get current worktree for hook context
		currentTree, _ := mgr.GetCurrent()
		var prevWorktree string
		if currentTree != nil {
			prevWorktree = currentTree.Name
		}

		// Fire pre-switch hooks
		hookCtx := &hooks.Context{
			Worktree:     name,
			PrevWorktree: prevWorktree,
		}
		if err := hooks.Fire(hooks.EventPreSwitch, hookCtx); err != nil {
			fmt.Fprintf(os.Stderr, "warning: pre-switch hooks failed: %v\n", err)
			// Continue anyway
		}

		// Store current session as last if inside tmux
		if tmux.IsInsideTmux() {
			currentSession, err := tmux.GetCurrentSession()
			if err == nil {
				tmux.StoreLastSession(currentSession)
			}
		}

		// Handle tmux session
		if tmux.IsTmuxAvailable() {
			projectName := mgr.GetProjectName()
			sessionName := worktree.TmuxSessionName(projectName, name)
			exists, err := tmux.SessionExists(sessionName)
			if err != nil {
				return fmt.Errorf("failed to check session: %w", err)
			}

			if !exists {
				// Create session if it doesn't exist
				if err := tmux.CreateSession(sessionName, targetTree.Path); err != nil {
					return fmt.Errorf("failed to create session: %w", err)
				}
				fmt.Printf("✓ Created tmux session '%s'\n", sessionName)
			}

			// Switch or attach to session
			if tmux.IsInsideTmux() {
				if err := tmux.SwitchSession(sessionName); err != nil {
					return fmt.Errorf("failed to switch session: %w", err)
				}
			} else {
				fmt.Printf("✓ Tmux session '%s' ready\n", sessionName)
				fmt.Printf("Run: tmux attach -t %s\n", sessionName)
			}
		}

		// Output directory change command for shell integration
		hasShellIntegration := os.Getenv("GROVE_SHELL") == "1"
		
		if hasShellIntegration {
			// Shell wrapper will parse this and execute cd
			fmt.Printf("cd:%s\n", targetTree.Path)
		} else {
			// Not in shell wrapper - show helpful message
			fmt.Fprintf(os.Stderr, "\nNote: Directory switching requires shell integration.\n")
			fmt.Fprintf(os.Stderr, "Add this to your shell config (~/.zshrc or ~/.bashrc):\n\n")
			fmt.Fprintf(os.Stderr, "  eval \"$(grove init zsh)\"   # for zsh\n")
			fmt.Fprintf(os.Stderr, "  eval \"$(grove init bash)\"  # for bash\n\n")
			fmt.Fprintf(os.Stderr, "To change directory manually:\n")
			fmt.Fprintf(os.Stderr, "  cd %s\n", targetTree.Path)
		}

		// Fire post-switch hooks
		if err := hooks.Fire(hooks.EventPostSwitch, hookCtx); err != nil {
			fmt.Fprintf(os.Stderr, "warning: post-switch hooks failed: %v\n", err)
			// Continue anyway
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(toCmd)
}

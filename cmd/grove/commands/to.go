package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/LeahArmstrong/grove-cli/internal/hooks"
	"github.com/LeahArmstrong/grove-cli/internal/output"
	"github.com/LeahArmstrong/grove-cli/internal/tmux"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
	"github.com/spf13/cobra"
)

var toJSON bool

var toCmd = &cobra.Command{
	Use:     "to <name>",
	Aliases: []string{"switch"},
	Short:   "Switch to a worktree",
	Long: `Switch to a worktree by name. If a tmux session exists for the worktree, switch to it.
If no tmux session exists, create one.

When using shell integration, this will also change your current directory.`,
	Args: cobra.ExactArgs(1),
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		name := args[0]
		if name == "" {
			return fmt.Errorf("worktree name cannot be empty")
		}

		mgr, err := worktree.NewManager(ctx.ProjectRoot)
		if err != nil {
			return fmt.Errorf("failed to initialize worktree manager: %w", err)
		}

		// Find worktree by short name or full name
		targetTree, err := mgr.Find(name)
		if err != nil {
			return fmt.Errorf("failed to find worktree: %w", err)
		}
		if targetTree == nil {
			return fmt.Errorf("worktree '%s' not found", name)
		}

		// Check if worktree is stale (directory missing)
		if targetTree.IsPrunable {
			return fmt.Errorf("worktree '%s' is stale (directory missing). Run 'grove rm %s' to clean up", name, name)
		}

		// Get current worktree for hook context and state update
		currentTree, _ := mgr.GetCurrent()
		var prevWorktree string
		if currentTree != nil {
			prevWorktree = currentTree.DisplayName()
			// Update last_worktree in state before switching
			_ = ctx.State.SetLastWorktree(prevWorktree)
		}

		// Fire pre-switch hooks
		hookCtx := &hooks.Context{
			Worktree:     name,
			PrevWorktree: prevWorktree,
		}
		if err := hooks.Fire(hooks.EventPreSwitch, hookCtx); err != nil {
			fmt.Fprintf(os.Stderr, "warning: pre-switch hooks failed: %v\n", err)
		}

		// Store current session as last if inside tmux
		if tmux.IsInsideTmux() {
			currentSession, err := tmux.GetCurrentSession()
			if err == nil {
				tmux.StoreLastSession(currentSession)
			}
		}

		projectName := mgr.GetProjectName()

		// Handle tmux session
		if tmux.IsTmuxAvailable() {
			sessionName := worktree.TmuxSessionName(projectName, name)
			exists, err := tmux.SessionExists(sessionName)
			if err != nil {
				return fmt.Errorf("failed to check session: %w", err)
			}

			if !exists {
				if err := tmux.CreateSession(sessionName, targetTree.Path); err != nil {
					return fmt.Errorf("failed to create session: %w", err)
				}
				if !toJSON {
					fmt.Printf("✓ Created tmux session '%s'\n", sessionName)
				}
			}

			if tmux.IsInsideTmux() {
				if err := tmux.SwitchSession(sessionName); err != nil {
					return fmt.Errorf("failed to switch session: %w", err)
				}
			} else if !toJSON {
				fmt.Printf("✓ Tmux session '%s' ready\n", sessionName)
				fmt.Printf("Run: tmux attach -t %s\n", sessionName)
			}
		}

		// Update last_accessed_at for target worktree
		_ = ctx.State.TouchWorktree(targetTree.DisplayName())

		// JSON output mode
		if toJSON {
			result := output.SwitchResult{
				SwitchTo: targetTree.Path,
				Name:     targetTree.DisplayName(),
				Branch:   targetTree.Branch,
				Path:     targetTree.Path,
			}
			data, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		// Output directory change command for shell integration
		hasShellIntegration := os.Getenv("GROVE_SHELL") == "1"

		if hasShellIntegration {
			// Shell wrapper will parse this and execute cd
			fmt.Printf("cd:%s\n", targetTree.Path)
		} else {
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
		}

		return nil
	}),
}

func init() {
	toCmd.Flags().BoolVarP(&toJSON, "json", "j", false, "Output as JSON with switch_to field")
	rootCmd.AddCommand(toCmd)
}

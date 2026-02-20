package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

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
			_ = ctx.State.SetLastWorktree(currentTree.DisplayName())
		}

		// Store current session as last if inside tmux
		if tmux.IsInsideTmux() {
			currentSession, err := tmux.GetCurrentSession()
			if err == nil {
				_ = tmux.StoreLastSession(currentSession)
			}
		}

		projectName := mgr.GetProjectName()

		// Switch to session
		if tmux.IsTmuxAvailable() && tmux.IsInsideTmux() {
			sessionName := worktree.TmuxSessionName(projectName, lastWorktree)
			if err := tmux.SwitchSession(sessionName); err != nil {
				return fmt.Errorf("failed to switch session: %w", err)
			}
		}

		// Update last_accessed_at for target worktree
		_ = ctx.State.TouchWorktree(targetTree.DisplayName())

		// JSON output mode
		if lastJSON {
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
			fmt.Printf("cd:%s\n", targetTree.Path)
		} else {
			fmt.Fprintf(os.Stderr, "\nNote: Directory switching requires shell integration.\n")
			fmt.Fprintf(os.Stderr, "Add this to your shell config (~/.zshrc or ~/.bashrc):\n\n")
			fmt.Fprintf(os.Stderr, "  eval \"$(grove install zsh)\"   # for zsh\n")
			fmt.Fprintf(os.Stderr, "  eval \"$(grove install bash)\"  # for bash\n\n")
			fmt.Fprintf(os.Stderr, "To change directory manually:\n")
			fmt.Fprintf(os.Stderr, "  cd %s\n", targetTree.Path)
		}

		return nil
	}),
}

func init() {
	lastCmd.Flags().BoolVarP(&lastJSON, "json", "j", false, "Output as JSON with switch_to field")
	rootCmd.AddCommand(lastCmd)
}

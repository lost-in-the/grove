package commands

import (
	"fmt"
	"strings"

	"github.com/LeahArmstrong/grove-cli/internal/config"
	"github.com/LeahArmstrong/grove-cli/internal/tmux"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
	"github.com/spf13/cobra"
)

var lastCmd = &cobra.Command{
	Use:   "last",
	Short: "Switch to the previous worktree",
	Long:  `Switch to the last worktree you were working in.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get last session
		lastSession, err := tmux.GetLastSession()
		if err != nil {
			return fmt.Errorf("no last session found: %w", err)
		}

		// Load config to get prefix
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Remove prefix to get worktree name
		name := lastSession
		if strings.HasPrefix(lastSession, cfg.Tmux.Prefix) {
			name = strings.TrimPrefix(lastSession, cfg.Tmux.Prefix)
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
			return fmt.Errorf("last worktree '%s' not found", name)
		}

		// Store current session as last if inside tmux
		if tmux.IsInsideTmux() {
			currentSession, err := tmux.GetCurrentSession()
			if err == nil {
				tmux.StoreLastSession(currentSession)
			}
		}

		// Switch to session
		if tmux.IsTmuxAvailable() && tmux.IsInsideTmux() {
			if err := tmux.SwitchSession(lastSession); err != nil {
				return fmt.Errorf("failed to switch session: %w", err)
			}
		}

		// Output directory change command for shell integration
		fmt.Printf("cd:%s\n", targetTree.Path)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(lastCmd)
}

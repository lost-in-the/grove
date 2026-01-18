package commands

import (
	"errors"
	"fmt"

	"github.com/LeahArmstrong/grove-cli/internal/config"
	"github.com/LeahArmstrong/grove-cli/internal/hooks"
	"github.com/LeahArmstrong/grove-cli/internal/state"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
	"github.com/LeahArmstrong/grove-cli/plugins/docker"
	"github.com/spf13/cobra"
)

var (
	freezeAll bool
)

var freezeCmd = &cobra.Command{
	Use:   "freeze [name]",
	Short: "Freeze a worktree to pause work",
	Long: `Freeze a worktree to mark it as inactive and stop related services.

If no name is provided, freezes the current worktree.
Use --all to freeze all worktrees except the current one.

This command will:
  • Mark the worktree as frozen in state
  • Fire pre-freeze hooks
  • Attempt to stop Docker containers (if docker plugin enabled)

Freezing is idempotent - safe to freeze an already-frozen worktree.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get worktree manager
		mgr, err := worktree.NewManager("")
		if err != nil {
			return fmt.Errorf("failed to initialize worktree manager: %w", err)
		}

		// Load config
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Initialize state manager
		stateMgr, err := state.NewManager("")
		if err != nil {
			return fmt.Errorf("failed to initialize state manager: %w", err)
		}

		// Determine which worktrees to freeze
		var targetNames []string
		
		if freezeAll {
			// Freeze all worktrees except current
			trees, err := mgr.List()
			if err != nil {
				return fmt.Errorf("failed to list worktrees: %w", err)
			}

			currentTree, err := mgr.GetCurrent()
			if err != nil {
				return fmt.Errorf("failed to get current worktree: %w", err)
			}

			for _, tree := range trees {
				// Skip current worktree
				if currentTree != nil && tree.Path == currentTree.Path {
					continue
				}
				targetNames = append(targetNames, tree.DisplayName())
			}
		} else if len(args) > 0 {
			// Freeze specific worktree by name
			targetNames = []string{args[0]}
		} else {
			// Freeze current worktree
			currentTree, err := mgr.GetCurrent()
			if err != nil {
				return fmt.Errorf("failed to get current worktree: %w", err)
			}
			targetNames = []string{currentTree.DisplayName()}
		}

		// Initialize docker plugin if enabled
		dockerPlugin := docker.New()
		if err := dockerPlugin.Init(cfg); err == nil && dockerPlugin.Enabled() {
			// Docker plugin is available
		}

		// Freeze each target worktree
		for _, name := range targetNames {
			if err := freezeWorktree(mgr, stateMgr, cfg, dockerPlugin, name); err != nil {
				fmt.Printf("⚠ Failed to freeze '%s': %v\n", name, err)
				continue
			}
			fmt.Printf("✓ Frozen worktree '%s'\n", name)
		}

		if len(targetNames) > 1 {
			fmt.Printf("\n✓ Frozen %d worktrees\n", len(targetNames))
		}

		return nil
	},
}

func freezeWorktree(mgr *worktree.Manager, stateMgr *state.Manager, cfg *config.Config, dockerPlugin *docker.Plugin, name string) error {
	// Find the worktree
	tree, err := mgr.Find(name)
	if err != nil {
		return fmt.Errorf("failed to find worktree: %w", err)
	}
	if tree == nil {
		return fmt.Errorf("worktree '%s' not found", name)
	}

	// Fire pre-freeze hook
	hookCtx := &hooks.Context{
		Worktree: name,
		Config:   cfg,
	}
	if err := hooks.Fire(hooks.EventPreFreeze, hookCtx); err != nil {
		// Log but don't fail on hook errors
		fmt.Printf("⚠ Pre-freeze hook failed: %v\n", err)
	}

	// Try to stop Docker containers if plugin is enabled
	if dockerPlugin != nil && dockerPlugin.Enabled() {
		if err := dockerPlugin.Down(tree.Path); err != nil {
			// Only warn if it's not a "no compose file" error
			if !errors.Is(err, docker.ErrNoComposeFile) {
				fmt.Printf("⚠ Failed to stop Docker containers: %v\n", err)
			}
		}
	}

	// Mark as frozen in state
	if err := stateMgr.Freeze(name); err != nil {
		return fmt.Errorf("failed to freeze worktree: %w", err)
	}

	return nil
}

func init() {
	freezeCmd.Flags().BoolVar(&freezeAll, "all", false, "Freeze all worktrees except current")
	rootCmd.AddCommand(freezeCmd)
}

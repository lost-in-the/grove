package commands

import (
	"errors"
	"fmt"
	"os"

	"github.com/LeahArmstrong/grove-cli/internal/config"
	"github.com/LeahArmstrong/grove-cli/internal/hooks"
	"github.com/LeahArmstrong/grove-cli/internal/state"
	"github.com/LeahArmstrong/grove-cli/internal/tmux"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
	"github.com/LeahArmstrong/grove-cli/plugins/docker"
	"github.com/spf13/cobra"
)

var resumeCmd = &cobra.Command{
	Use:   "resume <name>",
	Short: "Resume a frozen worktree",
	Long: `Resume a frozen worktree to continue work.

The worktree name is required.

This command will:
  • Clear the frozen state
  • Fire post-resume hooks
  • Attempt to start Docker containers (if docker plugin enabled)
  • Switch to the worktree with tmux session management
  • Change directory (with shell integration)

Resuming is idempotent - safe to resume a non-frozen worktree.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if name == "" {
			return fmt.Errorf("worktree name cannot be empty")
		}

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

		// Find the worktree
		tree, err := mgr.Find(name)
		if err != nil {
			return fmt.Errorf("failed to find worktree: %w", err)
		}
		if tree == nil {
			return fmt.Errorf("worktree '%s' not found", name)
		}

		// Clear frozen state
		if err := stateMgr.Resume(name); err != nil {
			return fmt.Errorf("failed to resume worktree: %w", err)
		}

		// Fire post-resume hook
		hookCtx := &hooks.Context{
			Worktree: name,
			Config:   cfg,
		}
		if err := hooks.Fire(hooks.EventPostResume, hookCtx); err != nil {
			// Log but don't fail on hook errors
			fmt.Printf("⚠ Post-resume hook failed: %v\n", err)
		}

		// Initialize docker plugin and try to start containers
		dockerPlugin := docker.New()
		if err := dockerPlugin.Init(cfg); err == nil && dockerPlugin.Enabled() {
			if err := dockerPlugin.Up(tree.Path, true); err != nil {
				// Only warn if it's not a "no compose file" error
				if !errors.Is(err, docker.ErrNoComposeFile) {
					fmt.Printf("⚠ Failed to start Docker containers: %v\n", err)
				}
			}
		}

		fmt.Printf("✓ Resumed worktree '%s'\n", name)

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
				if err := tmux.CreateSession(sessionName, tree.Path); err != nil {
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
			fmt.Printf("cd:%s\n", tree.Path)
		} else {
			// Not in shell wrapper - show helpful message
			fmt.Fprintf(os.Stderr, "\nNote: Directory switching requires shell integration.\n")
			fmt.Fprintf(os.Stderr, "Add this to your shell config (~/.zshrc or ~/.bashrc):\n\n")
			fmt.Fprintf(os.Stderr, "  eval \"$(grove init zsh)\"   # for zsh\n")
			fmt.Fprintf(os.Stderr, "  eval \"$(grove init bash)\"  # for bash\n\n")
			fmt.Fprintf(os.Stderr, "To change directory manually:\n")
			fmt.Fprintf(os.Stderr, "  cd %s\n", tree.Path)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(resumeCmd)
}

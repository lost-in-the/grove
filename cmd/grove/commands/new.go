package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/LeahArmstrong/grove-cli/internal/exitcode"
	"github.com/LeahArmstrong/grove-cli/internal/output"
	"github.com/LeahArmstrong/grove-cli/internal/state"
	"github.com/LeahArmstrong/grove-cli/internal/tmux"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	newJSON   bool
	newMirror string // Remote branch to mirror (e.g., "origin/main")
)

var newCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Create a new worktree and tmux session",
	Long: `Create a new git worktree with the specified name and create a tmux session for it.

The worktree will be created in the parent directory of the current repository.
A new branch with the same name will be created automatically.

Use --mirror to create an environment worktree that tracks a remote branch.
Environment worktrees are read-only and can be synced with 'grove sync'.

Examples:
  grove new feature-auth          # Create new worktree with branch feature-auth
  grove new staging --mirror origin/main  # Create environment worktree tracking origin/main`,
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

		// Check if worktree already exists
		if existingWt, _ := mgr.Find(name); existingWt != nil {
			return fmt.Errorf("worktree '%s' already exists\n\nOptions:\n  • Switch to it: grove to %s\n  • Remove it first: grove rm %s\n  • Use different name: grove new %s-v2",
				name, name, name, name)
		}

		var branchName string
		isEnvironment := newMirror != ""

		if isEnvironment {
			// Environment worktree - verify remote branch exists
			if !strings.Contains(newMirror, "/") {
				// Assume origin if no remote specified
				newMirror = "origin/" + newMirror
			}

			// Fetch to ensure we have latest refs
			fetchCmd := exec.Command("git", "-C", ctx.ProjectRoot, "fetch", "--prune")
			_ = fetchCmd.Run()

			// Verify the remote branch exists
			verifyCmd := exec.Command("git", "-C", ctx.ProjectRoot, "rev-parse", "--verify", newMirror)
			if err := verifyCmd.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: remote branch '%s' not found\n", newMirror)
				fmt.Fprintf(os.Stderr, "Run 'git fetch' and verify the branch exists\n")
				os.Exit(exitcode.ResourceNotFound)
			}

			// Use env/{name} as local branch for environment worktrees
			branchName = "env/" + name

			// Create worktree from the remote branch
			if err := mgr.CreateFromBranch(name, newMirror); err != nil {
				return fmt.Errorf("failed to create environment worktree: %w", err)
			}

			if !newJSON {
				fmt.Printf("✓ Created environment worktree '%s' tracking %s\n", name, newMirror)
			}
		} else {
			// Regular worktree - use name as branch name
			branchName = name
			if err := mgr.Create(name, branchName); err != nil {
				return fmt.Errorf("failed to create worktree: %w", err)
			}

			if !newJSON {
				fmt.Printf("✓ Created worktree '%s'\n", name)
			}
		}

		// Find the newly created worktree to get its path
		wt, err := mgr.Find(name)
		if err != nil || wt == nil {
			return fmt.Errorf("failed to find created worktree: %w", err)
		}

		// Register worktree in state
		now := time.Now()
		wsState := &state.WorktreeState{
			Path:           wt.Path,
			Branch:         branchName,
			Root:           false,
			CreatedAt:      now,
			LastAccessedAt: now,
			Environment:    isEnvironment,
		}

		if isEnvironment {
			wsState.Mirror = newMirror
			wsState.LastSyncedAt = &now
		}

		_ = ctx.State.AddWorktree(name, wsState)

		projectName := mgr.GetProjectName()

		// Create tmux session if tmux is available
		if tmux.IsTmuxAvailable() {
			sessionName := worktree.TmuxSessionName(projectName, name)
			if err := tmux.CreateSession(sessionName, wt.Path); err != nil {
				if !newJSON {
					fmt.Printf("⚠ Failed to create tmux session: %v\n", err)
				}
			} else if !newJSON {
				fmt.Printf("✓ Created tmux session '%s'\n", sessionName)
			}
		}

		// JSON output mode
		if newJSON {
			result := output.NewWorktreeResult{
				SwitchTo: wt.Path,
				Name:     name,
				Branch:   branchName,
				Path:     wt.Path,
				Created:  true,
			}
			data, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(data))
		}

		return nil
	}),
}

func init() {
	newCmd.Flags().BoolVarP(&newJSON, "json", "j", false, "Output as JSON with switch_to field")
	newCmd.Flags().StringVar(&newMirror, "mirror", "", "Create environment worktree tracking a remote branch (e.g., origin/main)")
	rootCmd.AddCommand(newCmd)
}

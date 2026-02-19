package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/LeahArmstrong/grove-cli/internal/exitcode"
	"github.com/LeahArmstrong/grove-cli/internal/hooks"
	"github.com/LeahArmstrong/grove-cli/internal/state"
	"github.com/LeahArmstrong/grove-cli/internal/tmux"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
	"github.com/spf13/cobra"
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
	Use:     "fork <name>",
	Aliases: []string{"split"},
	Short:   "Fork current worktree into a new one",
	Long: `Fork the current worktree, creating a new worktree branching from the current HEAD.

The new branch name will be {current-branch}-{name} unless --branch-name is specified.
By default, prompts to handle uncommitted changes. Use --move-wip, --copy-wip, or --no-wip to skip prompt.

Examples:
  grove fork feature-auth        # Fork into new worktree with branch main-feature-auth
  grove fork hotfix --branch-name emergency-fix  # Use specific branch name
  grove fork experiment --move-wip   # Move uncommitted changes to fork
  grove fork test --no-switch    # Fork but stay in current worktree`,
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
				fmt.Printf("Forking from environment worktree (mirror: %s)\n", ws.Mirror)
			}
		}

		// Determine new branch name
		newBranchName := forkBranchName
		if newBranchName == "" {
			newBranchName = fmt.Sprintf("%s-%s", currentTree.Branch, name)
		}

		// Check if branch already exists
		checkCmd := exec.Command("git", "-C", currentTree.Path, "show-ref", "--verify", "--quiet", "refs/heads/"+newBranchName)
		if err := checkCmd.Run(); err == nil {
			// Branch exists
			fmt.Fprintf(os.Stderr, "Error: branch '%s' already exists\n", newBranchName)
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
				if !isInteractive() {
					return fmt.Errorf("uncommitted changes detected; use --move-wip, --copy-wip, or --no-wip")
				}

				files, _ := wipHandler.ListWIPFiles()
				fmt.Printf("\n⚠ Uncommitted changes detected (%d files):\n", len(files))
				for i, f := range files {
					if i >= 5 {
						fmt.Printf("  ... and %d more\n", len(files)-5)
						break
					}
					fmt.Printf("  %s\n", f)
				}
				fmt.Println("\nHow do you want to handle them?")
				fmt.Println("  1. Move to fork (fork starts with changes, current becomes clean)")
				fmt.Println("  2. Copy to fork (both have changes)")
				fmt.Println("  3. Leave in current (fork starts clean)")
				fmt.Println("  4. Cancel")
				fmt.Print("\nChoice [1-4]: ")

				var choice string
				fmt.Scanln(&choice)

				switch choice {
				case "1":
					forkMoveWIP = true
				case "2":
					forkCopyWIP = true
				case "3":
					forkNoWIP = true
				default:
					fmt.Println("Cancelled")
					os.Exit(exitcode.UserCancelled)
				}
			}

			// Execute WIP handling
			if forkMoveWIP || forkCopyWIP {
				// Create patch from current changes
				wipPatch, err = wipHandler.CreatePatch()
				if err != nil {
					return fmt.Errorf("failed to capture changes: %w", err)
				}

				if forkMoveWIP {
					// Reset current working tree (changes will be applied to fork)
					resetCmd := exec.Command("git", "-C", currentTree.Path, "checkout", "--", ".")
					if output, err := resetCmd.CombinedOutput(); err != nil {
						return fmt.Errorf("failed to reset working tree: %w\n%s", err, output)
					}
					// Clean untracked files
					cleanCmd := exec.Command("git", "-C", currentTree.Path, "clean", "-fd")
					if output, err := cleanCmd.CombinedOutput(); err != nil {
						return fmt.Errorf("failed to clean untracked files: %w\n%s", err, output)
					}
				}
			}
		}

		// Create branch from base reference
		createBranchCmd := exec.Command("git", "-C", currentTree.Path, "branch", newBranchName, baseRef)
		if output, err := createBranchCmd.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: git operation failed: %s\n", output)
			os.Exit(exitcode.GitOperationFailed)
		}

		// Create worktree
		if err := mgr.CreateFromBranch(name, newBranchName); err != nil {
			// Cleanup: delete the branch we just created
			exec.Command("git", "-C", currentTree.Path, "branch", "-D", newBranchName).Run()
			return fmt.Errorf("failed to create worktree: %w", err)
		}

		// Find the created worktree
		newTree, err := mgr.Find(name)
		if err != nil || newTree == nil {
			return fmt.Errorf("failed to find created worktree")
		}

		fmt.Printf("✓ Created worktree '%s' with branch '%s'\n", name, newBranchName)

		// Apply WIP patch to new worktree if needed
		if len(wipPatch) > 0 && (forkMoveWIP || forkCopyWIP) {
			newWipHandler := worktree.NewWIPHandler(newTree.Path)
			if err := newWipHandler.ApplyPatch(wipPatch); err != nil {
				fmt.Printf("⚠ Failed to apply changes to fork: %v\n", err)
			} else {
				if forkMoveWIP {
					fmt.Println("✓ Moved uncommitted changes to fork")
				} else {
					fmt.Println("✓ Copied uncommitted changes to fork")
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
		_ = ctx.State.AddWorktree(name, wsState)

		// Fire post-create hook
		hookCtx := &hooks.Context{
			Worktree:     name,
			Config:       ctx.Config,
			WorktreePath: newTree.Path,
			MainPath:     ctx.ProjectRoot,
		}
		if err := hooks.Fire(hooks.EventPostCreate, hookCtx); err != nil {
			fmt.Printf("⚠ Post-create hook failed: %v\n", err)
		}

		projectName := mgr.GetProjectName()

		// Create tmux session
		if tmux.IsTmuxAvailable() {
			sessionName := worktree.TmuxSessionName(projectName, name)
			if err := tmux.CreateSession(sessionName, newTree.Path); err != nil {
				fmt.Printf("⚠ Failed to create tmux session: %v\n", err)
			} else {
				fmt.Printf("✓ Created tmux session '%s'\n", sessionName)
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
			data, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		// Switch to new worktree unless --no-switch
		if !forkNoSwitch {
			// Update last_worktree before switching
			_ = ctx.State.SetLastWorktree(parentName)

			// Store current session as last if inside tmux
			if tmux.IsInsideTmux() {
				currentSession, err := tmux.GetCurrentSession()
				if err == nil {
					tmux.StoreLastSession(currentSession)
				}
			}

			// Switch tmux session
			if tmux.IsTmuxAvailable() && tmux.IsInsideTmux() {
				sessionName := worktree.TmuxSessionName(projectName, name)
				if err := tmux.SwitchSession(sessionName); err != nil {
					fmt.Printf("⚠ Failed to switch session: %v\n", err)
				}
			}

			// Update last_accessed_at for target worktree
			_ = ctx.State.TouchWorktree(name)

			// Output directory change for shell integration
			hasShellIntegration := os.Getenv("GROVE_SHELL") == "1"
			if hasShellIntegration {
				fmt.Printf("cd:%s\n", newTree.Path)
			} else {
				fmt.Fprintf(os.Stderr, "\nNote: Directory switching requires shell integration.\n")
				fmt.Fprintf(os.Stderr, "To change directory manually:\n")
				fmt.Fprintf(os.Stderr, "  cd %s\n", newTree.Path)
			}
		} else {
			fmt.Printf("\nTo switch to the new worktree:\n  grove to %s\n", name)
		}

		return nil
	}),
}

// isInteractive checks if we're running interactively.
func isInteractive() bool {
	// Check if stdin is a terminal
	fileInfo, _ := os.Stdin.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
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

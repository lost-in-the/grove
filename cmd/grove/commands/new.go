package commands

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/LeahArmstrong/grove-cli/internal/cli"
	"github.com/LeahArmstrong/grove-cli/internal/exitcode"
	"github.com/LeahArmstrong/grove-cli/internal/log"
	"github.com/LeahArmstrong/grove-cli/internal/output"
	"github.com/LeahArmstrong/grove-cli/internal/tmux"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
)

var (
	newJSON     bool
	newMirror   string // Remote branch to mirror (e.g., "origin/main")
	newNoDocker bool   // Skip auto-starting Docker
	newBranch   string // Override branch name
	newFrom     string // Create branch from this ref
	newNoSwitch bool   // Skip auto-switching to the new worktree
)

var newCmd = &cobra.Command{
	Use:     "new <name>",
	Aliases: []string{"spawn"},
	Short:   "Create a new worktree and tmux session",
	Long: `Create a new git worktree with the specified name and create a tmux session for it.

The worktree will be created in the parent directory of the current repository.
A new branch with the same name will be created automatically.
Use --branch to override the branch name.
Use --from to specify the base ref for the new branch (default: HEAD).

When Docker agent stacks are configured, containers start automatically.
Use --no-docker to skip Docker auto-start.

Use --mirror to create an environment worktree that tracks a remote branch.
Environment worktrees are read-only and can be synced with 'grove sync'.

Examples:
  grove new feature-auth                          # Create worktree + tmux + Docker
  grove new feature-auth --branch custom-branch   # Use custom branch name
  grove new feature-auth --from develop            # Branch from develop
  grove new feature-auth --no-docker               # Skip Docker auto-start
  grove spawn feature-x                            # Alias (implies --json output)
  grove new staging --mirror origin/main           # Environment worktree tracking origin/main`,
	Args: cobra.ExactArgs(1),
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		// spawn alias implies JSON output
		if cmd.CalledAs() == "spawn" {
			newJSON = true
		}

		w := cli.NewStdout()
		stderr := cli.NewStderr()

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
				cli.Error(stderr, "remote branch '%s' not found", newMirror)
				cli.Faint(stderr, "Run 'git fetch' and verify the branch exists")
				os.Exit(exitcode.ResourceNotFound)
			}

			// Use env/{name} as local branch for environment worktrees
			branchName = "env/" + name

			// Create worktree from the remote branch
			if err := mgr.CreateFromBranch(name, newMirror); err != nil {
				return fmt.Errorf("failed to create environment worktree: %w", err)
			}

			if !newJSON {
				cli.Success(w, "Created environment worktree '%s' tracking %s", name, newMirror)
			}
		} else {
			// Regular worktree - use --branch if provided, otherwise name
			if newBranch != "" {
				branchName = newBranch
			} else {
				branchName = name
			}

			if newFrom != "" {
				// Create branch from specified ref
				if err := mgr.CreateFromRef(name, branchName, newFrom); err != nil {
					return fmt.Errorf("failed to create worktree: %w", err)
				}
			} else {
				if err := mgr.Create(name, branchName); err != nil {
					return fmt.Errorf("failed to create worktree: %w", err)
				}
			}

			if !newJSON {
				cli.Success(w, "Created worktree '%s'", name)
			}
		}

		// Post-create setup: find, symlink, state, hooks, docker
		wt, err := setupCreatedWorktree(ctx, mgr, name, branchName, worktreeSetupOpts{
			IsEnvironment: isEnvironment,
			Mirror:        newMirror,
			NoDocker:      newNoDocker,
			JSONOutput:    newJSON,
		}, w)
		if err != nil {
			return err
		}

		projectName := mgr.GetProjectName()

		// Create tmux session if tmux is available
		if tmux.IsTmuxAvailable() {
			sessionName := worktree.TmuxSessionName(projectName, name)
			if err := tmux.CreateSession(sessionName, wt.Path); err != nil {
				if !newJSON {
					cli.Warning(w, "Failed to create tmux session: %v", err)
				}
			} else if !newJSON {
				cli.Success(w, "Created tmux session '%s'", sessionName)
			}
		}

		// JSON output mode — return early to avoid cd: directive collision
		if newJSON {
			result := output.NewWorktreeResult{
				Name:    name,
				Branch:  branchName,
				Path:    wt.Path,
				Created: true,
			}
			if !newNoSwitch {
				result.SwitchTo = wt.Path
			}
			if err := output.PrintJSON(result); err != nil {
				return err
			}
			return nil
		}

		// Determine current worktree name for state tracking
		currentWorktreeName := ""
		if currentWt, _ := mgr.GetCurrent(); currentWt != nil {
			currentWorktreeName = currentWt.DisplayName()
		}

		// Switch to new worktree unless --no-switch
		if !newNoSwitch {
			// Update last_worktree before switching
			if currentWorktreeName != "" {
				if err := ctx.State.SetLastWorktree(currentWorktreeName); err != nil {
					log.Printf("failed to set last worktree %q: %v", currentWorktreeName, err)
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

			// Switch tmux session if inside tmux (skip in agent mode)
			var tmuxSwitched bool
			if !ctx.Config.AgentMode && tmux.IsTmuxAvailable() && tmux.IsInsideTmux() {
				sessionName := worktree.TmuxSessionName(projectName, name)
				if err := tmux.SwitchSession(sessionName); err != nil {
					cli.Warning(w, "Failed to switch session: %v", err)
				} else {
					tmuxSwitched = true
				}
			}

			// Update last_accessed_at for target worktree
			if err := ctx.State.TouchWorktree(name); err != nil {
				log.Printf("failed to touch worktree %q: %v", name, err)
			}

			// Skip cd directive when tmux switch already moved the user
			if !tmuxSwitched {
				hasShellIntegration := os.Getenv("GROVE_SHELL") == "1"
				if hasShellIntegration {
					cli.Directive("cd", wt.Path)
				} else {
					cli.Info(stderr, "Directory switching requires shell integration.")
					cli.Faint(stderr, "To change directory manually:")
					cli.Faint(stderr, "  cd %s", wt.Path)
				}
			}
		} else {
			cli.Info(w, "To switch to the new worktree: grove to %s", name)
		}

		return nil
	}),
}

func init() {
	newCmd.Flags().BoolVarP(&newJSON, "json", "j", false, "Output as JSON with switch_to field")
	newCmd.Flags().StringVarP(&newBranch, "branch", "b", "", "Branch name to create (default: worktree name)")
	newCmd.Flags().StringVarP(&newFrom, "from", "f", "", "Create branch from this ref (default: HEAD)")
	newCmd.Flags().StringVar(&newMirror, "mirror", "", "Create environment worktree tracking a remote branch (e.g., origin/main)")
	newCmd.Flags().BoolVar(&newNoDocker, "no-docker", false, "Skip Docker auto-start")
	newCmd.Flags().BoolVarP(&newNoSwitch, "no-switch", "n", false, "Stay in current worktree after creation")
	newCmd.MarkFlagsMutuallyExclusive("mirror", "from")
	newCmd.MarkFlagsMutuallyExclusive("mirror", "branch")
	rootCmd.AddCommand(newCmd)
}

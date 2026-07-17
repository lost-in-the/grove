package commands

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/cmdexec"
	"github.com/lost-in-the/grove/internal/exitcode"
	"github.com/lost-in-the/grove/internal/hooks"
	"github.com/lost-in-the/grove/internal/output"
	"github.com/lost-in-the/grove/internal/tmux"
	"github.com/lost-in-the/grove/internal/worktree"
)

var (
	newJSON       bool
	newMirror     string // Remote branch to mirror (e.g., "origin/main")
	newNoDocker   bool   // Skip auto-starting Docker
	newBranch     string // Override branch name
	newFrom       string // Create branch from this ref
	newFromBranch string // Check out an existing branch in the new worktree
	newDirty      bool   // Carry over `git diff HEAD` from the current worktree
	newNoSwitch   bool   // Skip auto-switching to the new worktree
	newNoTmux     bool   // Skip tmux session creation/switch
)

var newCmd = &cobra.Command{
	Use:     "new [name]",
	Aliases: []string{"spawn", "n"},
	Short:   "Create a new worktree and tmux session",
	Long: `Create a new git worktree with the specified name and create a tmux session for it.

The worktree will be created in the parent directory of the current repository.
A new branch with the same name will be created automatically.
Use --branch to override the branch name.
Use --from to specify the base ref for the new branch (default: HEAD).

When Docker agent stacks are configured, containers start automatically.
Use --no-docker to skip Docker auto-start.

Use --no-tmux to skip tmux entirely for this invocation: no session is
created and your tmux client is not switched. Docker and hooks still run.

Use --mirror to create an environment worktree that tracks a remote branch.
Environment worktrees are read-only and can be synced with 'grove sync'.

Use --from-branch to check out an existing local or remote branch in the new
worktree (no new branch is created). Pair with --dirty to also carry over
uncommitted changes ('git diff HEAD' — working tree + staged) from the current
worktree. Useful when promoting an in-progress branch from your main checkout
into its own worktree.

Examples:
  grove new feature-auth                          # Create worktree + tmux + Docker
  grove new feature-auth --branch custom-branch   # Use custom branch name
  grove new feature-auth --from develop            # Branch from develop
  grove new feature-auth --no-docker               # Skip Docker auto-start
  grove spawn feature-x                            # Alias (implies --json output)
  grove new staging --mirror origin/main           # Environment worktree tracking origin/main
  grove new payments --from-branch feature/payments         # Adopt existing branch as worktree
  grove new payments --from-branch feature/payments --dirty # Also carry over uncommitted edits`,
	Args: cobra.MaximumNArgs(1),
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		// spawn alias implies JSON output
		if cmd.CalledAs() == "spawn" {
			newJSON = true
		}

		w := cli.NewStdout()
		stderr := cli.NewStderr()

		var name string
		if len(args) == 0 {
			if !cli.IsInteractive() {
				return fmt.Errorf("worktree name required: grove new <name>")
			}
			var err error
			name, err = cli.ReadLine("Worktree name: ")
			if err != nil {
				return err
			}
		} else {
			name = args[0]
		}

		if name == "" {
			return fmt.Errorf("worktree name cannot be empty")
		}
		if msg := worktree.ValidateWorktreeName(name); msg != "" {
			return fmt.Errorf("invalid worktree name '%s': %s", name, msg)
		}

		// --dirty only makes sense paired with --from-branch (adopting an
		// existing branch). For the default flow (new branch from HEAD) the
		// branch is empty and a working-tree carry-over has no obvious
		// destination semantics.
		if newDirty && newFromBranch == "" {
			return fmt.Errorf("--dirty requires --from-branch")
		}

		// Capture the diff BEFORE creating the worktree. If creation fails we
		// just discard the patch; if it succeeds we apply it in the new
		// worktree after setup. `git diff HEAD` combines staged + unstaged
		// tracked changes; untracked files are intentionally excluded.
		var dirtyPatch []byte
		if newDirty {
			out, diffErr := cmdexec.Output(context.TODO(), "git", []string{"-C", ctx.ProjectRoot, "diff", "HEAD"}, "", cmdexec.GitLocal)
			if diffErr != nil {
				return fmt.Errorf("failed to capture working-tree diff: %w", diffErr)
			}
			dirtyPatch = out
		}

		mgr, err := ctx.WorktreeManager()
		if err != nil {
			return err
		}

		// Quick path-based existence check — avoids List() with N parallel
		// dirty checks. If a non-standard or prunable worktree owns the name,
		// the eventual git worktree add will surface it.
		if mgr.PathExists(name) {
			return fmt.Errorf("worktree '%s' already exists\n\nOptions:\n  • Switch to it: grove to %s\n  • Remove it first: grove rm %s\n  • Use different name: grove new %s-v2",
				name, name, name, name)
		}

		// Fire pre-create config hooks (hooks.toml) before the worktree exists.
		// {{.new_path}} is the future directory; pre_create actions should set
		// working_dir = "main" since the "new" path is not present yet (B6). A
		// required action failing aborts before the worktree is created (B7).
		if err := runConfigHooks(cli.NewStderr(), hooks.EventPreCreate, mgr.GetProjectName(), name, newBranch, mgr.PathForName(name), "", ctx.ProjectRoot); err != nil {
			return err
		}

		var branchName string
		isEnvironment := newMirror != ""
		mirror := newMirror

		if isEnvironment {
			// Environment worktree - verify remote branch exists
			if !strings.Contains(mirror, "/") {
				// Assume origin if no remote specified
				mirror = "origin/" + mirror
			}

			// Fetch to ensure we have latest refs
			if _, err := cmdexec.CombinedOutput(context.TODO(), "git", []string{"-C", ctx.ProjectRoot, "fetch", "--prune"}, "", cmdexec.GitRemote); err != nil {
				cli.Warning(stderr, "git fetch failed: %v", err)
				cli.Faint(stderr, "Proceeding with local refs — remote branch may be stale")
			}

			// Verify the remote branch exists
			if err := cmdexec.Run(context.TODO(), "git", []string{"-C", ctx.ProjectRoot, "rev-parse", "--verify", mirror}, "", cmdexec.GitLocal); err != nil {
				cli.Error(stderr, "remote branch '%s' not found", mirror)
				cli.Faint(stderr, "Run 'git fetch' and verify the branch exists")
				os.Exit(exitcode.ResourceNotFound)
			}

			// Use env/{name} as local branch for environment worktrees. Must
			// actually create that branch (git worktree add -b) rather than
			// checking out the remote ref directly — the latter leaves the
			// worktree on a detached HEAD while state/JSON output/hooks still
			// report "env/{name}" as the branch, a branch that doesn't exist.
			branchName = "env/" + name

			// Create worktree with a new local branch tracking the remote ref
			if err := mgr.CreateFromRef(name, branchName, mirror); err != nil {
				return fmt.Errorf("failed to create environment worktree: %w", err)
			}

			if !newJSON {
				cli.Success(w, "Created environment worktree '%s' tracking %s", name, mirror)
			}
		} else if newFromBranch != "" {
			// Adopt an existing branch into a new worktree. No new branch is
			// created; the worktree checks out newFromBranch directly. Git
			// refuses if the branch is already checked out elsewhere — that
			// guardrail is intentional and surfaces to the user as-is.
			branchName = newFromBranch
			if err := mgr.CreateFromBranch(name, newFromBranch); err != nil {
				return fmt.Errorf("failed to create worktree from branch %q: %w", newFromBranch, err)
			}

			if !newJSON {
				cli.Success(w, "Created worktree '%s' from branch '%s'", name, newFromBranch)
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
			Mirror:        mirror,
			NoDocker:      newNoDocker,
			JSONOutput:    newJSON,
		}, w)
		if err != nil {
			return err
		}

		// Apply the diff captured before worktree creation. Empty diff is a
		// no-op (issued as informational text only); a non-empty diff that
		// fails to apply is surfaced as a warning rather than a hard error
		// because the worktree itself is intact and useful — the user can
		// inspect and re-apply manually.
		if newDirty {
			if len(dirtyPatch) == 0 {
				if !newJSON {
					cli.Info(w, "--dirty: no uncommitted changes to transfer")
				}
			} else if err := applyDirtyPatch(wt.Path, dirtyPatch); err != nil {
				cli.Warning(stderr, "--dirty: failed to apply patch to new worktree: %v", err)
				cli.Faint(stderr, "The patch is intact in your current worktree; resolve manually and re-apply.")
			} else if !newJSON {
				cli.Success(w, "Transferred uncommitted changes (--dirty)")
			}
		}

		projectName := mgr.GetProjectName()

		// Create tmux session if tmux is available (skip with --no-tmux; agent
		// mode intentionally still creates the detached session — see AGENTS.md)
		if !newNoTmux && tmux.IsTmuxAvailable() {
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

		// Determine current worktree name for state tracking. CurrentPath +
		// DisplayNameForPath skips the List()-with-N-dirty-checks that
		// GetCurrent would trigger.
		currentWorktreeName := ""
		if currentPath, err := mgr.CurrentPath(); err == nil {
			currentWorktreeName = mgr.DisplayNameForPath(currentPath)
		}

		// Switch to new worktree unless --no-switch. Agent mode, --no-tmux,
		// and tmux mode "off" suppress the client switch (terminal takeover).
		if !newNoSwitch {
			suppressTmux := effectiveTmuxMode(ctx.Config.Tmux.Mode, ctx.Config.AgentMode, newNoTmux, false) == tmuxModeOff
			sessionName := worktree.TmuxSessionName(projectName, name)
			if !switchToWorktree(ctx, stderr, currentWorktreeName, name, sessionName, wt.Path, suppressTmux) {
				emitCdOrExplain(stderr, wt.Path)
			}
		} else {
			cli.Info(w, "To switch to the new worktree: grove to %s", name)
		}

		return nil
	}),
}

// applyDirtyPatch writes the captured diff to a temp file alongside the new
// worktree and applies it via `git apply`. The temp file is removed regardless
// of apply outcome so a stray patch file doesn't litter the worktree root.
func applyDirtyPatch(worktreePath string, patch []byte) error {
	f, err := os.CreateTemp("", "grove-dirty-*.patch")
	if err != nil {
		return fmt.Errorf("create temp patch file: %w", err)
	}
	defer func() { _ = os.Remove(f.Name()) }()

	if _, err := f.Write(patch); err != nil {
		_ = f.Close()
		return fmt.Errorf("write temp patch file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close temp patch file: %w", err)
	}

	out, err := cmdexec.CombinedOutput(context.TODO(), "git", []string{"-C", worktreePath, "apply", f.Name()}, "", cmdexec.GitLocal)
	if err != nil {
		return fmt.Errorf("git apply: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func init() {
	newCmd.Flags().BoolVarP(&newJSON, "json", "j", false, "Output as JSON with switch_to field")
	newCmd.Flags().StringVarP(&newBranch, "branch", "b", "", "Branch name to create (default: worktree name)")
	newCmd.Flags().StringVarP(&newFrom, "from", "f", "", "Create branch from this ref (default: HEAD)")
	newCmd.Flags().StringVar(&newFromBranch, "from-branch", "", "Check out an existing branch in the new worktree (no new branch created)")
	newCmd.Flags().BoolVar(&newDirty, "dirty", false, "Carry over `git diff HEAD` from the current worktree (requires --from-branch)")
	newCmd.Flags().StringVar(&newMirror, "mirror", "", "Create environment worktree tracking a remote branch (e.g., origin/main)")
	newCmd.Flags().BoolVar(&newNoDocker, "no-docker", false, "Skip Docker auto-start")
	newCmd.Flags().BoolVar(&newNoTmux, "no-tmux", false, "Skip tmux session creation/switch for this invocation")
	newCmd.Flags().BoolVarP(&newNoSwitch, "no-switch", "n", false, "Stay in current worktree after creation")
	newCmd.MarkFlagsMutuallyExclusive("mirror", "from")
	newCmd.MarkFlagsMutuallyExclusive("mirror", "branch")
	newCmd.MarkFlagsMutuallyExclusive("mirror", "from-branch")
	newCmd.MarkFlagsMutuallyExclusive("from", "from-branch")
	newCmd.MarkFlagsMutuallyExclusive("branch", "from-branch")
	rootCmd.AddCommand(newCmd)
}

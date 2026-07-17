package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/log"
	"github.com/lost-in-the/grove/internal/output"
	"github.com/lost-in-the/grove/internal/tmux"
	"github.com/lost-in-the/grove/internal/worktree"
)

var (
	openNoCreate bool
	openCommand  string
	openNoPopup  bool
	openJSON     bool
	openNoDocker bool
)

func init() {
	openCmd.Flags().BoolVar(&openNoCreate, "no-create", false, "Only attach to existing worktree, error if not found")
	openCmd.Flags().StringVar(&openCommand, "command", "", "Override session command")
	openCmd.Flags().BoolVar(&openNoPopup, "no-popup", false, "Skip popup, use tmux switch/attach instead")
	openCmd.Flags().BoolVarP(&openJSON, "json", "j", false, "Output as JSON")
	openCmd.Flags().BoolVar(&openNoDocker, "no-docker", false, "Skip Docker auto-start")
	rootCmd.AddCommand(openCmd)
}

var openCmd = &cobra.Command{
	Use:               "open [name]",
	Aliases:           []string{"o"},
	Short:             "Open a worktree session (create if needed)",
	ValidArgsFunction: completeWorktreeNames,
	Long: `Open a worktree by creating it if needed, ensuring a tmux session exists,
launching the configured session command, and attaching.

This is idempotent — safe to run repeatedly. If the worktree and session
already exist, it reattaches without recreating.

The session command is configured in .grove/config.toml:

  [session]
  command = "claude"   # What to run (default: shell)
  popup = true         # Use tmux display-popup
  popup_width = "80%"
  popup_height = "80%"

Examples:
  grove open feature-x              # Create if needed + attach + launch command
  grove open feature-x --no-create  # Attach only, error if doesn't exist
  grove open feature-x --command sh # Override the session command`,
	Args: cobra.MaximumNArgs(1),
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		w := cli.NewStdout()
		stderr := cli.NewStderr()

		var name string
		if len(args) == 0 {
			selected, err := selectWorktree(ctx, "Open which worktree?")
			if err != nil {
				return err
			}
			name = selected
		} else {
			name = args[0]
		}

		if name == "" {
			return fmt.Errorf("worktree name cannot be empty")
		}

		mgr, err := ctx.WorktreeManager()
		if err != nil {
			return err
		}

		projectName := mgr.GetProjectName()

		// Step 1: Ensure worktree exists
		wt, err := mgr.Find(name)
		if err != nil {
			return fmt.Errorf("failed to find worktree '%s': %w", name, err)
		}
		created := false

		if wt == nil {
			if openNoCreate {
				return fmt.Errorf("worktree '%s' not found (use 'grove open %s' without --no-create to create it)", name, name)
			}

			// Create the worktree
			branchName := name
			if err := mgr.Create(name, branchName); err != nil {
				return fmt.Errorf("failed to create worktree: %w", err)
			}

			// Post-create setup: find, symlink, state, hooks, docker
			wt, err = setupCreatedWorktree(ctx, mgr, name, branchName, worktreeSetupOpts{
				NoDocker:   openNoDocker,
				JSONOutput: openJSON,
			}, w)
			if err != nil {
				return err
			}

			created = true
			if !openJSON {
				cli.Success(w, "Created worktree '%s'", name)
			}
		}

		// Operate on the resolved short name, not the raw argument — Find also
		// matches by branch or full directory name, and the canonical tmux
		// session name and state key are both keyed on the short name (B21).
		displayName := wt.DisplayName()

		// Step 2: Ensure tmux session exists — unless agent mode / tmux "off"
		// suppresses it (grove open outside tmux would otherwise run a blocking
		// attach and take over an agent's terminal, the thing agent mode exists
		// to prevent).
		tmuxMode := ctx.Config.Tmux.Mode
		if tmuxMode == "" {
			tmuxMode = tmuxModeAuto
		}
		tmuxMode = effectiveTmuxMode(tmuxMode, ctx.Config.AgentMode, false, false)

		if tmuxMode == tmuxModeOff || !tmux.IsTmuxAvailable() {
			if openJSON {
				return printOpenJSON(wt, displayName, created)
			}
			if !tmux.IsTmuxAvailable() {
				cli.Faint(stderr, "tmux not available, skipping session management")
			}
			// Only emit the raw cd: protocol line when the shell wrapper is
			// listening; otherwise explain how to switch manually.
			emitCdOrExplain(stderr, wt.Path)
			return nil
		}

		sessionName := worktree.TmuxSessionName(projectName, displayName)
		sessionExists, err := tmux.SessionExists(sessionName)
		if err != nil {
			return fmt.Errorf("failed to check session: %w", err)
		}

		// Determine the session command
		sessionCmd := ctx.Config.Session.Command
		if openCommand != "" {
			sessionCmd = openCommand
		}

		if !sessionExists {
			// Create session with command if configured
			if err := tmux.CreateSessionWithCommand(sessionName, wt.Path, sessionCmd); err != nil {
				return fmt.Errorf("failed to create session: %w", err)
			}
			if !openJSON {
				if sessionCmd != "" {
					cli.Success(w, "Created session '%s' running '%s'", sessionName, sessionCmd)
				} else {
					cli.Success(w, "Created session '%s'", sessionName)
				}
			}
		} else if sessionCmd != "" {
			// Session exists — check if command is already running
			pane, pErr := tmux.GetPaneInfo(sessionName)
			if pErr == nil && pane.IsShell() && pane.CurrentCommand != sessionCmd {
				if err := tmux.SendKeys(sessionName, sessionCmd); err != nil {
					if !openJSON {
						cli.Warning(w, "Session exists but failed to launch '%s': %v", sessionCmd, err)
					}
				} else if !openJSON {
					cli.Success(w, "Launched '%s' in existing session", sessionCmd)
				}
			}
		}

		// Update state
		if err := ctx.State.TouchWorktree(displayName); err != nil {
			log.Printf("failed to touch worktree %q: %v", displayName, err)
		}

		// JSON output mode
		if openJSON {
			return printOpenJSON(wt, displayName, created)
		}

		// Step 3: Attach — popup or switch/attach
		useCC := tmux.ShouldUseControlMode(ctx.Config.Tmux.ControlMode)
		usePopup := ctx.Config.Session.Popup != nil && *ctx.Config.Session.Popup && !openNoPopup

		if usePopup && tmux.IsInsideTmux() {
			width := ctx.Config.Session.PopupWidth
			height := ctx.Config.Session.PopupHeight
			return tmux.DisplayPopup(sessionName, width, height)
		}

		// Standard tmux attach/switch
		if tmux.IsInsideTmux() {
			return tmux.SwitchSession(sessionName)
		}

		// Outside tmux
		hasShellIntegration := os.Getenv("GROVE_SHELL") == "1"
		if hasShellIntegration {
			cli.Directive("cd", wt.Path)
			cli.TmuxAttachDirective(sessionName, useCC)
		} else {
			if useCC {
				return tmux.AttachSessionControlMode(sessionName)
			}
			return tmux.AttachSession(sessionName)
		}

		return nil
	}),
}

func printOpenJSON(wt *worktree.Worktree, name string, created bool) error {
	result := output.NewWorktreeResult{
		SwitchTo: wt.Path,
		Name:     name,
		Branch:   wt.Branch,
		Path:     wt.Path,
		Created:  created,
	}
	return output.PrintJSON(result)
}

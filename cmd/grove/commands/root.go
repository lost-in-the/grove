package commands

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/exitcode"
	"github.com/lost-in-the/grove/internal/grove"
	"github.com/lost-in-the/grove/internal/log"
	"github.com/lost-in-the/grove/internal/state"
	"github.com/lost-in-the/grove/internal/tui"
	"github.com/lost-in-the/grove/internal/updatecheck"
	"github.com/lost-in-the/grove/internal/version"
	"github.com/lost-in-the/grove/internal/worktree"
	"github.com/lost-in-the/grove/plugins/docker"
)

var (
	noUpdateNotifierFlag bool
	checkUpdateFlag      bool
)

// bareAction is what a bare `grove` invocation (no subcommand) should do.
type bareAction int

const (
	bareShowHelp  bareAction = iota // non-interactive or TUI disabled
	bareDiagnose                    // interactive but not in a grove project
	bareLaunchTUI                   // interactive, inside a grove project
)

// decideBareAction returns the action for a bare `grove` invocation.
// Help never depends on project state, so callers may pass inProject=false
// before running project detection and only detect when the result is not
// bareShowHelp.
func decideBareAction(isTTY, tuiDisabled, inProject bool) bareAction {
	switch {
	case !isTTY || tuiDisabled:
		return bareShowHelp
	case !inProject:
		return bareDiagnose
	default:
		return bareLaunchTUI
	}
}

var rootCmd = &cobra.Command{
	Use:   "grove",
	Short: "Zero-friction worktree management",
	Long: `Grove is a zero-friction worktree + tmux manager for developers.

It provides fast context switching between git worktrees with automatic
tmux session management. Every command completes in less than 500ms.

GETTING STARTED: Add shell integration to your ~/.zshrc or ~/.bashrc:
  eval "$(grove install zsh)"   # or bash

This enables directory switching and tab completion. Add --alias for a
shorthand alias (e.g. 'w'). Use 'grove install --help' for details.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		log.Printf("command: %s, args: %v", cmd.Name(), args)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Only launch TUI when:
		// 1. No subcommand was invoked (this RunE only fires for bare "grove")
		// 2. Both stdin AND stdout are TTYs — otherwise `grove > out.txt` or
		//    `grove | less` would render alt-screen escape sequences into the
		//    file/pipe (B34); require an interactive stdout too.
		// 3. TUI is not disabled via env var
		isTTY := term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
		tuiDisabled := os.Getenv("GROVE_TUI") == "0"
		if decideBareAction(isTTY, tuiDisabled, false) == bareShowHelp {
			return cmd.Help()
		}

		// Check if we're in a grove project
		groveDir, err := grove.IsGroveProject()
		if err != nil {
			return fmt.Errorf("failed to detect grove project: %w", err)
		}
		if decideBareAction(isTTY, tuiDisabled, groveDir != "") == bareDiagnose {
			// Interactive but not in a grove project: print the same
			// diagnosis the RequireGroveContext commands print and exit 10.
			// This runs before tea.NewProgram, so no terminal capability
			// queries are emitted and nothing can leak onto the prompt.
			// An explicit --check-update must still be honored here: the
			// os.Exit below skips PersistentPostRunE where it normally runs.
			if checkUpdateFlag {
				if err := updatecheck.CheckNow(os.Stderr, version.Version); err != nil {
					fmt.Fprintf(os.Stderr, "update check failed: %v\n", err)
				}
			}
			cwd, _ := os.Getwd()
			printNoGroveDiagnosis(cwd)
			os.Exit(exitcode.NotGroveProject)
		}

		// Same startup notices RequireGroveContext emits — this runs before the
		// alt-screen takes over so the stderr messages stay visible. The bare-
		// grove TUI is many users' only entry point, so skipping them here (as it
		// used to) hid the shell-integration and config-migration nudges from
		// exactly the audience most likely to need them.
		warnOutdatedShellIntegration()
		migrateGroveExcludes(groveDir)

		projectRoot := grove.MustProjectRoot(groveDir)

		mgr, err := worktree.NewManager(projectRoot)
		if err != nil {
			return fmt.Errorf("failed to initialize worktree manager: %w", err)
		}

		stateMgr, err := state.NewManager(groveDir)
		if err != nil {
			return fmt.Errorf("failed to initialize state: %w", err)
		}

		cfg, cfgErr := config.LoadFromGroveDir(groveDir)
		if cfgErr != nil {
			log.Printf("config load failed, using defaults: %v", cfgErr)
			cli.Warning(cli.NewStderr(), "Failed to load config, using defaults: %v", cfgErr)
			cfg = config.LoadDefaults()
		}
		pluginMgr := registerPlugins(cfg)

		switchPath, forceUp, err := tui.Run(mgr, stateMgr, projectRoot, pluginMgr)
		if err != nil {
			return err
		}

		if forceUp && switchPath != "" {
			dockerPlugin := docker.New()
			if initErr := dockerPlugin.Init(cfg); initErr == nil {
				_ = dockerPlugin.Up(switchPath, true)
			}
		}

		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if checkUpdateFlag {
			return updatecheck.CheckNow(os.Stderr, version.Version)
		}
		if updatecheck.Skip(noUpdateNotifierFlag, version.Version) {
			return nil
		}
		updatecheck.MaybeNotify(os.Stderr, version.Version)
		// Wait (bounded) for the async refresh: the process exits right
		// after this hook, and exiting kills the goroutine before it can
		// fetch and write the cache. The wait is a no-op when the cache is
		// fresh (the goroutine returns immediately without touching the
		// network).
		select {
		case <-updatecheck.RefreshAsync():
		case <-time.After(updatecheck.RefreshWaitBudget):
		}
		return nil
	},
}

// Execute runs the root command with the given context.
func Execute(ctx context.Context) error {
	return rootCmd.ExecuteContext(ctx)
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&noUpdateNotifierFlag, "no-update-notifier", false,
		"suppress the new-release notification on this run")
	rootCmd.PersistentFlags().BoolVar(&checkUpdateFlag, "check-update", false,
		"force a synchronous check for a newer grove release and exit")
}

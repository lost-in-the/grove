package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/exitcode"
	"github.com/lost-in-the/grove/internal/grove"
	"github.com/lost-in-the/grove/internal/hooks"
	"github.com/lost-in-the/grove/internal/log"
	"github.com/lost-in-the/grove/internal/plugins"
	"github.com/lost-in-the/grove/internal/shell"
	"github.com/lost-in-the/grove/internal/state"
	"github.com/lost-in-the/grove/internal/worktree"
	"github.com/lost-in-the/grove/plugins/docker"
)

// GroveContext holds the resolved grove project context
type GroveContext struct {
	GroveDir      string           // Path to .grove directory
	ProjectRoot   string           // Path to project root (parent of .grove)
	State         *state.Manager   // State manager instance
	Config        *config.Config   // Loaded configuration
	PluginManager *plugins.Manager // Plugin manager for status queries

	wtMgr *worktree.Manager // Memoized worktree manager (via WorktreeManager)
}

// WorktreeManager returns a lazily constructed, memoized worktree.Manager
// rooted at the project root. Commands should use this instead of calling
// worktree.NewManager(ctx.ProjectRoot) inline so construction and error
// wrapping live in one place.
func (c *GroveContext) WorktreeManager() (*worktree.Manager, error) {
	if c.wtMgr != nil {
		return c.wtMgr, nil
	}
	mgr, err := worktree.NewManager(c.ProjectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize worktree manager: %w", err)
	}
	c.wtMgr = mgr
	return mgr, nil
}

// RequireGroveContext wraps a command function to require grove project context.
// If not in a grove project, prints an error and exits with code 10.
func RequireGroveContext(fn func(cmd *cobra.Command, args []string, ctx *GroveContext) error) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		groveDir, err := grove.IsGroveProject()
		if err != nil {
			log.Printf("grove project detection failed: %v", err)
			return fmt.Errorf("failed to detect grove project: %w", err)
		}

		log.Printf("grove dir resolved to: %s", groveDir)

		// Warn if shell integration is outdated
		if v := os.Getenv("GROVE_SHELL_VERSION"); v != "" {
			if shellVer, err := strconv.Atoi(v); err == nil && shellVer < shell.ShellVersion {
				fmt.Fprintf(os.Stderr, "grove: shell integration outdated (v%d, current v%d) — run: grove setup\n", shellVer, shell.ShellVersion)
			}
		}

		if groveDir == "" {
			cwd, _ := os.Getwd()
			printNoGroveDiagnosis(cwd)
			os.Exit(exitcode.NotGroveProject)
			return nil // unreachable
		}

		migrateGroveExcludes(groveDir)

		// Create state manager
		stateMgr, err := state.NewManager(groveDir)
		if err != nil {
			return fmt.Errorf("failed to initialize state: %w", err)
		}

		// Load config from the resolved .grove directory (not cwd)
		// so that secondary worktrees use the main worktree's config
		cfg, err := config.LoadFromGroveDir(groveDir)
		if err != nil {
			log.Printf("config load failed, using defaults: %v", err)
			stderr := cli.NewStderr()
			cli.Warning(stderr, "Failed to load config, using defaults: %v", err)
			cfg = config.LoadDefaults()
		} else {
			log.Printf("config loaded, docker mode: %v", cfg.IsExternalDockerMode())
		}

		// Register plugins with the global hook registry
		pluginMgr := registerPlugins(cfg)
		log.Printf("plugins registered")

		ctx := &GroveContext{
			GroveDir:      groveDir,
			ProjectRoot:   grove.MustProjectRoot(groveDir),
			State:         stateMgr,
			Config:        cfg,
			PluginManager: pluginMgr,
		}

		// Drift detection: warn if cwd is a worktree that isn't in state.
		// Skip when running `grove adopt` itself — it's the resolution.
		if cmd.Name() != "adopt" {
			if cwd, err := os.Getwd(); err == nil {
				if reason := grove.DiagnoseDrift(cwd, ctx.ProjectRoot); reason == grove.ReasonDriftedWorktree {
					emitDriftNotice(cli.NewStderr(), filepath.Base(cwd), reason)
				}
			}
		}

		return fn(cmd, args, ctx)
	}
}

// migrateGroveExcludes self-heals the repository's git excludes on every
// project command (idempotent, one git call + one file read when already
// current) and surfaces the upgrade notice exactly once: older grove versions
// git-ignored .grove/config.toml — the committable project config — and
// waiting for the next `grove init`/`grove new` to migrate would leave
// `git add .grove/config.toml` silently refused in the meantime. Failures are
// logged, never fatal: excludes are a convenience, not a precondition.
// Modeled on the shell-integration version preflight above.
func migrateGroveExcludes(groveDir string) {
	projectRoot := grove.MustProjectRoot(groveDir)
	migrated, err := grove.EnsureGroveExcludes(projectRoot)
	if err != nil {
		log.Printf("git excludes migration: %v", err)
		// Fall through: the legacy-symlink notice is independent of the excludes.
	}
	// Surface the config-layout upgrade notice once. `migrated` covers repos
	// touched by a mid-development build (config.toml removed from the exclude
	// block); NeedsConfigMigrationNotice covers the case real upgraders hit —
	// released grove never excluded config.toml, so the only durable signal is
	// the legacy per-worktree config symlinks left in existing worktrees. Both
	// are gated so the message fires at most once.
	notify := grove.NeedsConfigMigrationNotice(groveDir, projectRoot)
	if migrated || notify {
		emitExcludesMigrationNotice()
	}
}

// emitExcludesMigrationNotice is the one-time (per repository) upgrade notice
// for the config-layout change. It fires only when a legacy exclude entry was
// actually removed — the migration is idempotent, so the message can never
// repeat. Kept in one place so init and the command context share wording.
func emitExcludesMigrationNotice() {
	fmt.Fprintln(os.Stderr, "grove: .grove/config.toml is now a committable project file — commit it to share config with your team")
	fmt.Fprintln(os.Stderr, "grove: existing worktrees may carry a legacy .grove/config.toml symlink (shows as untracked) — run 'grove doctor' for cleanup steps")
}

// printNoGroveDiagnosis prints the "not a grove project" diagnosis for cwd to
// stderr. Shared by RequireGroveContext and the bare-grove TUI path so the
// wording cannot drift between the CLI and TUI entry points.
func printNoGroveDiagnosis(cwd string) {
	stderr := cli.NewStderr()
	diag := grove.DiagnoseNoGrove(cwd)

	switch diag.Reason {
	case grove.ReasonNotGitRepo:
		cli.Error(stderr, "not a grove project — not inside a git repository")
	case grove.ReasonMainWorktreeMissingGrove:
		cli.Error(stderr, "not a grove project — main worktree has no .grove directory")
		fmt.Fprintln(os.Stderr)
		cli.Faint(stderr, "Run 'grove init' from the main worktree:")
		cli.Faint(stderr, "  cd %s && grove init", diag.MainWorktreePath)
	default:
		cli.Error(stderr, "not a grove project")
		fmt.Fprintln(os.Stderr)
		cli.Faint(stderr, "Run 'grove init' to initialize a new grove project,")
		cli.Faint(stderr, "or change to a directory containing a .grove folder.")
	}
}

// emitDriftNotice prints a non-fatal warning when the cwd is a git worktree
// that grove doesn't have in its state. The user can ignore the message;
// it's intended to nudge them toward `grove adopt`.
func emitDriftNotice(w *cli.Writer, name string, reason grove.DriftReason) {
	if reason != grove.ReasonDriftedWorktree {
		return
	}
	cli.Warning(w, "this worktree (%s) wasn't created by grove and isn't registered in state", name)
	cli.Faint(w, "run 'grove adopt' to bootstrap it (registers state, records excludes, runs hooks)")
}

var pluginsRegistered bool
var globalPluginManager *plugins.Manager

// registerPlugins initializes and registers plugin hooks with the global registry.
// Returns the plugin manager for status queries.
func registerPlugins(cfg *config.Config) *plugins.Manager {
	if pluginsRegistered {
		return globalPluginManager
	}
	pluginsRegistered = true

	mgr := plugins.NewManager(cfg)
	globalPluginManager = mgr

	dockerPlugin := docker.New()
	if err := mgr.Register(dockerPlugin); err != nil {
		// Docker not available — silently skip
		return mgr
	}
	if !dockerPlugin.Enabled() {
		return mgr
	}
	if err := dockerPlugin.RegisterHooks(hooks.GlobalRegistry()); err != nil {
		log.Printf("failed to register docker hooks: %v", err)
	}
	return mgr
}

// ExitWithCode exits the program with the given exit code.
// This is a helper for commands that need to exit with specific codes.
func ExitWithCode(code int) {
	os.Exit(code)
}

// PrintError prints a styled error message to stderr.
func PrintError(format string, args ...interface{}) {
	cli.Error(cli.NewStderr(), format, args...)
}

// PrintSuggestion prints a styled suggestion to stderr.
func PrintSuggestion(format string, args ...interface{}) {
	cli.Info(cli.NewStderr(), format, args...)
}

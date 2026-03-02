package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LeahArmstrong/grove-cli/internal/cli"
	"github.com/LeahArmstrong/grove-cli/internal/config"
	"github.com/LeahArmstrong/grove-cli/internal/exitcode"
	"github.com/LeahArmstrong/grove-cli/internal/grove"
	"github.com/LeahArmstrong/grove-cli/internal/hooks"
	"github.com/LeahArmstrong/grove-cli/internal/log"
	"github.com/LeahArmstrong/grove-cli/internal/plugins"
	"github.com/LeahArmstrong/grove-cli/internal/state"
	"github.com/LeahArmstrong/grove-cli/plugins/docker"
)

// GroveContext holds the resolved grove project context
type GroveContext struct {
	GroveDir      string           // Path to .grove directory
	ProjectRoot   string           // Path to project root (parent of .grove)
	State         *state.Manager   // State manager instance
	Config        *config.Config   // Loaded configuration
	PluginManager *plugins.Manager // Plugin manager for status queries
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

		if groveDir == "" {
			stderr := cli.NewStderr()
			cli.Error(stderr, "not a grove project")
			fmt.Fprintln(os.Stderr)
			cli.Faint(stderr, "Run 'grove setup' to initialize a new grove project,")
			cli.Faint(stderr, "or change to a directory containing a .grove folder.")
			os.Exit(exitcode.NotGroveProject)
			return nil // unreachable
		}

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

		return fn(cmd, args, ctx)
	}
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

package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LeahArmstrong/grove-cli/internal/config"
	"github.com/LeahArmstrong/grove-cli/internal/exitcode"
	"github.com/LeahArmstrong/grove-cli/internal/grove"
	"github.com/LeahArmstrong/grove-cli/internal/hooks"
	"github.com/LeahArmstrong/grove-cli/internal/log"
	"github.com/LeahArmstrong/grove-cli/internal/state"
	"github.com/LeahArmstrong/grove-cli/plugins/docker"
)

// GroveContext holds the resolved grove project context
type GroveContext struct {
	GroveDir    string         // Path to .grove directory
	ProjectRoot string         // Path to project root (parent of .grove)
	State       *state.Manager // State manager instance
	Config      *config.Config // Loaded configuration
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
			fmt.Fprintln(os.Stderr, "Error: not a grove project")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "Run 'grove setup' to initialize a new grove project,")
			fmt.Fprintln(os.Stderr, "or change to a directory containing a .grove folder.")
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
			cfg = config.LoadDefaults()
		} else {
			log.Printf("config loaded, docker mode: %v", cfg.IsExternalDockerMode())
		}

		// Register plugins with the global hook registry
		registerPlugins(cfg)
		log.Printf("plugins registered")

		ctx := &GroveContext{
			GroveDir:    groveDir,
			ProjectRoot: grove.MustProjectRoot(groveDir),
			State:       stateMgr,
			Config:      cfg,
		}

		return fn(cmd, args, ctx)
	}
}

var pluginsRegistered bool

// registerPlugins initializes and registers plugin hooks with the global registry.
func registerPlugins(cfg *config.Config) {
	if pluginsRegistered {
		return
	}
	pluginsRegistered = true

	dockerPlugin := docker.New()
	if err := dockerPlugin.Init(cfg); err != nil {
		// Docker not available — silently skip
		return
	}
	if !dockerPlugin.Enabled() {
		return
	}
	_ = dockerPlugin.RegisterHooks(hooks.GlobalRegistry())
}

// ExitWithCode exits the program with the given exit code.
// This is a helper for commands that need to exit with specific codes.
func ExitWithCode(code int) {
	os.Exit(code)
}

// PrintError prints an error message to stderr with the standard format.
func PrintError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
}

// PrintSuggestion prints a suggestion to stderr.
func PrintSuggestion(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Suggestion: "+format+"\n", args...)
}

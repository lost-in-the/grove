package commands

import (
	"fmt"
	"os"

	"github.com/LeahArmstrong/grove-cli/internal/exitcode"
	"github.com/LeahArmstrong/grove-cli/internal/grove"
	"github.com/LeahArmstrong/grove-cli/internal/state"
	"github.com/spf13/cobra"
)

// GroveContext holds the resolved grove project context
type GroveContext struct {
	GroveDir    string          // Path to .grove directory
	ProjectRoot string          // Path to project root (parent of .grove)
	State       *state.Manager  // State manager instance
}

// RequireGroveContext wraps a command function to require grove project context.
// If not in a grove project, prints an error and exits with code 10.
func RequireGroveContext(fn func(cmd *cobra.Command, args []string, ctx *GroveContext) error) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		groveDir, err := grove.IsGroveProject()
		if err != nil {
			return fmt.Errorf("failed to detect grove project: %w", err)
		}

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

		ctx := &GroveContext{
			GroveDir:    groveDir,
			ProjectRoot: grove.MustProjectRoot(groveDir),
			State:       stateMgr,
		}

		return fn(cmd, args, ctx)
	}
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

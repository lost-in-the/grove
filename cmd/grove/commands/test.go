package commands

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/worktree"
	"github.com/lost-in-the/grove/plugins/docker"
)

var testCmd = &cobra.Command{
	Use:   "test <name> [args...]",
	Short: "Run the configured test command in a worktree",
	Long: `Run the configured test command in a specified worktree's directory.

The test command is configured in .grove/config.toml:

  [test]
  command = "bin/rails test"

Extra arguments are appended to the configured command:

  grove test my-feature spec/models/     # runs: bin/rails test spec/models/
  grove test my-feature 'spec/**/**.rb'  # globs expand in the target worktree`,
	Args: cobra.MinimumNArgs(1),
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		name := args[0]
		extraArgs := args[1:]

		if ctx.Config.Test.Command == "" {
			return fmt.Errorf("no test command configured\n\nAdd a [test] section to .grove/config.toml:\n\n  [test]\n  command = \"bin/rails test\"")
		}

		mgr, err := worktree.NewManager(ctx.ProjectRoot)
		if err != nil {
			return fmt.Errorf("failed to initialize worktree manager: %w", err)
		}

		targetTree, err := mgr.Find(name)
		if err != nil {
			return fmt.Errorf("failed to find worktree: %w", err)
		}
		if targetTree == nil {
			return fmt.Errorf("worktree '%s' not found", name)
		}

		fullCommand := ctx.Config.Test.Command
		if len(extraArgs) > 0 {
			fullCommand = fullCommand + " " + strings.Join(extraArgs, " ")
		}

		var runErr error

		if ctx.Config.Test.Service != "" {
			// Docker mode: run in an ephemeral container
			plugin := docker.New()
			if docker.HasActiveAgentSlot(ctx.Config, targetTree.Path) {
				plugin.SetIsolated(true)
			}
			if err := plugin.Init(ctx.Config); err != nil {
				return fmt.Errorf("failed to initialize docker plugin: %w", err)
			}
			runErr = plugin.Run(targetTree.Path, ctx.Config.Test.Service, fullCommand)
		} else {
			// Local mode: run directly in the worktree
			shellCmd := exec.Command("sh", "-c", fullCommand)
			shellCmd.Dir = targetTree.Path
			shellCmd.Stdout = os.Stdout
			shellCmd.Stderr = os.Stderr
			shellCmd.Stdin = os.Stdin
			runErr = shellCmd.Run()
		}

		if runErr != nil {
			if exitErr, ok := runErr.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			return fmt.Errorf("test command failed: %w", runErr)
		}

		return nil
	}),
}

func init() {
	testCmd.Flags().SetInterspersed(false)
	rootCmd.AddCommand(testCmd)
}

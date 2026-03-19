package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/worktree"
	claudeplugin "github.com/lost-in-the/grove/plugins/claude"
)

var sandboxJSON bool

func init() {
	sandboxStatusCmd.Flags().BoolVarP(&sandboxJSON, "json", "j", false, "Output as JSON")

	sandboxCmd.AddCommand(sandboxNewCmd)
	sandboxCmd.AddCommand(sandboxStartCmd)
	sandboxCmd.AddCommand(sandboxStopCmd)
	sandboxCmd.AddCommand(sandboxStatusCmd)
	sandboxCmd.AddCommand(sandboxExecCmd)
	sandboxCmd.AddCommand(sandboxRmCmd)
	rootCmd.AddCommand(sandboxCmd)
}

var sandboxCmd = &cobra.Command{
	Use:   "sandbox",
	Short: "Manage Claude Code devcontainer sandboxes",
	Long: `Manage devcontainer sandboxes for Claude Code per worktree.

Sandboxes provide network-isolated devcontainers with firewall rules
that make --dangerously-skip-permissions safe for unattended agent runs.

Examples:
  grove sandbox new feature-auth      # Build devcontainer for worktree
  grove sandbox start feature-auth    # Start the sandbox
  grove sandbox status                # Show all sandbox states
  grove sandbox exec feature-auth -- npm test  # Run command in sandbox
  grove sandbox rm feature-auth       # Stop and remove sandbox`,
}

var sandboxNewCmd = &cobra.Command{
	Use:   "new <worktree>",
	Short: "Build devcontainer for a worktree",
	Args:  cobra.ExactArgs(1),
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		stderr := cli.NewStderr()
		w := cli.NewStdout()

		wt, err := resolveWorktree(ctx.ProjectRoot, args[0])
		if err != nil {
			return err
		}

		cli.Step(stderr, "Building devcontainer for '%s'...", args[0])
		if err := claudeplugin.BuildSandbox(wt.Path); err != nil {
			return fmt.Errorf("failed to build sandbox: %w", err)
		}

		cli.Success(w, "Sandbox built for '%s'", args[0])
		return nil
	}),
}

var sandboxStartCmd = &cobra.Command{
	Use:   "start <worktree>",
	Short: "Start a devcontainer sandbox",
	Args:  cobra.ExactArgs(1),
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		stderr := cli.NewStderr()
		w := cli.NewStdout()

		wt, err := resolveWorktree(ctx.ProjectRoot, args[0])
		if err != nil {
			return err
		}

		cli.Step(stderr, "Starting sandbox for '%s'...", args[0])
		if err := claudeplugin.StartSandbox(wt.Path); err != nil {
			return fmt.Errorf("failed to start sandbox: %w", err)
		}

		cli.Success(w, "Sandbox started for '%s'", args[0])
		return nil
	}),
}

var sandboxStopCmd = &cobra.Command{
	Use:   "stop <worktree>",
	Short: "Stop a devcontainer sandbox",
	Args:  cobra.ExactArgs(1),
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		stderr := cli.NewStderr()
		w := cli.NewStdout()

		wt, err := resolveWorktree(ctx.ProjectRoot, args[0])
		if err != nil {
			return err
		}

		cli.Step(stderr, "Stopping sandbox for '%s'...", args[0])
		if err := claudeplugin.StopSandbox(wt.Path); err != nil {
			return fmt.Errorf("failed to stop sandbox: %w", err)
		}

		cli.Success(w, "Sandbox stopped for '%s'", args[0])
		return nil
	}),
}

var sandboxStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sandbox status for all worktrees",
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		mgr, err := worktree.NewManager(ctx.ProjectRoot)
		if err != nil {
			return fmt.Errorf("failed to initialize worktree manager: %w", err)
		}

		trees, err := mgr.List()
		if err != nil {
			return fmt.Errorf("failed to list worktrees: %w", err)
		}

		var paths []string
		for _, t := range trees {
			paths = append(paths, t.Path)
		}

		if sandboxJSON {
			data, err := claudeplugin.SandboxStatusJSON(paths)
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		w := cli.NewStdout()
		infos := claudeplugin.SandboxStatus(paths)
		if len(infos) == 0 {
			cli.Info(w, "No sandboxes found")
			return nil
		}

		fmt.Fprintf(os.Stdout, "%-20s %-12s %s\n", "WORKTREE", "STATUS", "CONTAINER")
		for _, info := range infos {
			fmt.Fprintf(os.Stdout, "%-20s %-12s %s\n", info.Worktree, info.Status, info.Container)
		}
		return nil
	}),
}

var sandboxExecCmd = &cobra.Command{
	Use:   "exec <worktree> -- <command>",
	Short: "Run a command inside a sandbox",
	Args:  cobra.MinimumNArgs(1),
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		worktreeName := args[0]
		var execArgs []string
		for i, a := range args {
			if a == "--" {
				execArgs = args[i+1:]
				break
			}
		}
		if len(execArgs) == 0 {
			return fmt.Errorf("no command specified — use: grove sandbox exec <worktree> -- <command>")
		}

		wt, err := resolveWorktree(ctx.ProjectRoot, worktreeName)
		if err != nil {
			return err
		}

		return claudeplugin.ExecInSandbox(wt.Path, execArgs)
	}),
}

var sandboxRmCmd = &cobra.Command{
	Use:   "rm <worktree>",
	Short: "Stop and remove a sandbox",
	Args:  cobra.ExactArgs(1),
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		stderr := cli.NewStderr()
		w := cli.NewStdout()

		wt, err := resolveWorktree(ctx.ProjectRoot, args[0])
		if err != nil {
			return err
		}

		cli.Step(stderr, "Removing sandbox for '%s'...", args[0])
		if err := claudeplugin.RemoveSandbox(wt.Path); err != nil {
			return fmt.Errorf("failed to remove sandbox: %w", err)
		}

		cli.Success(w, "Sandbox removed for '%s'", args[0])
		return nil
	}),
}

// resolveWorktree finds a worktree by name and returns it.
func resolveWorktree(projectRoot string, name string) (*worktree.Worktree, error) {
	mgr, err := worktree.NewManager(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize worktree manager: %w", err)
	}

	wt, err := mgr.Find(name)
	if err != nil {
		return nil, fmt.Errorf("failed to find worktree: %w", err)
	}
	if wt == nil {
		return nil, fmt.Errorf("worktree '%s' not found", name)
	}

	return wt, nil
}

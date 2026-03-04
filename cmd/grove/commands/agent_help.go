package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var agentHelpCmd = &cobra.Command{
	Use:   "agent-help",
	Short: "Quick reference for AI agent workflows",
	Long:  `Print a concise reference for AI agents using grove. Includes environment variables, common commands, and tips for programmatic use.`,
	Run: func(cmd *cobra.Command, args []string) {
		writeAgentHelp(cmd)
	},
}

func writeAgentHelp(cmd *cobra.Command) {
	w := cmd.OutOrStdout()

	// Resolve binary path
	binPath, err := os.Executable()
	if err != nil {
		binPath = "grove"
	} else {
		// Resolve symlinks for cleaner display
		if resolved, err := filepath.EvalSymlinks(binPath); err == nil {
			binPath = resolved
		}
	}

	// Detect project name if in a grove project
	project := "(not in a grove project)"
	if cwd, err := os.Getwd(); err == nil {
		project = detectProjectName(cwd)
	}

	_, _ = fmt.Fprintf(w, `# Grove Agent Quick Reference
# Binary: %s
# Project: %s
# Shell: set GROVE_SHELL=1 for directive output (cd:, tmux-attach:, env:)
# Agent mode: set GROVE_AGENT_MODE=1 to suppress tmux attachment

## Environment
  GROVE_AGENT_MODE=1       Suppress tmux attachment
  GROVE_NONINTERACTIVE=1   Skip interactive prompts
  GROVE_SHELL=1            Enable directive output

## Common Commands
  grove new <name>          Create worktree + branch + tmux session
  grove ls                  List worktrees (table)
  grove ls --json           List worktrees (JSON, machine-readable)
  grove to <name>           Switch to worktree
  grove to <name> --peek    Switch without hooks (lightweight)
  grove fetch pr/<N>        Create worktree from GitHub PR
  grove fetch issue/<N>     Create worktree from GitHub issue
  grove rm <name>           Remove worktree
  grove here                Show current worktree info
  grove doctor              Check system health

## Agent Tips
  - Call the binary directly, not the shell function
  - Use --json flag on ls/new/to for machine-readable output
  - Set GROVE_AGENT_MODE=1 to prevent tmux from taking over your terminal
  - grove new emits cd:/path directives — parse these to know where the worktree was created

## Full Documentation
  grove --help              All commands
  docs/AGENT_GUIDE.md       Comprehensive agent reference
`, binPath, project)
}

func init() {
	rootCmd.AddCommand(agentHelpCmd)
}

package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
# Agent mode: set GROVE_AGENT_MODE=1 to suppress session attachment (and see Docker note below)

## Environment
  GROVE_AGENT_MODE=1       Suppress session attachment; with agent stacks enabled, the stack is never
                           moved (run grove up --isolated for a stack that serves the worktree)
  GROVE_NONINTERACTIVE=1   Skip interactive prompts
  GROVE_SHELL=1            Enable directive output

## Common Commands
  grove new <name>          Create worktree + branch + its session
  grove ls                  List worktrees (table)
  grove ls --json           List worktrees (JSON, machine-readable)
  grove to <name>           Move to worktree (does not touch the stack by default)
  grove to <name> --up      Move and bring the stack
  grove to <name> --here    Stay in this shell: change directory only, hooks still run
  grove to <name> --no-hooks  Skip hooks and the dirty check
  grove to <name> --peek    Short for --here --no-hooks (no side effects)
  grove up                  Bring the stack to this worktree
  grove down                Stop the stack
  grove fetch pr/<N>        Create worktree from GitHub PR
  grove fetch issue/<N>     Create worktree from GitHub issue
  grove rm <name>           Remove worktree
  grove here                Show current worktree info
  grove doctor              Check system health

## Agent Tips
  - Call the binary directly, not the shell function
  - Use --json (-j) for machine-readable output. Errors are then JSON on stderr.
    --json works on: %s
  - Set GROVE_AGENT_MODE=1 to prevent a session from taking over your terminal
  - By default grove to does not touch the stack ([switch] container_switch = "auto" or "prompt"
    changes that). With agent stacks enabled, grove to --up leaves the stack where
    it is (a human may be using it). If the target has no isolated stack yet, it
    also prints an 'agent mode:' notice on stderr.
    Run grove up --isolated before running tests
  - grove new emits cd:/path directives — parse these to know where the worktree was created

## Full Documentation
  grove --help              All commands
  docs/AGENT_GUIDE.md       Comprehensive agent reference
`, binPath, project, strings.Join(jsonCommandNames(), ", "))
}

// jsonCommandNames lists every command that takes --json, read from the
// commands themselves so the reference cannot go stale.
func jsonCommandNames() []string {
	var names []string
	for _, c := range rootCmd.Commands() {
		if c.Flags().Lookup("json") != nil {
			names = append(names, c.Name())
		}
	}
	return names
}

func init() {
	rootCmd.AddCommand(agentHelpCmd)
}

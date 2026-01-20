package commands

import (
	"fmt"

	"github.com/LeahArmstrong/grove-cli/internal/shell"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [shell]",
	Short: "Generate shell integration code",
	Long: `Generate shell integration code for your shell (zsh or bash).

SETUP: Add this line to your shell config file, then restart your shell:

  # For zsh (~/.zshrc):
  eval "$(grove init zsh)"

  # For bash (~/.bashrc):
  eval "$(grove init bash)"

Or apply immediately without restart:
  source <(grove init zsh)

WHAT THIS ENABLES:
  • Directory switching - 'grove to <name>' changes your working directory
  • Tab completion - Complete commands and worktree names with <TAB>
  • 'w' alias - Shorthand for 'grove' command

WHY EVAL: The integration defines shell functions and aliases that must run
in your current shell (not a subprocess). Without eval, you'd just see the
script printed to stdout.

NOTE: This is the recommended setup. For native zsh completion files only
(without directory switching), use 'grove completion zsh' instead.`,
	ValidArgs: []string{"zsh", "bash"},
	Args:      cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		shellType := args[0]

		var output string
		var err error

		switch shellType {
		case "zsh":
			output, err = shell.GenerateZshIntegration()
		case "bash":
			output, err = shell.GenerateBashIntegration()
		default:
			return fmt.Errorf("unsupported shell: %s (supported: zsh, bash)", shellType)
		}

		if err != nil {
			return fmt.Errorf("failed to generate shell integration: %w", err)
		}

		fmt.Print(output)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

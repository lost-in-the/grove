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
	
Add this to your shell configuration file (~/.zshrc or ~/.bashrc):

  eval "$(grove init zsh)"  # for zsh
  eval "$(grove init bash)" # for bash

This enables:
- Directory switching after 'grove to' command
- Tab completion for worktree names`,
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

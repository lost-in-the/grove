package commands

import (
	"fmt"

	"github.com/LeahArmstrong/grove-cli/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show current configuration",
	Long:  `Display the current grove configuration including defaults and any overrides from config files.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		fmt.Printf("Alias: %s\n", cfg.Alias)
		fmt.Printf("Projects Directory: %s\n", cfg.ProjectsDir)
		fmt.Printf("Default Branch: %s\n", cfg.DefaultBranch)
		fmt.Printf("\nSwitch:\n")
		fmt.Printf("  Dirty Handling: %s\n", cfg.Switch.DirtyHandling)
		fmt.Printf("\nNaming:\n")
		fmt.Printf("  Pattern: %s\n", cfg.Naming.Pattern)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}

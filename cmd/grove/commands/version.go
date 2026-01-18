package commands

import (
	"fmt"

	"github.com/LeahArmstrong/grove-cli/internal/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  `Print the version number of grove along with build information.`,
	Run: func(cmd *cobra.Command, args []string) {
		if verbose, _ := cmd.Flags().GetBool("verbose"); verbose {
			fmt.Println(version.GetFullVersion())
		} else {
			fmt.Println(version.GetVersion())
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	versionCmd.Flags().BoolP("verbose", "v", false, "Print full version information")
}

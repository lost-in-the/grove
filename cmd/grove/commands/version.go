package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/lost-in-the/grove/internal/updatecheck"
	"github.com/lost-in-the/grove/internal/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  `Print the version number of grove along with build information.`,
	Run: func(cmd *cobra.Command, args []string) {
		var output string
		if verbose, _ := cmd.Flags().GetBool("verbose"); verbose {
			output = version.GetFullVersion()
		} else {
			output = version.GetVersion()
		}
		if term.IsTerminal(int(os.Stdout.Fd())) {
			output += updatecheck.CachedUpdateAnnotation(version.Version)
		}
		fmt.Println(output)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	versionCmd.Flags().BoolP("verbose", "v", false, "Print full version information")
}

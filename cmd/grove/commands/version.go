package commands

import (
	"fmt"

	"github.com/spf13/cobra"

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
		// Annotate only when we'd otherwise notify the user about updates.
		// Skip honors CI/non-TTY/dev-version/opt-out env vars (NO_UPDATE_NOTIFIER,
		// GROVE_NO_UPDATE_NOTIFIER, GROVE_AGENT_MODE, GROVE_NONINTERACTIVE).
		// We pass false for the --no-update-notifier flag because the version cmd
		// doesn't accept it; the env-var path covers the user-level opt-outs.
		if !updatecheck.Skip(false, version.Version) {
			output += updatecheck.CachedUpdateAnnotation(version.Version)
		}
		fmt.Println(output)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	versionCmd.Flags().BoolP("verbose", "v", false, "Print full version information")
}

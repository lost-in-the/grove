package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/LeahArmstrong/grove-cli/internal/cli"
	"github.com/LeahArmstrong/grove-cli/internal/output"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
	"github.com/LeahArmstrong/grove-cli/plugins/docker"
)

var whichJSON bool

type whichOutput struct {
	Worktree string              `json:"worktree"`
	Branch   string              `json:"branch"`
	Path     string              `json:"path"`
	Services *whichServiceOutput `json:"services,omitempty"`
}

type whichServiceOutput struct {
	RunningFor     string `json:"running_for"`
	MatchesCurrent bool   `json:"matches_current"`
}

var whichCmd = &cobra.Command{
	Use:     "which",
	Aliases: []string{"status"},
	Short:   "Show current worktree and service status",
	Long: `Display a quick overview of the current worktree and whether Docker services
are running for it. Lighter than 'grove here' — focused on operational context.

Examples:
  grove which        # Show worktree + service status
  grove which -j     # JSON output for scripts
  grove status       # Alias`,
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		mgr, err := worktree.NewManager(ctx.ProjectRoot)
		if err != nil {
			return fmt.Errorf("failed to initialize worktree manager: %w", err)
		}

		tree, err := mgr.GetCurrent()
		if err != nil {
			return fmt.Errorf("failed to get current worktree: %w", err)
		}
		if tree == nil {
			return fmt.Errorf("not in a grove worktree")
		}

		// Get service info from Docker plugin
		svcInfo := docker.CurrentServiceInfo(ctx.Config, tree.Path)

		if whichJSON {
			result := whichOutput{
				Worktree: tree.DisplayName(),
				Branch:   tree.Branch,
				Path:     tree.Path,
			}
			if svcInfo != nil {
				result.Services = &whichServiceOutput{
					RunningFor:     svcInfo.RunningFor,
					MatchesCurrent: svcInfo.MatchesCurrent,
				}
			}
			return output.PrintJSON(result)
		}

		// Formatted output
		w := cli.NewStdout()
		cli.Label(w, "Worktree: ", tree.DisplayName())
		cli.Label(w, "Branch:   ", tree.Branch)

		if svcInfo != nil {
			if svcInfo.IsRunning && svcInfo.MatchesCurrent {
				cli.Label(w, "Services: ", cli.StatusText(w, cli.StatusActive, fmt.Sprintf("running for %s", svcInfo.RunningFor)))
			} else if svcInfo.IsRunning {
				cli.Label(w, "Services: ", cli.StatusText(w, cli.StatusWarning, fmt.Sprintf("running for %s (not this worktree)", svcInfo.RunningFor)))
			} else {
				cli.Label(w, "Services: ", cli.StatusText(w, cli.StatusError, "not running"))
			}
		}

		return nil
	}),
}

func init() {
	whichCmd.Flags().BoolVarP(&whichJSON, "json", "j", false, "Output as JSON")
	rootCmd.AddCommand(whichCmd)
}

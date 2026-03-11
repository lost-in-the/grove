package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/exitcode"
	"github.com/lost-in-the/grove/internal/output"
	"github.com/lost-in-the/grove/plugins/docker"
)

var psJSON bool

type psSlotOutput struct {
	Slot    int    `json:"slot"`
	Name    string `json:"worktree"`
	Project string `json:"composeProject"`
	URL     string `json:"url,omitempty"`
}

func init() {
	psCmd.Flags().BoolVarP(&psJSON, "json", "j", false, "Output as JSON")
	rootCmd.AddCommand(psCmd)
}

var psCmd = &cobra.Command{
	Use:     "ps",
	Aliases: []string{"agent-status"},
	Short:   "Show active stacks",
	Long: `List all active Docker stacks, their reference IDs, and URLs.

Each stack started with 'grove up --isolated' or auto-started by 'grove new'
runs its own containers independently.

Examples:
  grove ps          # Show active stacks
  grove ps --json   # Machine-readable output`,
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		w := cli.NewStdout()

		slots, err := docker.ListActiveSlots(ctx.Config)
		if err != nil {
			if psJSON {
				output.PrintJSONError(exitcode.ExternalCommandFailed, fmt.Sprintf("failed to list stacks: %v", err))
				return nil
			}
			cli.Info(w, "Stacks not configured for this project")
			cli.Faint(w, "")
			cli.Faint(w, "To enable, add to .grove/config.toml:")
			cli.Faint(w, "")
			cli.Faint(w, "  [plugins.docker.external.agent]")
			cli.Faint(w, "  enabled = true")
			cli.Faint(w, "  services = [\"app\"]")
			cli.Faint(w, "  template_path = \"agent-stacks/template.yml\"")
			return nil
		}

		if psJSON {
			out := make([]psSlotOutput, 0, len(slots))
			for _, s := range slots {
				out = append(out, psSlotOutput{
					Slot:    s.Slot,
					Name:    s.Worktree,
					Project: docker.AgentComposeProjectName(ctx.Config, s.Slot),
					URL:     docker.AgentURL(ctx.Config, s.Slot),
				})
			}
			return output.PrintJSON(out)
		}

		if len(slots) == 0 {
			cli.Info(w, "No active stacks")
			return nil
		}

		maxSlots := 5
		if ctx.Config != nil && ctx.Config.Plugins.Docker.External != nil &&
			ctx.Config.Plugins.Docker.External.Agent != nil &&
			ctx.Config.Plugins.Docker.External.Agent.MaxSlots > 0 {
			maxSlots = ctx.Config.Plugins.Docker.External.Agent.MaxSlots
		}

		cli.Header(w, "STACKS (%d/%d)", len(slots), maxSlots)

		for _, s := range slots {
			url := docker.AgentURL(ctx.Config, s.Slot)
			if url != "" {
				cli.Bold(w, "  #%d  %-16s %s", s.Slot, s.Worktree, url)
			} else {
				cli.Bold(w, "  #%d  %s", s.Slot, s.Worktree)
			}
		}

		return nil
	}),
}

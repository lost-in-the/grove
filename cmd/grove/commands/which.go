package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/output"
	"github.com/lost-in-the/grove/plugins/docker"
)

var whichJSON bool

type whichOutput struct {
	Worktree string              `json:"worktree"`
	Branch   string              `json:"branch"`
	Path     string              `json:"path"`
	Services *whichServiceOutput `json:"services,omitempty"`
}

// whichServiceOutput.State is one of running, stopped, degraded, unknown,
// agent, or not_configured. ConfiguredFor is what the env file says the shared
// stack is configured for, not what the container has mounted.
type whichServiceOutput struct {
	State          string `json:"state"`
	ConfiguredFor  string `json:"configured_for,omitempty"`
	MatchesCurrent bool   `json:"matches_current"`
	Slot           int    `json:"slot,omitempty"`
}

type servicesDescription struct {
	Text  string
	Level cli.StatusLevel
	State string
	Hint  string
}

const mountCheckHint = "This is the env file setting. Run 'grove here --check-mount' to see what the containers have mounted."

// describeServices turns a service snapshot into the text, color level, JSON
// state, and optional hint `grove which` prints. displayName maps a worktree
// path to its short name.
func describeServices(info *docker.ServiceInfo, displayName func(string) string) servicesDescription {
	if info.Slot > 0 {
		return servicesDescription{Text: fmt.Sprintf("agent stack #%d", info.Slot), Level: cli.StatusActive, State: "agent"}
	}
	if info.Shared && info.ConfiguredPath == "" {
		return servicesDescription{Text: "not configured for any worktree", Level: cli.StatusWarning, State: "not_configured"}
	}

	d := servicesDescription{}
	if !info.Shared {
		if info.IsRunning {
			return servicesDescription{Text: "running", Level: cli.StatusActive, State: "running"}
		}
		if info.State == docker.StackUnknown {
			return servicesDescription{Text: "state unknown (docker did not answer)", Level: cli.StatusWarning, State: "unknown"}
		}
		return servicesDescription{Text: "not running", Level: cli.StatusError, State: "stopped"}
	}

	configured := fmt.Sprintf("configured for %s", displayName(info.ConfiguredPath))
	switch info.State {
	case docker.StackRunning:
		d.State, d.Level, d.Text = "running", cli.StatusActive, "running, "+configured
		if !info.MatchesCurrent {
			d.Level, d.Text = cli.StatusWarning, d.Text+" (not this worktree)"
		}
	case docker.StackDegraded:
		d.State, d.Level = "degraded", cli.StatusWarning
		d.Text = fmt.Sprintf("degraded (%s failed), %s", strings.Join(info.Blockers, ", "), configured)
	case docker.StackStopped:
		d.State, d.Level, d.Text = "stopped", cli.StatusError, "not running, "+configured
	default:
		d.State, d.Level = "unknown", cli.StatusWarning
		d.Text = "state unknown (docker did not answer), " + configured
	}
	if info.State != docker.StackStopped {
		d.Hint = mountCheckHint
	}
	return d
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
		mgr, err := ctx.WorktreeManager()
		if err != nil {
			return err
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
		var desc servicesDescription
		if svcInfo != nil {
			desc = describeServices(svcInfo, mgr.DisplayNameForPath)
		}

		if whichJSON {
			result := whichOutput{
				Worktree: tree.DisplayName(),
				Branch:   tree.Branch,
				Path:     tree.Path,
			}
			if svcInfo != nil {
				result.Services = &whichServiceOutput{
					State:          desc.State,
					MatchesCurrent: svcInfo.MatchesCurrent,
					Slot:           svcInfo.Slot,
				}
				if svcInfo.Shared && svcInfo.ConfiguredPath != "" {
					result.Services.ConfiguredFor = mgr.DisplayNameForPath(svcInfo.ConfiguredPath)
				}
			}
			return output.PrintJSON(result)
		}

		// Formatted output
		w := cli.NewStdout()
		cli.Label(w, "Worktree: ", tree.DisplayName())
		cli.Label(w, "Branch:   ", tree.Branch)

		if svcInfo != nil {
			cli.Label(w, "Services: ", cli.StatusText(w, desc.Level, desc.Text))
			if desc.Hint != "" {
				cli.Faint(w, "          %s", desc.Hint)
			}
		}

		return nil
	}),
}

func init() {
	whichCmd.Flags().BoolVarP(&whichJSON, "json", "j", false, "Output as JSON")
	rootCmd.AddCommand(whichCmd)
}

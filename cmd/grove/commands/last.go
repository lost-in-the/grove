package commands

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/output"
	"github.com/lost-in-the/grove/internal/tmux"
)

var lastJSON bool

var lastCmd = &cobra.Command{
	Use:     "last",
	Aliases: []string{"la"},
	Short:   "Switch to the previous worktree",
	Long:    `Switch to the last worktree you were working in.`,
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		stderr := cli.NewStderr()

		mgr, err := ctx.WorktreeManager()
		if err != nil {
			return err
		}

		// Try to get last worktree from state first (V2 approach)
		lastWorktree, err := ctx.State.GetLastWorktree()
		usedLegacyFallback := false
		if err != nil || lastWorktree == "" {
			// Fallback to tmux session tracking (legacy approach). This file is
			// global (cross-project), so whatever it yields is only a hint and
			// must be validated against this project's worktrees below.
			lastSession, serr := tmux.GetLastSession()
			if serr != nil || lastSession == "" {
				return noPreviousWorktree(stderr, lastJSON)
			}
			usedLegacyFallback = true

			projectName := mgr.GetProjectName()
			expectedPrefix := projectName + "-"
			if trimmed, found := strings.CutPrefix(lastSession, expectedPrefix); found {
				lastWorktree = trimmed
			} else {
				lastWorktree = lastSession
			}
		}

		// Find the target worktree
		targetTree, err := mgr.Find(lastWorktree)
		if err != nil {
			return fmt.Errorf("failed to find worktree: %w", err)
		}
		if targetTree == nil {
			// A stale global session from another project (the legacy fallback)
			// isn't an error — there's simply nothing to switch back to here.
			if usedLegacyFallback {
				return noPreviousWorktree(stderr, lastJSON)
			}
			return fmt.Errorf("last worktree '%s' not found", lastWorktree)
		}

		// Delegate to the shared switch flow so `grove last` behaves exactly
		// like `grove to <last>` — dirty handling, pre/post-switch hooks, and
		// correct --json ordering (no tmux relocation before the JSON is
		// emitted) all included, none of which the old lightweight epilogue did
		// (B19/B20).
		return performSwitch(ctx, targetTree.DisplayName(), lastJSON, false, false)
	}),
}

// noPreviousWorktree reports, without erroring, that there is no previous
// worktree to switch to in the current project. Running `grove last` before
// any in-project switch is a no-op, not a failure — and it must never chase a
// stale cross-project session recorded in the global last_session file.
//
// In --json mode it emits a valid JSON object (empty switch_to + a message)
// rather than a human sentence, so machine consumers always get parseable
// output on this path.
func noPreviousWorktree(stderr io.Writer, jsonMode bool) error {
	const msg = "No previous worktree in this project yet — switch with 'grove to <name>' first."
	if jsonMode {
		return output.PrintJSON(struct {
			SwitchTo string `json:"switch_to"`
			Message  string `json:"message"`
		}{SwitchTo: "", Message: msg})
	}
	_, _ = fmt.Fprintln(stderr, msg)
	return nil
}

func init() {
	lastCmd.Flags().BoolVarP(&lastJSON, "json", "j", false, "Output as JSON with switch_to field")
	rootCmd.AddCommand(lastCmd)
}

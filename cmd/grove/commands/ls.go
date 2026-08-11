package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/mux"
	"github.com/lost-in-the/grove/internal/output"
	"github.com/lost-in-the/grove/internal/plugins"
	"github.com/lost-in-the/grove/internal/worktree"
)

var (
	lsPaths bool
	lsJSON  bool
	lsQuiet bool
)

// lsWorktreeOutput represents a worktree in JSON output
type lsWorktreeOutput struct {
	Name        string            `json:"name"`
	FullName    string            `json:"full_name"`
	Branch      string            `json:"branch"`
	Path        string            `json:"path"`
	Status      string            `json:"status"`
	Tmux        string            `json:"tmux"`
	Agent       string            `json:"agent,omitempty"`
	Containers  string            `json:"containers,omitempty"`
	Current     bool              `json:"current"`
	Environment bool              `json:"environment,omitempty"`
	Services    []lsServiceStatus `json:"services,omitempty"`
}

// lsServiceStatus represents a plugin's status in JSON output.
type lsServiceStatus struct {
	Provider string `json:"provider"`
	Status   string `json:"status"`
	Level    string `json:"level"`
	Detail   string `json:"detail,omitempty"`
}

// lsOutput represents the JSON output structure for grove ls
type lsOutput struct {
	Project   string             `json:"project"`
	Current   string             `json:"current"`
	Worktrees []lsWorktreeOutput `json:"worktrees"`
}

var lsCmd = &cobra.Command{
	Use:     "ls",
	Aliases: []string{"list", "l"},
	Short:   "List all worktrees",
	Long:    `List all git worktrees with their status (clean/dirty) and branch information.`,
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		mgr, err := ctx.WorktreeManager()
		if err != nil {
			return err
		}

		// Fast path: quiet mode skips dirty checks entirely.
		if lsQuiet {
			names, err := mgr.ListNames()
			if err != nil {
				return fmt.Errorf("failed to list worktree names: %w", err)
			}
			for _, name := range names {
				fmt.Println(name)
			}
			return nil
		}

		// --paths only needs porcelain paths, so use the light listing and skip
		// List()'s N parallel `git status` calls (P5).
		if lsPaths {
			trees, err := mgr.ListLight()
			if err != nil {
				return fmt.Errorf("failed to list worktrees: %w", err)
			}
			for _, tree := range trees {
				fmt.Println(tree.Path)
			}
			return nil
		}

		trees, err := mgr.List()
		if err != nil {
			return fmt.Errorf("failed to list worktrees: %w", err)
		}

		if len(trees) == 0 {
			if !lsJSON {
				fmt.Println("No worktrees found")
			}
			return nil
		}

		// Get project name for tmux session naming
		projectName := mgr.GetProjectName()

		// Resolve current worktree from the trees we already fetched, rather
		// than calling GetCurrent (which re-runs List with N parallel git
		// status calls).
		var currentTree *worktree.Worktree
		if currentPath, err := mgr.CurrentPath(); err == nil {
			for _, t := range trees {
				if t.Path == currentPath {
					currentTree = t
					break
				}
			}
		}

		// One multiplexer listing serves every row's status.
		tmuxAvailable := ctx.Mux().Available()
		sessions := loadSessionIndex(ctx)

		// Collect plugin statuses
		var pluginStatuses map[string][]plugins.StatusEntry
		if ctx.PluginManager != nil {
			paths := make([]string, len(trees))
			for i, t := range trees {
				paths[i] = t.Path
			}
			pluginStatuses = ctx.PluginManager.CollectStatuses(paths)
		}

		// JSON mode
		if lsJSON {
			currentName := ""
			if currentTree != nil {
				currentName = currentTree.DisplayName()
			}

			result := lsOutput{
				Project:   projectName,
				Current:   currentName,
				Worktrees: make([]lsWorktreeOutput, 0, len(trees)),
			}

			for _, tree := range trees {
				status := statusClean
				if tree.IsPrunable {
					status = "stale"
				} else if tree.IsDirty {
					status = statusDirty
				}

				tmuxStatus := tmuxStatusNone
				if tmuxAvailable {
					tmuxStatus = tmuxStatusFor(tree, projectName, sessions)
				}

				isEnv, _ := ctx.State.IsEnvironment(tree.ShortName)
				isCurrent := currentTree != nil && tree.Path == currentTree.Path

				wo := lsWorktreeOutput{
					Name:        tree.DisplayName(),
					FullName:    tree.Name,
					Branch:      tree.Branch,
					Path:        tree.Path,
					Status:      status,
					Tmux:        tmuxStatus,
					Agent:       agentStatusDisplay(agentStatusFor(tree, projectName, sessions)),
					Current:     isCurrent,
					Environment: isEnv,
				}

				if entries, ok := pluginStatuses[tree.Path]; ok {
					wo.Containers = containersSummary(entries)
					for _, e := range entries {
						wo.Services = append(wo.Services, lsServiceStatus{
							Provider: e.ProviderName,
							Status:   e.Short,
							Level:    statusLevelString(e.Level),
							Detail:   e.Detail,
						})
					}
				}

				result.Worktrees = append(result.Worktrees, wo)
			}

			return output.PrintJSON(result)
		}

		// Default: formatted table output using cli.Table
		w := cli.NewStdout()

		// Check if any worktree has container status
		hasContainers := false
		for _, tree := range trees {
			if entries, ok := pluginStatuses[tree.Path]; ok {
				for _, e := range entries {
					if e.Short != "" {
						hasContainers = true
						break
					}
				}
			}
			if hasContainers {
				break
			}
		}

		// Build column definitions
		statusColorFn := func(value string) string {
			return cli.StatusText(w, cli.StatusLevel(value), value)
		}
		tmuxColorFn := func(value string) string {
			return cli.StatusText(w, cli.StatusLevel(value), value)
		}

		indicatorColorFn := func(value string) string {
			if value != "" {
				return cli.Accent(w, value)
			}
			return value
		}

		// Only backends that observe coding agents (herdr) fill this in, so the
		// column appears only when there is something to show.
		hasAgents := anyAgentReported(trees, projectName, sessions)
		agentColorFn := func(value string) string {
			return cli.StatusText(w, agentStatusLevel(mux.AgentStatus(value)), value)
		}

		columns := []cli.Column{
			{Title: "", MinWidth: 2, MaxWidth: 2, ColorFn: indicatorColorFn},
			{Title: "NAME", MaxWidth: 30},
			{Title: "BRANCH", MaxWidth: 25},
			{Title: "STATUS", MinWidth: 10, ColorFn: statusColorFn},
			{Title: sessionColumnTitle(ctx.Mux()), MinWidth: 12, ColorFn: tmuxColorFn},
		}
		if hasAgents {
			columns = append(columns, cli.Column{Title: "AGENT", MinWidth: 8, ColorFn: agentColorFn})
		}
		if hasContainers {
			columns = append(columns, cli.Column{Title: "CONTAINERS"})
		}
		columns = append(columns, cli.Column{Title: "PATH"})

		tbl := cli.NewTable(w, columns...)

		for _, tree := range trees {
			indicator := ""
			if currentTree != nil && tree.Path == currentTree.Path {
				indicator = "●"
			}

			status := statusClean
			if tree.IsPrunable {
				status = "stale"
			} else if tree.IsDirty {
				status = statusDirty
			}

			tmuxStatus := tmuxStatusNone
			if tmuxAvailable {
				tmuxStatus = tmuxStatusFor(tree, projectName, sessions)
			}

			containers := containersSummary(pluginStatuses[tree.Path])

			pathStr := tree.Path
			if isEnv, _ := ctx.State.IsEnvironment(tree.ShortName); isEnv {
				pathStr += " (env)"
			}

			row := []string{indicator, tree.DisplayName(), tree.Branch, status, tmuxStatus}
			if hasAgents {
				row = append(row, agentStatusDisplay(agentStatusFor(tree, projectName, sessions)))
			}
			if hasContainers {
				row = append(row, containers)
			}
			row = append(row, pathStr)
			tbl.AddRow(row...)
		}

		tbl.Render()

		return nil
	}),
}

// containersSummary joins each plugin status entry's short status with ","
// so JSON and table output agree when multiple providers report. Empty
// shorts are skipped.
func containersSummary(entries []plugins.StatusEntry) string {
	var parts []string
	for _, e := range entries {
		if e.Short != "" {
			parts = append(parts, e.Short)
		}
	}
	return strings.Join(parts, ",")
}

// tmuxStatusFor returns "attached", "detached", or "none" for a worktree.
// sessions may be nil (when no multiplexer is available).
func tmuxStatusFor(tree *worktree.Worktree, projectName string, sessions *mux.Index) string {
	if s, ok := sessions.Lookup(muxLookupTarget(projectName, tree.DisplayName(), tree.Path)); ok {
		return string(s.Status)
	}
	// Fall back to a session named after the worktree directory, which is how
	// sessions created outside grove (or under an older naming pattern) look.
	if s, ok := sessions.Lookup(mux.Target{Name: tree.Name}); ok {
		return string(s.Status)
	}
	return tmuxStatusNone
}

// agentStatusFor returns the coding-agent state a backend reports for a
// worktree's session, or mux.AgentUnreported when it cannot observe one.
func agentStatusFor(tree *worktree.Worktree, projectName string, sessions *mux.Index) mux.AgentStatus {
	if s, ok := sessions.Lookup(muxLookupTarget(projectName, tree.DisplayName(), tree.Path)); ok {
		return s.Agent
	}
	return mux.AgentUnreported
}

// agentStatusDisplay renders an agent state for a table cell. States where no
// agent was actually observed render blank rather than as a word — under tmux
// that is every row, and under herdr every worktree sitting at a shell prompt.
func agentStatusDisplay(a mux.AgentStatus) string {
	if !a.Observed() {
		return ""
	}
	return string(a)
}

// agentStatusLevel maps an agent state onto grove's semantic colors, ranked by
// how much the user needs to look at it: blocked is waiting on them, done
// finished unseen, working is in flight, and the rest are quiet.
func agentStatusLevel(a mux.AgentStatus) cli.StatusLevel {
	switch a {
	case mux.AgentBlocked:
		return cli.StatusError
	case mux.AgentDone:
		return cli.StatusOK
	case mux.AgentWorking:
		return cli.StatusWarning
	default:
		return cli.StatusNone
	}
}

// anyAgentReported reports whether any worktree has an observable agent, which
// is what decides if the AGENT column is worth its width.
func anyAgentReported(trees []*worktree.Worktree, projectName string, sessions *mux.Index) bool {
	for _, tree := range trees {
		if agentStatusFor(tree, projectName, sessions).Observed() {
			return true
		}
	}
	return false
}

func statusLevelString(level plugins.StatusLevel) string {
	switch level {
	case plugins.StatusActive:
		return "active"
	case plugins.StatusInfo:
		return "info"
	case plugins.StatusWarning:
		return "warning"
	case plugins.StatusError:
		return "error"
	default:
		return tmuxStatusNone
	}
}

func init() {
	lsCmd.Flags().BoolVarP(&lsPaths, "paths", "p", false, "Show full paths only (scriptable output)")
	lsCmd.Flags().BoolVarP(&lsJSON, "json", "j", false, "Output as JSON")
	lsCmd.Flags().BoolVarP(&lsQuiet, "quiet", "q", false, "Names only, one per line")
	rootCmd.AddCommand(lsCmd)
}

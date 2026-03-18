package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/output"
	"github.com/lost-in-the/grove/internal/plugins"
	"github.com/lost-in-the/grove/internal/tmux"
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
	FullName    string            `json:"fullName"`
	Branch      string            `json:"branch"`
	Path        string            `json:"path"`
	Status      string            `json:"status"`
	Tmux        string            `json:"tmux"`
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
		mgr, err := worktree.NewManager(ctx.ProjectRoot)
		if err != nil {
			return fmt.Errorf("failed to initialize worktree manager: %w", err)
		}

		// Fast paths: quiet and paths modes skip dirty checks entirely
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

		trees, err := mgr.List()
		if err != nil {
			return fmt.Errorf("failed to list worktrees: %w", err)
		}

		if len(trees) == 0 {
			if !lsPaths && !lsJSON {
				fmt.Println("No worktrees found")
			}
			return nil
		}

		// Paths only mode
		if lsPaths {
			for _, tree := range trees {
				fmt.Println(tree.Path)
			}
			return nil
		}

		// Get project name for tmux session naming
		projectName := mgr.GetProjectName()

		// Get current worktree to mark it
		currentTree, _ := mgr.GetCurrent()

		// Get tmux sessions for status
		tmuxAvailable := tmux.IsTmuxAvailable()
		var sessions map[string]*tmux.Session
		if tmuxAvailable {
			sessionList, err := tmux.ListSessions()
			if err == nil {
				sessions = make(map[string]*tmux.Session)
				for _, s := range sessionList {
					sessions[s.Name] = s
				}
			}
		}

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
				if tmuxAvailable && sessions != nil {
					sessionName := worktree.TmuxSessionName(projectName, tree.DisplayName())
					if session, ok := sessions[sessionName]; ok {
						if session.Attached {
							tmuxStatus = tmuxStatusAttached
						} else {
							tmuxStatus = tmuxStatusDetached
						}
					} else if session, ok := sessions[tree.Name]; ok {
						if session.Attached {
							tmuxStatus = tmuxStatusAttached
						} else {
							tmuxStatus = tmuxStatusDetached
						}
					}
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
					Current:     isCurrent,
					Environment: isEnv,
				}

				if entries, ok := pluginStatuses[tree.Path]; ok {
					for _, e := range entries {
						wo.Containers = e.Short
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

		columns := []cli.Column{
			{Title: "", MinWidth: 2, MaxWidth: 2, ColorFn: indicatorColorFn},
			{Title: "NAME", MaxWidth: 30},
			{Title: "BRANCH", MaxWidth: 25},
			{Title: "STATUS", MinWidth: 10, ColorFn: statusColorFn},
			{Title: "TMUX", MinWidth: 12, ColorFn: tmuxColorFn},
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

			containers := ""
			if entries, ok := pluginStatuses[tree.Path]; ok {
				var parts []string
				for _, e := range entries {
					if e.Short != "" {
						parts = append(parts, e.Short)
					}
				}
				containers = strings.Join(parts, ",")
			}

			pathStr := tree.Path
			if isEnv, _ := ctx.State.IsEnvironment(tree.ShortName); isEnv {
				pathStr += " (env)"
			}

			if hasContainers {
				tbl.AddRow(indicator, tree.DisplayName(), tree.Branch, status, tmuxStatus, containers, pathStr)
			} else {
				tbl.AddRow(indicator, tree.DisplayName(), tree.Branch, status, tmuxStatus, pathStr)
			}
		}

		tbl.Render()

		return nil
	}),
}

// tmuxStatusFor returns "attached", "detached", or "none" for a worktree.
// sessions may be nil (when tmux is not available).
func tmuxStatusFor(tree *worktree.Worktree, projectName string, sessions map[string]*tmux.Session) string {
	if sessions == nil {
		return tmuxStatusNone
	}
	sessionName := worktree.TmuxSessionName(projectName, tree.DisplayName())
	for _, key := range []string{sessionName, tree.Name} {
		if session, ok := sessions[key]; ok {
			if session.Attached {
				return tmuxStatusAttached
			}
			return tmuxStatusDetached
		}
	}
	return tmuxStatusNone
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

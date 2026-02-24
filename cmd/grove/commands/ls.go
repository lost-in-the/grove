package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/LeahArmstrong/grove-cli/internal/plugins"
	"github.com/LeahArmstrong/grove-cli/internal/tmux"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
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
	Aliases: []string{"list"},
	Short:   "List all worktrees",
	Long:    `List all git worktrees with their status (clean/dirty) and branch information.`,
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		mgr, err := worktree.NewManager(ctx.ProjectRoot)
		if err != nil {
			return fmt.Errorf("failed to initialize worktree manager: %w", err)
		}

		trees, err := mgr.List()
		if err != nil {
			return fmt.Errorf("failed to list worktrees: %w", err)
		}

		if len(trees) == 0 {
			if !lsQuiet && !lsPaths && !lsJSON {
				fmt.Println("No worktrees found")
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

		// Paths only mode
		if lsPaths {
			for _, tree := range trees {
				fmt.Println(tree.Path)
			}
			return nil
		}

		// Quiet mode - names only
		if lsQuiet {
			for _, tree := range trees {
				fmt.Println(tree.DisplayName())
			}
			return nil
		}

		// JSON mode
		if lsJSON {
			currentName := ""
			if currentTree != nil {
				currentName = currentTree.DisplayName()
			}

			output := lsOutput{
				Project:   projectName,
				Current:   currentName,
				Worktrees: make([]lsWorktreeOutput, 0, len(trees)),
			}

			for _, tree := range trees {
				status := "clean"
				if tree.IsPrunable {
					status = "stale"
				} else if tree.IsDirty {
					status = "dirty"
				}

				tmuxStatus := "none"
				if tmuxAvailable && sessions != nil {
					sessionName := worktree.TmuxSessionName(projectName, tree.DisplayName())
					if session, ok := sessions[sessionName]; ok {
						if session.Attached {
							tmuxStatus = "attached"
						} else {
							tmuxStatus = "detached"
						}
					} else if session, ok := sessions[tree.Name]; ok {
						if session.Attached {
							tmuxStatus = "attached"
						} else {
							tmuxStatus = "detached"
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

				output.Worktrees = append(output.Worktrees, wo)
			}

			jsonBytes, err := json.MarshalIndent(output, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Println(string(jsonBytes))
			return nil
		}

		// Build row data and compute dynamic column widths
		type lsRow struct {
			indicator  string
			name       string
			branch     string
			status     string
			tmux       string
			containers string
			path       string
			env        string
		}

		rows := make([]lsRow, 0, len(trees))
		nameW, branchW, containerW := len("NAME"), len("BRANCH"), 0

		for _, tree := range trees {
			r := lsRow{
				indicator: "  ",
				name:      tree.DisplayName(),
				branch:    tree.Branch,
				path:      tree.Path,
			}
			if currentTree != nil && tree.Path == currentTree.Path {
				r.indicator = "● "
			}
			if tree.IsPrunable {
				r.status = "stale"
			} else if tree.IsDirty {
				r.status = "dirty"
			} else {
				r.status = "clean"
			}
			r.tmux = "none"
			if tmuxAvailable && sessions != nil {
				sessionName := worktree.TmuxSessionName(projectName, tree.DisplayName())
				if session, ok := sessions[sessionName]; ok {
					if session.Attached {
						r.tmux = "attached"
					} else {
						r.tmux = "detached"
					}
				} else if session, ok := sessions[tree.Name]; ok {
					if session.Attached {
						r.tmux = "attached"
					} else {
						r.tmux = "detached"
					}
				}
			}
			if entries, ok := pluginStatuses[tree.Path]; ok {
				var parts []string
				for _, e := range entries {
					if e.Short != "" {
						parts = append(parts, e.Short)
					}
				}
				r.containers = strings.Join(parts, ",")
			}
			if isEnv, _ := ctx.State.IsEnvironment(tree.ShortName); isEnv {
				r.env = " (env)"
			}

			if len(r.name) > nameW {
				nameW = len(r.name)
			}
			if len(r.branch) > branchW {
				branchW = len(r.branch)
			}
			if len(r.containers) > containerW {
				containerW = len(r.containers)
			}
			rows = append(rows, r)
		}

		// Cap column widths for readability
		if nameW > 30 {
			nameW = 30
		}
		if branchW > 25 {
			branchW = 25
		}

		hasContainers := containerW > 0
		if hasContainers && containerW < len("CONTAINERS") {
			containerW = len("CONTAINERS")
		}

		// Default: formatted table output
		if hasContainers {
			fmtStr := fmt.Sprintf("%%s %%-%ds %%-%ds %%-10s %%-12s %%-%ds %%s\n", nameW, branchW, containerW)
			fmt.Printf(fmtStr, "", "NAME", "BRANCH", "STATUS", "TMUX", "CONTAINERS", "PATH")
		} else {
			fmtStr := fmt.Sprintf("%%s %%-%ds %%-%ds %%-10s %%-12s %%s\n", nameW, branchW)
			fmt.Printf(fmtStr, "", "NAME", "BRANCH", "STATUS", "TMUX", "PATH")
		}
		totalW := 3 + nameW + 1 + branchW + 1 + 10 + 1 + 12 + 1 + 40
		if hasContainers {
			totalW += containerW + 1
		}
		fmt.Println(strings.Repeat("─", totalW))

		for _, r := range rows {
			name := r.name
			if len(name) > nameW {
				name = name[:nameW-1] + "…"
			}
			branch := r.branch
			if len(branch) > branchW {
				branch = branch[:branchW-1] + "…"
			}
			if hasContainers {
				fmtStr := fmt.Sprintf("%%s %%-%ds %%-%ds %%-10s %%-12s %%-%ds %%s\n", nameW, branchW, containerW)
				fmt.Printf(fmtStr, r.indicator, name, branch, r.status, r.tmux, r.containers, r.path+r.env)
			} else {
				fmtStr := fmt.Sprintf("%%s %%-%ds %%-%ds %%-10s %%-12s %%s\n", nameW, branchW)
				fmt.Printf(fmtStr, r.indicator, name, branch, r.status, r.tmux, r.path+r.env)
			}
		}

		return nil
	}),
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
		return "none"
	}
}

func init() {
	lsCmd.Flags().BoolVarP(&lsPaths, "paths", "p", false, "Show full paths only (scriptable output)")
	lsCmd.Flags().BoolVarP(&lsJSON, "json", "j", false, "Output as JSON")
	lsCmd.Flags().BoolVarP(&lsQuiet, "quiet", "q", false, "Names only, one per line")
	rootCmd.AddCommand(lsCmd)
}

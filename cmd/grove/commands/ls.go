package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

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
	Name        string `json:"name"`
	FullName    string `json:"fullName"`
	Branch      string `json:"branch"`
	Path        string `json:"path"`
	Status      string `json:"status"`
	Tmux        string `json:"tmux"`
	Current     bool   `json:"current"`
	Environment bool   `json:"environment,omitempty"`
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

				output.Worktrees = append(output.Worktrees, lsWorktreeOutput{
					Name:        tree.DisplayName(),
					FullName:    tree.Name,
					Branch:      tree.Branch,
					Path:        tree.Path,
					Status:      status,
					Tmux:        tmuxStatus,
					Current:     isCurrent,
					Environment: isEnv,
				})
			}

			jsonBytes, err := json.MarshalIndent(output, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Println(string(jsonBytes))
			return nil
		}

		// Default: formatted table output
		fmt.Printf("%-3s %-12s %-15s %-10s %-12s %s\n", "", "NAME", "BRANCH", "STATUS", "TMUX", "PATH")
		fmt.Println("──────────────────────────────────────────────────────────────────────────────────────────")

		for _, tree := range trees {
			// Current indicator
			indicator := "  "
			if currentTree != nil && tree.Path == currentTree.Path {
				indicator = "● "
			}

			// Git status
			status := "clean"
			if tree.IsPrunable {
				status = "stale"
			} else if tree.IsDirty {
				status = "dirty"
			}

			// Tmux status - use consistent session naming: {project}-{name}
			tmuxStatus := "none"
			if tmuxAvailable && sessions != nil {
				// Session name follows the {project}-{shortname} pattern
				sessionName := worktree.TmuxSessionName(projectName, tree.DisplayName())
				if session, ok := sessions[sessionName]; ok {
					if session.Attached {
						tmuxStatus = "attached"
					} else {
						tmuxStatus = "detached"
					}
				} else if session, ok := sessions[tree.Name]; ok {
					// Fallback: check if session exists with full directory name
					if session.Attached {
						tmuxStatus = "attached"
					} else {
						tmuxStatus = "detached"
					}
				}
			}

			// Check environment status
			isEnv, _ := ctx.State.IsEnvironment(tree.ShortName)
			envIndicator := ""
			if isEnv {
				envIndicator = " (env)"
			}

			fmt.Printf("%s %-12s %-15s %-10s %-12s %s%s\n",
				indicator,
				tree.DisplayName(),
				tree.Branch,
				status,
				tmuxStatus,
				tree.Path,
				envIndicator)
		}

		return nil
	}),
}

func init() {
	lsCmd.Flags().BoolVarP(&lsPaths, "paths", "p", false, "Show full paths only (scriptable output)")
	lsCmd.Flags().BoolVarP(&lsJSON, "json", "j", false, "Output as JSON")
	lsCmd.Flags().BoolVarP(&lsQuiet, "quiet", "q", false, "Names only, one per line")
	rootCmd.AddCommand(lsCmd)
}

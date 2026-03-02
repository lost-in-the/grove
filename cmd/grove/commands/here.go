package commands

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/LeahArmstrong/grove-cli/internal/cli"
	"github.com/LeahArmstrong/grove-cli/internal/output"
	"github.com/LeahArmstrong/grove-cli/internal/tmux"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
	"github.com/LeahArmstrong/grove-cli/plugins/docker"
)

const (
	// maxDirtyFilesShown is the maximum number of dirty files to display
	maxDirtyFilesShown = 5
)

var (
	hereQuiet bool
	hereJSON  bool
)

// hereOutput represents the JSON output structure for grove here
type hereOutput struct {
	Name        string     `json:"name"`
	FullName    string     `json:"fullName"`
	Project     string     `json:"project"`
	Branch      string     `json:"branch"`
	Path        string     `json:"path"`
	Commit      commitInfo `json:"commit"`
	Status      string     `json:"status"`
	Changes     []string   `json:"changes,omitempty"`
	Tmux        tmuxInfo   `json:"tmux"`
	Environment bool       `json:"environment,omitempty"`
	Mirror      string     `json:"mirror,omitempty"`
	AgentSlot   int        `json:"agentSlot,omitempty"`
	AgentURL    string     `json:"agentURL,omitempty"`
}

type commitInfo struct {
	Hash      string `json:"hash"`
	ShortHash string `json:"shortHash"`
	Message   string `json:"message"`
	Age       string `json:"age"`
}

type tmuxInfo struct {
	Session string `json:"session"`
	Status  string `json:"status"`
}

var hereCmd = &cobra.Command{
	Use:   "here",
	Short: "Show current worktree information",
	Long:  `Display information about the current worktree including name, branch, and status.`,
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

		displayName := tree.DisplayName()

		// Quiet mode: just print the name
		if hereQuiet {
			fmt.Println(displayName)
			return nil
		}

		projectName := mgr.GetProjectName()
		tmuxSessionName := worktree.TmuxSessionName(projectName, tree.ShortName)
		tmuxStatus := tmux.GetSessionStatus(tmuxSessionName)

		// Fallback: check with directory basename
		if tmuxStatus == "none" {
			tmuxSessionName = filepath.Base(tree.Path)
			tmuxStatus = tmux.GetSessionStatus(tmuxSessionName)
		}

		// Get environment info from state
		isEnv, _ := ctx.State.IsEnvironment(displayName)
		mirror := ""
		if isEnv {
			if ws, err := ctx.State.GetWorktree(displayName); err == nil && ws != nil {
				mirror = ws.Mirror
			}
		}

		// Check for active agent stack
		agentSlot := docker.FindWorktreeSlot(ctx.Config, tree.Path)
		agentURL := ""
		if agentSlot > 0 {
			agentURL = docker.AgentURL(ctx.Config, agentSlot)
		}

		// JSON mode
		if hereJSON {
			status := "clean"
			if tree.IsDirty {
				status = "dirty"
			}

			var changes []string
			if tree.DirtyFiles != "" {
				changes = strings.Split(tree.DirtyFiles, "\n")
				filtered := make([]string, 0, len(changes))
				for _, c := range changes {
					if c != "" {
						filtered = append(filtered, c)
					}
				}
				changes = filtered
			}

			result := hereOutput{
				Name:     displayName,
				FullName: tree.Name,
				Project:  projectName,
				Branch:   tree.Branch,
				Path:     tree.Path,
				Commit: commitInfo{
					Hash:      tree.Commit,
					ShortHash: tree.ShortCommit,
					Message:   tree.CommitMessage,
					Age:       tree.CommitAge,
				},
				Status:  status,
				Changes: changes,
				Tmux: tmuxInfo{
					Session: tmuxSessionName,
					Status:  tmuxStatus,
				},
				Environment: isEnv,
				Mirror:      mirror,
				AgentSlot:   agentSlot,
				AgentURL:    agentURL,
			}

			return output.PrintJSON(result)
		}

		// Default: formatted output
		w := cli.NewStdout()

		cli.Header(w, "%s (%s)", displayName, tree.Branch)
		cli.Label(w, "Path:   ", tree.Path)
		cli.Label(w, "Branch: ", tree.Branch)

		// Show commit info
		if tree.ShortCommit != "" && tree.CommitMessage != "" {
			cli.Label(w, "Commit: ", fmt.Sprintf("%s - %s (%s)", tree.ShortCommit, tree.CommitMessage, tree.CommitAge))
		} else {
			cli.Label(w, "Commit: ", tree.Commit)
		}

		// Show status with color
		if tree.IsDirty {
			cli.Label(w, "Status: ", cli.StatusText(w, cli.StatusDirty, "● Dirty"))
		} else {
			cli.Label(w, "Status: ", cli.StatusText(w, cli.StatusClean, "✓ Clean"))
		}

		// Show dirty files if present
		if tree.IsDirty && tree.DirtyFiles != "" {
			lines := strings.Split(tree.DirtyFiles, "\n")
			if len(lines) > maxDirtyFilesShown {
				for i := 0; i < maxDirtyFilesShown; i++ {
					cli.Faint(w, "         %s", lines[i])
				}
				cli.Faint(w, "         ... and %d more", len(lines)-maxDirtyFilesShown)
			} else {
				for _, line := range lines {
					if line != "" {
						cli.Faint(w, "         %s", line)
					}
				}
			}
		}

		// Show environment info
		if isEnv {
			envValue := "environment"
			if mirror != "" {
				envValue = fmt.Sprintf("environment (mirror: %s)", mirror)
			}
			cli.Label(w, "Type:   ", envValue)
		}

		// Show agent stack info
		if agentSlot > 0 {
			cli.Label(w, "Stack:  ", fmt.Sprintf("isolated (slot %d)", agentSlot))
			if agentURL != "" {
				cli.Label(w, "URL:    ", agentURL)
			}
		}

		// Show tmux status
		tmuxValue := tmuxSessionName
		if tmuxStatus != "none" {
			tmuxValue = fmt.Sprintf("%s (%s)", tmuxSessionName, tmuxStatus)
		}
		cli.Label(w, "tmux:   ", tmuxValue)

		return nil
	}),
}

func init() {
	hereCmd.Flags().BoolVarP(&hereQuiet, "quiet", "q", false, "Just print the worktree name")
	hereCmd.Flags().BoolVarP(&hereJSON, "json", "j", false, "Output as JSON")
	rootCmd.AddCommand(hereCmd)
}

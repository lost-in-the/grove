package commands

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/LeahArmstrong/grove-cli/internal/tmux"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
	"github.com/spf13/cobra"
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

			output := hereOutput{
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
			}

			jsonBytes, err := json.MarshalIndent(output, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Println(string(jsonBytes))
			return nil
		}

		// Default: formatted output
		statusIcon := "✓ Clean"
		if tree.IsDirty {
			statusIcon = "● Dirty"
		}

		fmt.Printf("%s (%s)\n", displayName, tree.Branch)
		fmt.Println(strings.Repeat("━", 40))
		fmt.Printf("Path:    %s\n", tree.Path)
		fmt.Printf("Branch:  %s\n", tree.Branch)

		// Show commit info
		if tree.ShortCommit != "" && tree.CommitMessage != "" {
			fmt.Printf("Commit:  %s - %s (%s)\n", tree.ShortCommit, tree.CommitMessage, tree.CommitAge)
		} else {
			fmt.Printf("Commit:  %s\n", tree.Commit)
		}

		fmt.Printf("Status:  %s\n", statusIcon)

		// Show dirty files if present
		if tree.IsDirty && tree.DirtyFiles != "" {
			lines := strings.Split(tree.DirtyFiles, "\n")
			if len(lines) > maxDirtyFilesShown {
				for i := 0; i < maxDirtyFilesShown; i++ {
					fmt.Printf("         %s\n", lines[i])
				}
				fmt.Printf("         ... and %d more\n", len(lines)-maxDirtyFilesShown)
			} else {
				for _, line := range lines {
					if line != "" {
						fmt.Printf("         %s\n", line)
					}
				}
			}
		}

		// Show environment info
		if isEnv {
			fmt.Printf("Type:    environment")
			if mirror != "" {
				fmt.Printf(" (mirror: %s)", mirror)
			}
			fmt.Println()
		}

		// Show tmux status
		fmt.Printf("tmux:    %s", tmuxSessionName)
		if tmuxStatus != "none" {
			fmt.Printf(" (%s)", tmuxStatus)
		}
		fmt.Println()

		return nil
	}),
}

func init() {
	hereCmd.Flags().BoolVarP(&hereQuiet, "quiet", "q", false, "Just print the worktree name")
	hereCmd.Flags().BoolVarP(&hereJSON, "json", "j", false, "Output as JSON")
	rootCmd.AddCommand(hereCmd)
}

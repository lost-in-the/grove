package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/LeahArmstrong/grove-cli/internal/exitcode"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	syncAll  bool
	syncJSON bool
)

// SyncResult represents the JSON output for grove sync
type SyncResult struct {
	Synced  []SyncedWorktree `json:"synced"`
	Skipped []SkippedSync    `json:"skipped,omitempty"`
}

// SyncedWorktree represents a successfully synced worktree
type SyncedWorktree struct {
	Name       string `json:"name"`
	Mirror     string `json:"mirror"`
	OldCommit  string `json:"old_commit"`
	NewCommit  string `json:"new_commit"`
	CommitsAhead int  `json:"commits_ahead"`
}

// SkippedSync represents a worktree that was skipped
type SkippedSync struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

var syncCmd = &cobra.Command{
	Use:   "sync [name]",
	Short: "Sync environment worktrees with their mirrors",
	Long: `Sync environment worktrees with their remote tracking branches.

Environment worktrees (created with --mirror) track a remote branch.
This command fetches and fast-forwards those worktrees.

If no name is specified and current worktree is an environment, syncs current.
Use --all to sync all environment worktrees.

Examples:
  grove sync              # Sync current environment worktree
  grove sync production   # Sync specific environment worktree
  grove sync --all        # Sync all environment worktrees`,
	Args: cobra.MaximumNArgs(1),
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		mgr, err := worktree.NewManager(ctx.ProjectRoot)
		if err != nil {
			return fmt.Errorf("failed to initialize worktree manager: %w", err)
		}

		var targets []string

		if syncAll {
			// Sync all environment worktrees
			for _, name := range ctx.State.ListWorktrees() {
				ws, _ := ctx.State.GetWorktree(name)
				if ws != nil && ws.Environment {
					targets = append(targets, name)
				}
			}
			if len(targets) == 0 {
				fmt.Println("No environment worktrees found.")
				return nil
			}
		} else if len(args) > 0 {
			// Sync specific worktree
			targets = []string{args[0]}
		} else {
			// Sync current if it's an environment worktree
			currentTree, err := mgr.GetCurrent()
			if err != nil {
				return fmt.Errorf("failed to get current worktree: %w", err)
			}
			if currentTree == nil {
				return fmt.Errorf("could not determine current worktree")
			}

			isEnv, _ := ctx.State.IsEnvironment(currentTree.ShortName)
			if !isEnv {
				fmt.Fprintf(os.Stderr, "Error: current worktree '%s' is not an environment worktree\n", currentTree.ShortName)
				fmt.Fprintf(os.Stderr, "Use 'grove sync <name>' to specify an environment worktree, or --all\n")
				os.Exit(exitcode.ConstraintViolated)
			}
			targets = []string{currentTree.ShortName}
		}

		result := SyncResult{
			Synced:  []SyncedWorktree{},
			Skipped: []SkippedSync{},
		}

		for _, name := range targets {
			// Verify it's an environment worktree
			ws, _ := ctx.State.GetWorktree(name)
			if ws == nil {
				result.Skipped = append(result.Skipped, SkippedSync{
					Name:   name,
					Reason: "not found in state",
				})
				continue
			}

			if !ws.Environment {
				result.Skipped = append(result.Skipped, SkippedSync{
					Name:   name,
					Reason: "not an environment worktree",
				})
				if !syncAll {
					fmt.Fprintf(os.Stderr, "Error: '%s' is not an environment worktree\n", name)
					os.Exit(exitcode.ConstraintViolated)
				}
				continue
			}

			if ws.Mirror == "" {
				result.Skipped = append(result.Skipped, SkippedSync{
					Name:   name,
					Reason: "no mirror configured",
				})
				continue
			}

			// Get current commit before sync
			oldCommit, _ := getCurrentCommit(ws.Path)

			// Fetch from remote
			fetchCmd := exec.Command("git", "-C", ws.Path, "fetch", "--prune")
			if output, err := fetchCmd.CombinedOutput(); err != nil {
				if !syncJSON {
					fmt.Printf("  ⚠ Failed to fetch for '%s': %v\n%s", name, err, output)
				}
				result.Skipped = append(result.Skipped, SkippedSync{
					Name:   name,
					Reason: fmt.Sprintf("fetch failed: %v", err),
				})
				continue
			}

			// Fast-forward to mirror
			// Get the mirror ref (e.g., origin/main)
			mirror := ws.Mirror
			ffCmd := exec.Command("git", "-C", ws.Path, "merge", "--ff-only", mirror)
			if output, err := ffCmd.CombinedOutput(); err != nil {
				if !syncJSON {
					fmt.Printf("  ⚠ Failed to fast-forward '%s': %v\n%s", name, err, output)
				}
				result.Skipped = append(result.Skipped, SkippedSync{
					Name:   name,
					Reason: fmt.Sprintf("fast-forward failed (may have local changes): %v", err),
				})
				continue
			}

			// Get new commit after sync
			newCommit, _ := getCurrentCommit(ws.Path)

			// Count commits synced
			commitsAhead := 0
			if oldCommit != newCommit {
				countCmd := exec.Command("git", "-C", ws.Path, "rev-list", "--count", oldCommit+".."+newCommit)
				if output, err := countCmd.Output(); err == nil {
					fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &commitsAhead)
				}
			}

			// Update last_synced_at
			now := time.Now()
			ws.LastSyncedAt = &now
			_ = ctx.State.AddWorktree(name, ws)

			result.Synced = append(result.Synced, SyncedWorktree{
				Name:         name,
				Mirror:       mirror,
				OldCommit:    oldCommit,
				NewCommit:    newCommit,
				CommitsAhead: commitsAhead,
			})
		}

		// Output
		if syncJSON {
			data, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		// Human-readable output
		if len(result.Synced) > 0 {
			for _, s := range result.Synced {
				if s.CommitsAhead > 0 {
					fmt.Printf("✓ Synced '%s' (%s) - %d new commit(s)\n", s.Name, s.Mirror, s.CommitsAhead)
				} else {
					fmt.Printf("✓ '%s' is up to date with %s\n", s.Name, s.Mirror)
				}
			}
		}

		if len(result.Skipped) > 0 && !syncAll {
			for _, s := range result.Skipped {
				fmt.Printf("⚠ Skipped '%s': %s\n", s.Name, s.Reason)
			}
		}

		return nil
	}),
}

// getCurrentCommit returns the current HEAD commit SHA (short, 7 chars)
func getCurrentCommit(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	trimmed := strings.TrimSpace(string(output))
	if len(trimmed) < 7 {
		return trimmed, nil
	}
	return trimmed[:7], nil
}

func init() {
	syncCmd.Flags().BoolVar(&syncAll, "all", false, "Sync all environment worktrees")
	syncCmd.Flags().BoolVarP(&syncJSON, "json", "j", false, "Output as JSON")
	rootCmd.AddCommand(syncCmd)
}

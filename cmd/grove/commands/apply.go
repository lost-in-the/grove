package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/LeahArmstrong/grove-cli/internal/config"
	"github.com/LeahArmstrong/grove-cli/internal/exitcode"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	applyCommits bool
	applyWIP     bool
	applyPick    []string
	applyDryRun  bool
	applyJSON    bool
)

// ApplyResult represents the JSON output for grove apply.
type ApplyResult struct {
	Source         string       `json:"source"`
	Target         string       `json:"target"`
	CommitsApplied int          `json:"commits_applied,omitempty"`
	Commits        []CommitInfo `json:"commits,omitempty"`
	WIPApplied     bool         `json:"wip_applied,omitempty"`
	WIPFiles       []string     `json:"wip_files,omitempty"`
	DryRun         bool         `json:"dry_run,omitempty"`
	Success        bool         `json:"success"`
}

// CommitInfo represents a commit for JSON output.
type CommitInfo struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
}

var applyCmd = &cobra.Command{
	Use:   "apply <name>",
	Short: "Apply changes from another worktree",
	Long: `Apply commits or uncommitted changes from another worktree to the current one.

By default, applies both committed and uncommitted changes from the source worktree.
Use --commits or --wip to apply only one type of change.
Use --pick to apply specific commits by SHA.

Examples:
  grove apply feature-auth           # Apply all changes from feature-auth
  grove apply feature-auth --commits # Apply only commits (cherry-pick)
  grove apply feature-auth --wip     # Apply only uncommitted changes
  grove apply feature-auth --pick abc123,def456  # Apply specific commits
  grove apply feature-auth --dry-run # Show what would be applied`,
	Args: cobra.ExactArgs(1),
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		sourceName := args[0]

		// Load config to check constraints
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		mgr, err := worktree.NewManager(ctx.ProjectRoot)
		if err != nil {
			return fmt.Errorf("failed to initialize worktree manager: %w", err)
		}

		// Get current worktree (target)
		currentTree, err := mgr.GetCurrent()
		if err != nil {
			return fmt.Errorf("failed to get current worktree: %w", err)
		}
		if currentTree == nil {
			return fmt.Errorf("could not determine current worktree")
		}

		targetName := currentTree.DisplayName()

		// Check if target is immutable
		if cfg.IsImmutable(targetName) {
			msg := fmt.Sprintf("worktree '%s' is immutable and cannot receive changes", targetName)
			if applyJSON {
				printApplyJSONError(exitcode.ConstraintViolated, msg)
			} else {
				fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
				fmt.Fprintln(os.Stderr, "\nImmutable worktrees are protected from modifications.")
				fmt.Fprintln(os.Stderr, "Configure immutable worktrees in .grove/config.toml under [protection]")
			}
			os.Exit(exitcode.ConstraintViolated)
		}

		// Find source worktree
		sourceTree, err := mgr.Find(sourceName)
		if err != nil {
			return fmt.Errorf("failed to find worktree: %w", err)
		}
		if sourceTree == nil {
			msg := fmt.Sprintf("worktree '%s' not found", sourceName)
			if applyJSON {
				printApplyJSONError(exitcode.ResourceNotFound, msg)
			} else {
				fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
				fmt.Fprintln(os.Stderr, "\nUse 'grove ls' to see available worktrees.")
			}
			os.Exit(exitcode.ResourceNotFound)
		}

		result := ApplyResult{
			Source:  sourceTree.DisplayName(),
			Target:  targetName,
			DryRun:  applyDryRun,
			Success: true,
		}

		// Determine what to apply
		doCommits := applyCommits || len(applyPick) > 0 || (!applyCommits && !applyWIP)
		doWIP := applyWIP || (!applyCommits && !applyWIP && len(applyPick) == 0)

		// Don't apply WIP if --pick is specified
		if len(applyPick) > 0 {
			doWIP = false
		}

		if !applyJSON && !applyDryRun {
			fmt.Printf("Applying changes from %s → %s\n", result.Source, result.Target)
			fmt.Println(strings.Repeat("━", 50))
		}

		// Apply commits
		if doCommits {
			var commits []CommitInfo
			var err error

			if len(applyPick) > 0 {
				// Apply specific commits
				commits, err = applySpecificCommits(currentTree.Path, sourceTree.Path, applyPick, applyDryRun, applyJSON)
			} else {
				// Apply all commits since common ancestor
				commits, err = applyCommitsSinceAncestor(currentTree.Path, sourceTree, applyDryRun, applyJSON)
			}

			if err != nil {
				result.Success = false
				if applyJSON {
					result.Commits = commits
					printApplyResult(result)
				}
				os.Exit(exitcode.GitOperationFailed)
			}

			result.Commits = commits
			result.CommitsApplied = len(commits)
		}

		// Apply WIP
		if doWIP {
			wipFiles, err := applyWIPChanges(currentTree.Path, sourceTree.Path, applyDryRun, applyJSON)
			if err != nil {
				result.Success = false
				if applyJSON {
					result.WIPFiles = wipFiles
					printApplyResult(result)
				}
				os.Exit(exitcode.GitOperationFailed)
			}

			if len(wipFiles) > 0 {
				result.WIPApplied = true
				result.WIPFiles = wipFiles
			}
		}

		// Output results
		if applyJSON {
			printApplyResult(result)
			return nil
		}

		// Human-readable summary
		if applyDryRun {
			fmt.Println("\n[Dry run - no changes were made]")
		}

		if result.CommitsApplied == 0 && !result.WIPApplied {
			fmt.Println("\nNo changes to apply")
		} else {
			fmt.Println("\n✓ Changes applied successfully")
		}

		return nil
	}),
}

// applyCommitsSinceAncestor cherry-picks commits from source that are not in current.
func applyCommitsSinceAncestor(targetPath string, sourceTree *worktree.Worktree, dryRun, jsonOutput bool) ([]CommitInfo, error) {
	// Find merge base
	mergeBaseCmd := exec.Command("git", "-C", targetPath, "merge-base", "HEAD", sourceTree.Branch)
	baseOutput, err := mergeBaseCmd.Output()
	if err != nil {
		if !jsonOutput {
			fmt.Fprintf(os.Stderr, "Warning: could not find common ancestor with %s\n", sourceTree.Branch)
		}
		return nil, nil
	}
	mergeBase := strings.TrimSpace(string(baseOutput))

	// Get commits in source that are after merge base
	revListCmd := exec.Command("git", "-C", sourceTree.Path, "rev-list", "--reverse",
		fmt.Sprintf("%s..HEAD", mergeBase))
	revOutput, err := revListCmd.Output()
	if err != nil {
		return nil, nil // No commits to apply
	}

	shas := strings.Split(strings.TrimSpace(string(revOutput)), "\n")
	if len(shas) == 0 || (len(shas) == 1 && shas[0] == "") {
		if !jsonOutput && !dryRun {
			fmt.Println("\nNo commits to apply")
		}
		return nil, nil
	}

	return applySpecificCommits(targetPath, sourceTree.Path, shas, dryRun, jsonOutput)
}

// applySpecificCommits cherry-picks specific commits.
func applySpecificCommits(targetPath, sourcePath string, shas []string, dryRun, jsonOutput bool) ([]CommitInfo, error) {
	var commits []CommitInfo

	// Get commit info for each SHA
	for _, sha := range shas {
		sha = strings.TrimSpace(sha)
		if sha == "" {
			continue
		}

		// Handle comma-separated SHAs
		for _, s := range strings.Split(sha, ",") {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}

			// Get commit message
			msgCmd := exec.Command("git", "-C", sourcePath, "log", "-1", "--format=%s", s)
			msgOutput, err := msgCmd.Output()
			if err != nil {
				if !jsonOutput {
					fmt.Fprintf(os.Stderr, "Warning: commit %s not found in source\n", s)
				}
				continue
			}

			commits = append(commits, CommitInfo{
				SHA:     s,
				Message: strings.TrimSpace(string(msgOutput)),
			})
		}
	}

	if len(commits) == 0 {
		return nil, nil
	}

	if !jsonOutput {
		fmt.Printf("\nCommits to apply (%d):\n", len(commits))
		for _, c := range commits {
			shortSHA := c.SHA
			if len(shortSHA) > 7 {
				shortSHA = shortSHA[:7]
			}
			fmt.Printf("  %s %s\n", shortSHA, c.Message)
		}
	}

	if dryRun {
		return commits, nil
	}

	// Cherry-pick each commit
	if !jsonOutput {
		fmt.Println("\nApplying commits...")
	}

	for _, c := range commits {
		cherryCmd := exec.Command("git", "-C", targetPath, "cherry-pick", c.SHA)
		if output, err := cherryCmd.CombinedOutput(); err != nil {
			if !jsonOutput {
				fmt.Fprintf(os.Stderr, "\n✗ Conflict applying %s\n", c.SHA[:7])
				fmt.Fprintf(os.Stderr, "%s\n", output)
				fmt.Fprintln(os.Stderr, "\nTo resolve:")
				fmt.Fprintln(os.Stderr, "  1. Fix conflicts in the affected files")
				fmt.Fprintln(os.Stderr, "  2. git add <resolved-files>")
				fmt.Fprintln(os.Stderr, "  3. git cherry-pick --continue")
				fmt.Fprintln(os.Stderr, "\nOr to abort:")
				fmt.Fprintln(os.Stderr, "  git cherry-pick --abort")
			}
			return commits, fmt.Errorf("cherry-pick failed")
		}

		if !jsonOutput {
			fmt.Printf("  ✓ %s\n", c.SHA[:7])
		}
	}

	return commits, nil
}

// applyWIPChanges creates a patch from source's uncommitted changes and applies to target.
func applyWIPChanges(targetPath, sourcePath string, dryRun, jsonOutput bool) ([]string, error) {
	sourceHandler := worktree.NewWIPHandler(sourcePath)

	// Check if source has WIP
	hasWIP, err := sourceHandler.HasWIP()
	if err != nil {
		return nil, fmt.Errorf("failed to check source for WIP: %w", err)
	}

	if !hasWIP {
		if !jsonOutput && !dryRun {
			fmt.Println("\nNo uncommitted changes to apply")
		}
		return nil, nil
	}

	// Get list of WIP files
	wipFiles, err := sourceHandler.ListWIPFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to list WIP files: %w", err)
	}

	if !jsonOutput {
		fmt.Printf("\nUncommitted changes to apply (%d files):\n", len(wipFiles))
		for i, f := range wipFiles {
			if i >= 10 {
				fmt.Printf("  ... and %d more\n", len(wipFiles)-10)
				break
			}
			fmt.Printf("  %s\n", f)
		}
	}

	if dryRun {
		return wipFiles, nil
	}

	// Create patch from source
	patch, err := sourceHandler.CreatePatch()
	if err != nil {
		return wipFiles, fmt.Errorf("failed to create patch from source: %w", err)
	}

	if len(patch) == 0 {
		return nil, nil
	}

	// Apply patch to target
	targetHandler := worktree.NewWIPHandler(targetPath)
	if err := targetHandler.ApplyPatch(patch); err != nil {
		if !jsonOutput {
			fmt.Fprintf(os.Stderr, "\n✗ Failed to apply uncommitted changes\n")
			fmt.Fprintf(os.Stderr, "%v\n", err)
			fmt.Fprintln(os.Stderr, "\nThe patch may have conflicts with your current changes.")
			fmt.Fprintln(os.Stderr, "Resolve manually or try 'grove compare' to see differences first.")
		}
		return wipFiles, fmt.Errorf("failed to apply WIP patch")
	}

	if !jsonOutput {
		fmt.Println("\n✓ Uncommitted changes applied")
	}

	return wipFiles, nil
}

// printApplyResult prints the apply result as JSON.
func printApplyResult(result ApplyResult) {
	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
}

// printApplyJSONError prints an error in JSON format.
func printApplyJSONError(code int, message string) {
	errOutput := struct {
		Error   bool   `json:"error"`
		Code    int    `json:"code"`
		Message string `json:"message"`
	}{
		Error:   true,
		Code:    code,
		Message: message,
	}
	data, _ := json.MarshalIndent(errOutput, "", "  ")
	fmt.Fprintln(os.Stderr, string(data))
}

func init() {
	applyCmd.Flags().BoolVar(&applyCommits, "commits", false, "Apply only committed changes (cherry-pick)")
	applyCmd.Flags().BoolVar(&applyWIP, "wip", false, "Apply only uncommitted changes")
	applyCmd.Flags().StringSliceVar(&applyPick, "pick", nil, "Apply specific commit(s) by SHA")
	applyCmd.Flags().BoolVar(&applyDryRun, "dry-run", false, "Show what would be applied without making changes")
	applyCmd.Flags().BoolVarP(&applyJSON, "json", "j", false, "Output as JSON")

	applyCmd.MarkFlagsMutuallyExclusive("commits", "wip")

	rootCmd.AddCommand(applyCmd)
}

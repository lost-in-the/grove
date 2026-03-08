package commands

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/LeahArmstrong/grove-cli/internal/cli"
	"github.com/LeahArmstrong/grove-cli/internal/cmdexec"
	"github.com/LeahArmstrong/grove-cli/internal/exitcode"
	"github.com/LeahArmstrong/grove-cli/internal/output"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
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
		w := cli.NewStdout()
		stderr := cli.NewStderr()

		cfg := ctx.Config

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
				cli.Error(stderr, "%s", msg)
				cli.Faint(stderr, "Immutable worktrees are protected from modifications.")
				cli.Faint(stderr, "Configure immutable worktrees in .grove/config.toml under [protection]")
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
				cli.Error(stderr, "%s", msg)
				cli.Faint(stderr, "Use 'grove ls' to see available worktrees.")
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
			cli.Header(w, "Applying changes from %s → %s", result.Source, result.Target)
		}

		// Apply commits
		if doCommits {
			var commits []CommitInfo
			var err error

			if len(applyPick) > 0 {
				// Apply specific commits
				commits, err = applySpecificCommits(w, stderr, currentTree.Path, sourceTree.Path, applyPick, applyDryRun, applyJSON)
			} else {
				// Apply all commits since common ancestor
				commits, err = applyCommitsSinceAncestor(w, stderr, currentTree.Path, sourceTree, applyDryRun, applyJSON)
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
			wipFiles, err := applyWIPChanges(w, stderr, currentTree.Path, sourceTree.Path, applyDryRun, applyJSON)
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
			cli.Faint(w, "[Dry run - no changes were made]")
		}

		if result.CommitsApplied == 0 && !result.WIPApplied {
			cli.Info(w, "No changes to apply")
		} else {
			cli.Success(w, "Changes applied successfully")
		}

		return nil
	}),
}

// applyCommitsSinceAncestor cherry-picks commits from source that are not in current.
func applyCommitsSinceAncestor(w, stderr *cli.Writer, targetPath string, sourceTree *worktree.Worktree, dryRun, jsonOutput bool) ([]CommitInfo, error) {
	// Find merge base
	baseOutput, err := cmdexec.Output(context.TODO(), "git", []string{"-C", targetPath, "merge-base", "HEAD", sourceTree.Branch}, "", cmdexec.GitLocal)
	if err != nil {
		if !jsonOutput {
			cli.Warning(stderr, "could not find common ancestor with %s", sourceTree.Branch)
		}
		return nil, nil
	}
	mergeBase := strings.TrimSpace(string(baseOutput))

	// Get commits in source that are after merge base
	revOutput, err := cmdexec.Output(context.TODO(), "git", []string{"-C", sourceTree.Path, "rev-list", "--reverse",
		fmt.Sprintf("%s..HEAD", mergeBase)}, "", cmdexec.GitLocal)
	if err != nil {
		return nil, fmt.Errorf("failed to list commits since common ancestor: %w", err)
	}

	shas := strings.Split(strings.TrimSpace(string(revOutput)), "\n")
	if len(shas) == 0 || (len(shas) == 1 && shas[0] == "") {
		if !jsonOutput && !dryRun {
			cli.Info(w, "No commits to apply")
		}
		return nil, nil
	}

	return applySpecificCommits(w, stderr, targetPath, sourceTree.Path, shas, dryRun, jsonOutput)
}

// applySpecificCommits cherry-picks specific commits.
func applySpecificCommits(w, stderr *cli.Writer, targetPath, sourcePath string, shas []string, dryRun, jsonOutput bool) ([]CommitInfo, error) {
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
			msgOutput, err := cmdexec.Output(context.TODO(), "git", []string{"-C", sourcePath, "log", "-1", "--format=%s", s}, "", cmdexec.GitLocal)
			if err != nil {
				if !jsonOutput {
					cli.Warning(stderr, "commit %s not found in source", s)
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
		cli.Bold(w, "Commits to apply (%d):", len(commits))
		for _, c := range commits {
			shortSHA := c.SHA
			if len(shortSHA) > 7 {
				shortSHA = shortSHA[:7]
			}
			cli.Faint(w, "  %s %s", shortSHA, c.Message)
		}
	}

	if dryRun {
		return commits, nil
	}

	// Cherry-pick each commit
	if !jsonOutput {
		cli.Step(w, "Applying commits...")
	}

	for _, c := range commits {
		if output, err := cmdexec.CombinedOutput(context.TODO(), "git", []string{"-C", targetPath, "cherry-pick", c.SHA}, "", cmdexec.GitLocal); err != nil {
			if !jsonOutput {
				cli.Error(stderr, "Conflict applying %s", c.SHA[:7])
				_, _ = fmt.Fprintf(stderr, "%s\n", output)
				cli.Faint(stderr, "To resolve:")
				cli.Faint(stderr, "  1. Fix conflicts in the affected files")
				cli.Faint(stderr, "  2. git add <resolved-files>")
				cli.Faint(stderr, "  3. git cherry-pick --continue")
				cli.Faint(stderr, "Or to abort:")
				cli.Faint(stderr, "  git cherry-pick --abort")
			}
			return commits, fmt.Errorf("cherry-pick failed")
		}

		if !jsonOutput {
			cli.Success(w, "%s", c.SHA[:7])
		}
	}

	return commits, nil
}

// applyWIPChanges creates a patch from source's uncommitted changes and applies to target.
func applyWIPChanges(w, stderr *cli.Writer, targetPath, sourcePath string, dryRun, jsonOutput bool) ([]string, error) {
	sourceHandler := worktree.NewWIPHandler(sourcePath)

	// Check if source has WIP
	hasWIP, err := sourceHandler.HasWIP()
	if err != nil {
		return nil, fmt.Errorf("failed to check source for WIP: %w", err)
	}

	if !hasWIP {
		if !jsonOutput && !dryRun {
			cli.Info(w, "No uncommitted changes to apply")
		}
		return nil, nil
	}

	// Get list of WIP files
	wipFiles, err := sourceHandler.ListWIPFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to list WIP files: %w", err)
	}

	if !jsonOutput {
		cli.Bold(w, "Uncommitted changes to apply (%d files):", len(wipFiles))
		for i, f := range wipFiles {
			if i >= 10 {
				cli.Faint(w, "  ... and %d more", len(wipFiles)-10)
				break
			}
			cli.Faint(w, "  %s", f)
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
			cli.Error(stderr, "Failed to apply uncommitted changes")
			_, _ = fmt.Fprintf(stderr, "%v\n", err)
			cli.Faint(stderr, "The patch may have conflicts with your current changes.")
			cli.Faint(stderr, "Resolve manually or try 'grove compare' to see differences first.")
		}
		return wipFiles, fmt.Errorf("failed to apply WIP patch")
	}

	if !jsonOutput {
		cli.Success(w, "Uncommitted changes applied")
	}

	return wipFiles, nil
}

// printApplyResult prints the apply result as JSON.
func printApplyResult(result ApplyResult) {
	if err := output.PrintJSON(result); err != nil {
		output.PrintJSONError(1, err.Error())
	}
}

// printApplyJSONError prints an error in JSON format.
func printApplyJSONError(code int, message string) {
	output.PrintJSONError(code, message)
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

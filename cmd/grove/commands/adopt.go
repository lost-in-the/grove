package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/cmdexec"
	"github.com/lost-in-the/grove/internal/worktree"
)

func init() {
	rootCmd.AddCommand(adoptCmd)
}

var adoptCmd = &cobra.Command{
	Use:   "adopt [path]",
	Short: "Bootstrap a git worktree that grove doesn't know about",
	Long: `Adopts an existing git worktree into grove's state.

Use when a worktree was created with 'git worktree add' instead of 'grove new':
the worktree exists, but grove never ran its bootstrap (state registration,
config symlink, post-create hooks, docker auto-start).

If [path] is omitted, the current directory is adopted.

Examples:
  grove adopt              # adopt the worktree the user is currently in
  grove adopt ../other-wt  # adopt by path`,
	Args: cobra.MaximumNArgs(1),
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		w := cli.NewStdout()

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get cwd: %w", err)
		}

		target, err := resolveAdoptTarget(cwd, args)
		if err != nil {
			return err
		}

		// Verify target is a git worktree of the main repo
		branch, gitErr := gitBranchAt(target)
		if gitErr != nil {
			return fmt.Errorf("not a git worktree at %s: %w", target, gitErr)
		}

		// Resolve symlinks on ProjectRoot since target is already EvalSymlinks-resolved
		// (resolveAdoptTarget). On macOS, /var/folders → /private/var/folders would
		// otherwise leak the main worktree into the adopt path.
		resolvedRoot, err := filepath.EvalSymlinks(ctx.ProjectRoot)
		if err != nil {
			resolvedRoot = ctx.ProjectRoot
		}
		if target == resolvedRoot {
			cli.Info(w, "the main worktree is always registered; nothing to adopt")
			return nil
		}

		mgr, err := worktree.NewManager(ctx.ProjectRoot)
		if err != nil {
			return fmt.Errorf("worktree manager: %w", err)
		}

		// Strip the naming pattern so the state key matches grove's convention
		// (state stores short names, not full directory names).
		name := mgr.ShortName(filepath.Base(target))

		if existing, err := ctx.State.GetWorktree(name); err == nil && existing != nil {
			existingResolved, _ := filepath.EvalSymlinks(existing.Path)
			if existingResolved == "" {
				existingResolved = existing.Path
			}
			if existingResolved == target {
				cli.Info(w, "worktree %q is already registered (path: %s)", name, target)
				return nil
			}
		}

		cli.Step(w, "Bootstrapping worktree %q at %s ...", name, target)

		bootstrapOpts := worktree.BootstrapOpts{
			Name:         name,
			Branch:       branch,
			WorktreePath: target,
			MainPath:     ctx.ProjectRoot,
			ProjectName:  mgr.GetProjectName(),
		}
		if err := worktree.BootstrapWorktree(ctx.State, ctx.Config, bootstrapOpts, w); err != nil {
			return fmt.Errorf("bootstrap: %w", err)
		}

		cli.Success(w, "adopted %q (branch: %s)", name, branch)
		cli.Faint(w, "config symlinked, state registered, post-create hooks fired")
		return nil
	}),
}

// resolveAdoptTarget picks the directory to adopt: explicit arg if given,
// otherwise cwd. Returns an absolute, EvalSymlinks-resolved path.
func resolveAdoptTarget(cwd string, args []string) (string, error) {
	target := cwd
	if len(args) == 1 {
		target = args[0]
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("resolve abs path %s: %w", target, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", abs, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a directory: %s", abs)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved, nil
	}
	return abs, nil
}

// gitBranchAt returns the current branch name of the git worktree at dir.
// Returns an error if the worktree is in detached HEAD state.
func gitBranchAt(dir string) (string, error) {
	out, err := cmdexec.Output(context.TODO(), "git", []string{"rev-parse", "--abbrev-ref", "HEAD"}, dir, cmdexec.GitLocal)
	if err != nil {
		return "", err
	}
	branch := strings.TrimSpace(string(out))
	if branch == "HEAD" {
		return "", fmt.Errorf("worktree is in detached HEAD state; check out a branch first")
	}
	return branch, nil
}

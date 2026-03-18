package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/cmdexec"
	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/grove"
	"github.com/lost-in-the/grove/internal/hooks"
	"github.com/lost-in-the/grove/internal/state"
	"github.com/lost-in-the/grove/internal/worktree"
	"github.com/lost-in-the/grove/plugins/docker"
)

func isGitRepo(dir string) bool {
	gitDir := filepath.Join(dir, ".git")
	_, err := os.Stat(gitDir)
	return err == nil
}

func detectProjectName(dir string) string {
	output, err := cmdexec.Output(context.TODO(), "git", []string{"remote", "get-url", "origin"}, dir, cmdexec.GitLocal)
	if err != nil {
		return filepath.Base(dir)
	}

	url := strings.TrimSpace(string(output))
	url = strings.TrimSuffix(url, ".git")
	parts := strings.Split(url, "/")
	if len(parts) == 0 {
		return filepath.Base(dir)
	}

	name := parts[len(parts)-1]
	if idx := strings.LastIndex(name, ":"); idx != -1 {
		name = name[idx+1:]
	}
	if name != "" {
		return name
	}

	return filepath.Base(dir)
}

func detectMainBranch(dir string) string {
	for _, branch := range []string{"main", "master"} {
		if err := cmdexec.Run(context.TODO(), "git", []string{"rev-parse", "--verify", branch}, dir, cmdexec.GitLocal); err == nil {
			return branch
		}
	}

	output, err := cmdexec.Output(context.TODO(), "git", []string{"rev-parse", "--abbrev-ref", "HEAD"}, dir, cmdexec.GitLocal)
	if err == nil {
		return strings.TrimSpace(string(output))
	}

	return "main"
}

func updateGitignore(dir string) error {
	gitignorePath := filepath.Join(dir, ".gitignore")

	content, err := os.ReadFile(gitignorePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if strings.Contains(string(content), ".grove/state.json") {
		return nil
	}

	entry := "\n# Grove (worktree manager)\n.grove/state.json\n.grove/state.json.bak\n.grove/.envrc\n"

	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	_, err = f.WriteString(entry)
	return err
}

func createWorktree(repoDir, projectName, name string) error {
	parentDir := filepath.Dir(repoDir)
	worktreeDir := filepath.Join(parentDir, fmt.Sprintf("%s-%s", projectName, name))

	output, err := cmdexec.Output(context.TODO(), "git", []string{"rev-parse", "--abbrev-ref", "HEAD"}, repoDir, cmdexec.GitLocal)
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}
	baseBranch := strings.TrimSpace(string(output))

	branchName := name
	// Worktree add streams progress to stdout/stderr — use exec.Command directly
	cmd := exec.Command("git", "worktree", "add", "-b", branchName, worktreeDir, baseBranch)
	cmd.Dir = repoDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// worktreeSetupOpts configures the post-create setup sequence.
type worktreeSetupOpts struct {
	IsEnvironment bool
	Mirror        string
	NoDocker      bool
	JSONOutput    bool
}

// setupCreatedWorktree runs the shared post-create sequence: find the worktree,
// symlink config, register state, execute hooks, and auto-start Docker.
func setupCreatedWorktree(ctx *GroveContext, mgr *worktree.Manager, name, branchName string, opts worktreeSetupOpts, w *cli.Writer) (*worktree.Worktree, error) {
	// Find the newly created worktree to get its path
	wt, err := mgr.Find(name)
	if err != nil || wt == nil {
		return nil, fmt.Errorf("failed to find created worktree: %w", err)
	}

	// Symlink config.toml from main worktree
	if err := grove.EnsureConfigSymlink(ctx.ProjectRoot, wt.Path); err != nil {
		if !opts.JSONOutput {
			cli.Warning(w, "Failed to symlink config: %v", err)
		}
	}

	// Register worktree in state
	now := time.Now()
	wsState := &state.WorktreeState{
		Path:           wt.Path,
		Branch:         branchName,
		Root:           false,
		CreatedAt:      now,
		LastAccessedAt: now,
		Environment:    opts.IsEnvironment,
	}
	if opts.IsEnvironment {
		wsState.Mirror = opts.Mirror
		wsState.LastSyncedAt = &now
	}
	if err := ctx.State.AddWorktree(name, wsState); err != nil {
		if !opts.JSONOutput {
			cli.Warning(w, "worktree created but state tracking failed: %v", err)
			cli.Faint(w, "run 'grove repair' to fix")
		}
	}

	projectName := mgr.GetProjectName()

	// Execute post-create hooks (per-project and global)
	runPostCreateHooks(w, ctx, name, branchName, projectName, wt.Path, opts.JSONOutput)

	// Auto-start Docker when configured
	autoStartDocker(w, ctx.Config, wt.Path, opts.NoDocker, opts.JSONOutput)

	return wt, nil
}

// runPostCreateHooks executes per-project and global post-create hooks.
func runPostCreateHooks(w *cli.Writer, ctx *GroveContext, name, branchName, projectName, wtPath string, jsonOutput bool) {
	hookExecutor, hookErr := hooks.NewExecutor()
	if hookErr != nil {
		if !jsonOutput {
			cli.Warning(w, "Failed to load hooks config: %v", hookErr)
		}
		return
	}
	if hookExecutor.HasHooksForEvent(hooks.EventPostCreate) {
		hookCtx := &hooks.ExecutionContext{
			Event:        hooks.EventPostCreate,
			Worktree:     name,
			WorktreeFull: projectName + "-" + name,
			Branch:       branchName,
			Project:      projectName,
			MainPath:     ctx.ProjectRoot,
			NewPath:      wtPath,
		}
		if !jsonOutput {
			cli.Step(w, "Running post-create hooks...")
		}
		if err := hookExecutor.Execute(hooks.EventPostCreate, hookCtx); err != nil {
			if !jsonOutput {
				cli.Warning(w, "Hook execution had errors: %v", err)
			}
		}
	}

	globalHookCtx := &hooks.Context{
		Worktree:     name,
		Config:       ctx.Config,
		WorktreePath: wtPath,
		MainPath:     ctx.ProjectRoot,
	}
	if err := hooks.Fire(hooks.EventPostCreate, globalHookCtx); err != nil {
		if !jsonOutput {
			cli.Warning(w, "Post-create plugin hook failed: %v", err)
		}
	}
}

// autoStartDocker starts the Docker stack for a new worktree if configured.
func autoStartDocker(w *cli.Writer, cfg *config.Config, wtPath string, noDocker, jsonOutput bool) {
	if noDocker || !shouldAutoDocker(cfg) {
		return
	}
	if !jsonOutput {
		cli.Step(w, "Starting Docker stack...")
	}
	dockerPlugin := docker.New()
	if cfg.AgentMode {
		dockerPlugin.SetIsolated(true)
	}
	if err := dockerPlugin.Init(cfg); err != nil {
		if !jsonOutput {
			cli.Warning(w, "Docker init failed: %v", err)
		}
		return
	}
	if !dockerPlugin.Enabled() {
		return
	}
	if err := dockerPlugin.Up(wtPath, true); err != nil {
		if !jsonOutput {
			cli.Warning(w, "Docker auto-start failed: %v", err)
		}
	} else if !jsonOutput {
		cli.Success(w, "Docker stack started")
	}
}

// shouldAutoDocker returns true when Docker should be auto-started on grove new.
// Enabled by default when agent stacks are configured, or explicitly via auto_up.
func shouldAutoDocker(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}

	// Explicit auto_up setting takes precedence
	if cfg.Plugins.Docker.AutoUp != nil {
		return *cfg.Plugins.Docker.AutoUp
	}

	// Default: auto-start when agent stacks are configured and enabled
	if cfg.IsExternalDockerMode() {
		ext := cfg.Plugins.Docker.External
		if ext != nil && ext.Agent != nil && ext.Agent.Enabled != nil && *ext.Agent.Enabled {
			return true
		}
	}

	return false
}

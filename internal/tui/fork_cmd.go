package tui

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/cmdexec"
	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/state"
	"github.com/lost-in-the/grove/internal/tuilog"
	"github.com/lost-in-the/grove/internal/worktree"
)

// WIPStrategy controls how uncommitted changes are handled during fork.
type WIPStrategy int

const (
	WIPMove  WIPStrategy = iota // move changes to new worktree
	WIPCopy                     // copy changes (keep in both)
	WIPLeave                    // leave changes in current
)

// forkRequest is a copy of a worktree onto a new branch cut from its HEAD.
type forkRequest struct {
	Source WorktreeItem
	Name   string // the new worktree's alias
	Branch string // the new branch
	HasWIP bool
	WIP    WIPStrategy
}

// forkCompleteMsg is sent after fork completes.
type forkCompleteMsg struct {
	name string
	path string
	err  error
}

// forkWorktreeCmd creates a forked worktree with optional WIP handling.
func forkWorktreeCmd(mgr *worktree.Manager, stateMgr *state.Manager, cfg *config.Config, projectRoot string, req forkRequest) tea.Cmd {
	return func() tea.Msg {
		source := req.Source
		name := req.Name
		newBranchName := req.Branch

		// Check if branch already exists
		if err := cmdexec.Run(context.TODO(), "git", []string{"-C", source.Path, "show-ref", "--verify", "--quiet", "refs/heads/" + newBranchName}, "", cmdexec.GitLocal); err == nil {
			return forkCompleteMsg{err: fmt.Errorf("branch %q already exists", newBranchName)}
		}

		// Capture WIP patch before any destructive operations
		wipPatch, err := forkCaptureWIP(req)
		if err != nil {
			return forkCompleteMsg{err: err}
		}

		// Create branch and worktree
		newTree, err := forkCreateBranchAndWorktree(mgr, source.Path, name, newBranchName)
		if err != nil {
			return forkCompleteMsg{err: err}
		}

		// Apply WIP and clean source if needed
		if err := forkApplyWIP(req, newTree.Path, wipPatch); err != nil {
			return forkCompleteMsg{name: name, path: newTree.Path, err: err}
		}

		// Provision through the shared bootstrap — the dashboard fork used to
		// register state + tmux only, skipping git excludes, SetupFiles, and the
		// plugin/config post_create hooks that `grove fork` and dashboard create
		// both run, leaving forked worktrees unprovisioned (no compose artifacts,
		// no post_create recipes). ParentWorktree records the fork source in
		// state. Bootstrap output is captured off the alt screen.
		var bootBuf bytes.Buffer
		pluginOut, bootErr := captureStdio(func() error {
			return worktree.BootstrapWorktree(stateMgr, cfg, worktree.BootstrapOpts{
				Name:           name,
				Branch:         newBranchName,
				WorktreePath:   newTree.Path,
				MainPath:       projectRoot,
				ProjectName:    mgr.GetProjectName(),
				ParentWorktree: source.ShortName,
			}, cli.NewWriter(&bootBuf, false))
		})
		if trimmed := strings.TrimSpace(joinCaptured(bootBuf.String(), pluginOut)); trimmed != "" {
			tuilog.Printf("fork bootstrap output for %q: %s", name, trimmed)
		}
		if bootErr != nil {
			// The worktree exists but bootstrap (e.g. a required post_create
			// hook) failed — surface it as a warning, not silent success.
			// Setting name marks it worktree-created-but-provisioning-failed,
			// which handleForkComplete shows as a warning toast, mirroring how
			// dashboard create threads hookErr. Skip the tmux session like the
			// create path does on bootstrap failure.
			tuilog.Printf("fork bootstrap failed for %q: %v", name, bootErr)
			return forkCompleteMsg{name: name, path: newTree.Path, err: fmt.Errorf("bootstrap failed: %w", bootErr)}
		}

		forkCreateTmuxSession(mgr, name, newTree.Path)

		return forkCompleteMsg{name: name, path: newTree.Path}
	}
}

// forkCaptureWIP captures a WIP patch from the source worktree if needed.
func forkCaptureWIP(req forkRequest) ([]byte, error) {
	if !req.HasWIP || (req.WIP != WIPMove && req.WIP != WIPCopy) {
		return nil, nil
	}
	wipHandler := worktree.NewWIPHandler(req.Source.Path)
	patch, err := wipHandler.CreatePatch()
	if err != nil {
		return nil, fmt.Errorf("failed to capture changes: %w", err)
	}
	return patch, nil
}

// forkCreateBranchAndWorktree creates the branch and worktree, cleaning up on failure.
func forkCreateBranchAndWorktree(mgr *worktree.Manager, sourcePath, name, branchName string) (*worktree.Worktree, error) {
	if output, err := cmdexec.CombinedOutput(context.TODO(), "git", []string{"-C", sourcePath, "branch", branchName, "HEAD"}, "", cmdexec.GitLocal); err != nil {
		return nil, fmt.Errorf("failed to create branch: %w\n%s", err, output)
	}

	if err := mgr.CreateFromBranch(name, branchName); err != nil {
		if cleanupErr := cmdexec.Run(context.TODO(), "git", []string{"-C", sourcePath, "branch", "-D", branchName}, "", cmdexec.GitLocal); cleanupErr != nil {
			return nil, fmt.Errorf("failed to create worktree: %w (orphaned branch %q may need manual cleanup)", err, branchName)
		}
		return nil, fmt.Errorf("failed to create worktree: %w", err)
	}

	newTree, err := mgr.Find(name)
	if err != nil || newTree == nil {
		return nil, errWorktreeNotFound
	}
	return newTree, nil
}

// forkApplyWIP applies the WIP patch to the new worktree and cleans the source if strategy is WIPMove.
func forkApplyWIP(req forkRequest, newPath string, wipPatch []byte) error {
	if len(wipPatch) == 0 {
		return nil
	}

	newWipHandler := worktree.NewWIPHandler(newPath)
	if err := newWipHandler.ApplyPatch(wipPatch); err != nil {
		return fmt.Errorf("worktree created but failed to apply changes: %w", err)
	}

	if req.WIP != WIPMove {
		return nil
	}

	sourcePath := req.Source.Path
	if output, err := cmdexec.CombinedOutput(context.TODO(), "git", []string{"-C", sourcePath, "checkout", "--", "."}, "", cmdexec.GitLocal); err != nil {
		return fmt.Errorf("forked but failed to clean source: %w\n%s", err, output)
	}
	if output, err := cmdexec.CombinedOutput(context.TODO(), "git", []string{"-C", sourcePath, "clean", "-fd"}, "", cmdexec.GitLocal); err != nil {
		return fmt.Errorf("forked but failed to clean untracked files: %w\n%s", err, output)
	}
	return nil
}

// forkCreateTmuxSession creates the tmux session for a freshly forked worktree.
// State registration now happens inside BootstrapWorktree.
func forkCreateTmuxSession(mgr *worktree.Manager, name, path string) {
	cfg := configForRepo(mgr.GetRepoRoot())
	m := muxFor(cfg)
	if !m.Available() || sessionsSuppressed(cfg) {
		return
	}
	target := muxTargetFor(mgr.GetProjectName(), mgr.GetRepoRoot(), name, path)
	if err := m.Ensure(target); err != nil {
		tuilog.Printf("warning: failed to create session %q: %v", target.Name, err)
	}
}

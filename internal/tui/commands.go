package tui

import (
	"bytes"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LeahArmstrong/grove-cli/internal/git"
	"github.com/LeahArmstrong/grove-cli/internal/hooks"
	"github.com/LeahArmstrong/grove-cli/internal/state"
	"github.com/LeahArmstrong/grove-cli/internal/tmux"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
)

func (m Model) fetchWorktrees() tea.Msg {
	items, err := FetchWorktrees(m.worktreeMgr, m.stateMgr)
	return worktreesFetchedMsg{items: items, err: err}
}

func deleteWorktreeCmd(mgr *worktree.Manager, stateMgr *state.Manager, projectRoot, name string, deleteBranch bool) tea.Cmd {
	return func() tea.Msg {
		projectName := mgr.GetProjectName()

		// Kill tmux session before removing worktree
		if tmux.IsTmuxAvailable() {
			sessionName := worktree.TmuxSessionName(projectName, name)
			if exists, _ := tmux.SessionExists(sessionName); exists {
				_ = tmux.KillSession(sessionName)
			}
		}

		// Capture the branch before removal so we can delete it afterwards
		var branch string
		wt, _ := mgr.Find(name)
		if wt != nil {
			branch = wt.Branch
		}

		// Run pre-remove hooks, capturing output to avoid corrupting TUI
		hookExecutor, hookErr := hooks.NewExecutor()
		if hookErr == nil && hookExecutor.HasHooksForEvent(hooks.EventPreRemove) {
			hookExecutor.Output = &bytes.Buffer{}
			hookCtx := &hooks.ExecutionContext{
				Event:    hooks.EventPreRemove,
				Worktree: name,
				Project:  projectName,
			}
			if wt != nil {
				hookCtx.Branch = wt.Branch
				hookCtx.NewPath = wt.Path
				hookCtx.WorktreeFull = projectName + "-" + name
			}
			_ = hookExecutor.Execute(hooks.EventPreRemove, hookCtx)
		}

		err := mgr.Remove(name)
		if err != nil {
			return worktreeDeletedMsg{name: name, deleteBranch: deleteBranch, err: err}
		}

		// Remove from state
		_ = stateMgr.RemoveWorktree(name)

		// Delete branch if requested
		if deleteBranch && branch != "" {
			branchMgr, branchErr := git.NewBranchManager(projectRoot)
			if branchErr == nil {
				_ = branchMgr.Delete(branch, false)
			}
		}

		return worktreeDeletedMsg{name: name, deleteBranch: deleteBranch, err: nil}
	}
}

func createWorktreeCmd(mgr *worktree.Manager, stateMgr *state.Manager, projectRoot, name, baseBranch string) tea.Cmd {
	return func() tea.Msg {
		branchArg := name
		if baseBranch != "" {
			branchArg = baseBranch
		}
		err := mgr.Create(name, branchArg)
		if err != nil {
			return worktreeCreatedMsg{name: name, err: err}
		}
		wt, err := mgr.Find(name)
		if err != nil || wt == nil {
			return worktreeCreatedMsg{name: name, err: fmt.Errorf("worktree created but not found")}
		}

		projectName := mgr.GetProjectName()

		// Register in state (matches grove new behavior)
		now := time.Now()
		wsState := &state.WorktreeState{
			Path:           wt.Path,
			Branch:         name,
			CreatedAt:      now,
			LastAccessedAt: now,
		}
		_ = stateMgr.AddWorktree(name, wsState)

		// Create tmux session
		if tmux.IsTmuxAvailable() {
			sessionName := worktree.TmuxSessionName(projectName, name)
			_ = tmux.CreateSession(sessionName, wt.Path)
		}

		// Run post-create hooks, capturing output to avoid corrupting TUI
		var hookBuf bytes.Buffer
		hookExecutor, hookErr := hooks.NewExecutor()
		if hookErr == nil && hookExecutor.HasHooksForEvent(hooks.EventPostCreate) {
			hookExecutor.Output = &hookBuf
			hookCtx := &hooks.ExecutionContext{
				Event:        hooks.EventPostCreate,
				Worktree:     name,
				WorktreeFull: projectName + "-" + name,
				Branch:       name,
				Project:      projectName,
				MainPath:     projectRoot,
				NewPath:      wt.Path,
			}
			_ = hookExecutor.Execute(hooks.EventPostCreate, hookCtx)
		}

		return worktreeCreatedMsg{name: name, path: wt.Path, hookOutput: hookBuf.String()}
	}
}

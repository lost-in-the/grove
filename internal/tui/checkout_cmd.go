package tui

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/lost-in-the/grove/internal/cmdexec"
	"github.com/lost-in-the/grove/internal/worktree"
)

// checkoutCompleteMsg is sent after the branch switch completes.
type checkoutCompleteMsg struct {
	branch string
	err    error
}

// checkoutBranchCmd performs the branch switch operation.
func checkoutBranchCmd(worktreePath, branch string, stash bool) tea.Cmd {
	return func() tea.Msg {
		// If stash requested, stash first
		if stash {
			wip := worktree.NewWIPHandler(worktreePath)
			if err := wip.Stash(fmt.Sprintf("grove: stash before switching to %s", branch)); err != nil {
				return checkoutCompleteMsg{err: fmt.Errorf("failed to stash changes: %w", err)}
			}
		}

		// Switch branch
		output, err := cmdexec.CombinedOutput(context.TODO(), "git", []string{"-C", worktreePath, "switch", branch}, "", cmdexec.GitLocal)
		if err != nil {
			return checkoutCompleteMsg{err: fmt.Errorf("failed to switch branch: %w\n%s", err, output)}
		}

		return checkoutCompleteMsg{branch: branch}
	}
}

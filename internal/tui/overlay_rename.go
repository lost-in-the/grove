package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/lost-in-the/grove/internal/state"
	"github.com/lost-in-the/grove/internal/tmux"
	"github.com/lost-in-the/grove/internal/tuilog"
	"github.com/lost-in-the/grove/internal/worktree"
)

// RenameState holds the state for the rename overlay.
type RenameState struct {
	Item     *WorktreeItem
	Input    textinput.Model
	Error    string
	Renaming bool
}

// NewRenameState creates a new RenameState for the given worktree item.
func NewRenameState(item *WorktreeItem) *RenameState {
	ti := textinput.New()
	ti.Prompt = "New name: "
	ti.Placeholder = item.ShortName
	ti.CharLimit = 100
	ti.Focus()
	return &RenameState{
		Item:  item,
		Input: ti,
	}
}

// renameCompleteMsg is sent after a rename attempt.
type renameCompleteMsg struct {
	oldName string
	newName string
	err     error
}

// renameWorktreeCmd performs the rename operation asynchronously.
func renameWorktreeCmd(mgr *worktree.Manager, stateMgr *state.Manager, oldName, newName string) tea.Cmd {
	return func() tea.Msg {
		projectName := mgr.GetProjectName()

		// Step 1: Move the git worktree
		if err := mgr.Move(oldName, newName); err != nil {
			return renameCompleteMsg{oldName: oldName, newName: newName, err: fmt.Errorf("failed to move worktree: %w", err)}
		}

		// Step 2: Rename in state
		if err := stateMgr.RenameWorktree(oldName, newName); err != nil {
			tuilog.Printf("warning: worktree moved but state rename failed for %q: %v", oldName, err)
		}

		// Step 3: Update the path in state
		newWt, findErr := mgr.Find(newName)
		if findErr == nil && newWt != nil {
			if ws, _ := stateMgr.GetWorktree(newName); ws != nil {
				ws.Path = newWt.Path
				_ = stateMgr.AddWorktree(newName, ws)
			}
		}

		// Step 4: Rename tmux session if it exists
		if tmux.IsTmuxAvailable() {
			oldSessionName := worktree.TmuxSessionName(projectName, oldName)
			newSessionName := worktree.TmuxSessionName(projectName, newName)

			if exists, err := tmux.SessionExists(oldSessionName); err == nil && exists {
				if err := tmux.RenameSession(oldSessionName, newSessionName); err != nil {
					tuilog.Printf("warning: failed to rename tmux session %q to %q: %v", oldSessionName, newSessionName, err)
				}
			}
		}

		return renameCompleteMsg{oldName: oldName, newName: newName, err: nil}
	}
}

// renderRename renders the rename overlay.
func renderRename(s *RenameState, width int) string {
	if s == nil || s.Item == nil {
		return ""
	}

	overlayWidth := calcOverlayWidth(width)
	indent := overlayIndent

	if s.Renaming {
		var b strings.Builder
		b.WriteString(indent + "Renaming worktree " + Styles.DetailValue.Render(s.Item.ShortName) + "...\n")
		b.WriteString("\n" + Styles.Footer.Render(indent+"Please wait..."))
		return Styles.OverlayBorder.Width(overlayWidth).Render(
			Styles.OverlayTitle.Render("Rename Worktree") + "\n\n" + b.String(),
		)
	}

	var b strings.Builder

	b.WriteString(indent + "Current name: " + Styles.DetailValue.Render(s.Item.ShortName) + "\n\n")
	b.WriteString(indent + s.Input.View() + "\n")

	newName := s.Input.Value()
	if newName != "" && s.Item.ShortName != newName {
		// Show what the full name will look like
		b.WriteString(indent + Styles.DetailDim.Render("→ directory will be renamed") + "\n")
	}

	if s.Error != "" {
		b.WriteString("\n" + indent + Styles.ErrorText.Render(s.Error) + "\n")
	}

	b.WriteString("\n" + Styles.Footer.Render(indent+"[enter] rename  [esc] cancel"))

	return Styles.OverlayBorder.Width(overlayWidth).Render(
		Styles.OverlayTitle.Render("Rename Worktree") + "\n\n" + b.String(),
	)
}

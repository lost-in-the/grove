package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/lost-in-the/grove/internal/cmdexec"
	"github.com/lost-in-the/grove/internal/state"
	"github.com/lost-in-the/grove/internal/tmux"
	"github.com/lost-in-the/grove/internal/tuilog"
	"github.com/lost-in-the/grove/internal/worktree"
)

// ForkStep represents the current step in the fork wizard.
type ForkStep int

const (
	ForkStepName ForkStep = iota
	ForkStepWIP           // WIP strategy selection (skipped if no WIP)
	ForkStepConfirm
)

// WIPStrategy controls how uncommitted changes are handled during fork.
type WIPStrategy int

const (
	WIPMove  WIPStrategy = iota // move changes to new worktree
	WIPCopy                     // copy changes (keep in both)
	WIPLeave                    // leave changes in current
)

// ForkState holds the state for the fork overlay.
type ForkState struct {
	Step        ForkStep
	Source      WorktreeItem
	Name        string
	NameInput   textinput.Model
	WIPStrategy WIPStrategy
	HasWIP      bool
	WIPFiles    []string
	WIPChoice   string // persists WIP form selection across back navigation
	WIPCursor   int    // cursor for WIP strategy selection
	Err         error
	Forking     bool
	Stepper     *Stepper
}

// newForkNameInput creates a configured textinput for fork naming.
func newForkNameInput() textinput.Model {
	ti := textinput.New()
	ti.Prompt = "Fork Name: "
	ti.Placeholder = "enter fork name"
	ti.CharLimit = 100
	ti.Focus()
	return ti
}

// NewForkState creates a new ForkState for the given source worktree.
func NewForkState(source WorktreeItem) *ForkState {
	return &ForkState{
		Step:      ForkStepName,
		Source:    source,
		NameInput: newForkNameInput(),
		Stepper:   NewStepper("Name", "WIP", "Confirm"),
	}
}

// forkWIPCheckMsg is sent after checking for WIP in the source worktree.
type forkWIPCheckMsg struct {
	hasWIP bool
	files  []string
	err    error
}

// forkCompleteMsg is sent after fork completes.
type forkCompleteMsg struct {
	name string
	path string
	err  error
}

// checkWIPCmd checks the source worktree for uncommitted changes.
func checkWIPCmd(source WorktreeItem) tea.Cmd {
	return func() tea.Msg {
		wip := worktree.NewWIPHandler(source.Path)
		hasWIP, err := wip.HasWIP()
		if err != nil {
			return forkWIPCheckMsg{err: err}
		}
		var files []string
		if hasWIP {
			files, err = wip.ListWIPFiles()
			if err != nil {
				return forkWIPCheckMsg{hasWIP: hasWIP, err: fmt.Errorf("failed to list WIP files: %w", err)}
			}
		}
		return forkWIPCheckMsg{hasWIP: hasWIP, files: files}
	}
}

// forkWorktreeCmd creates a forked worktree with optional WIP handling.
func forkWorktreeCmd(mgr *worktree.Manager, stateMgr *state.Manager, forkState *ForkState) tea.Cmd {
	return func() tea.Msg {
		source := forkState.Source
		name := forkState.Name
		strategy := forkState.WIPStrategy

		// Determine new branch name
		newBranchName := fmt.Sprintf("%s-%s", source.Branch, name)

		// Check if branch already exists
		if err := cmdexec.Run(context.TODO(), "git", []string{"-C", source.Path, "show-ref", "--verify", "--quiet", "refs/heads/" + newBranchName}, "", cmdexec.GitLocal); err == nil {
			return forkCompleteMsg{err: fmt.Errorf("branch %q already exists", newBranchName)}
		}

		// Handle WIP — capture patch before any destructive operations
		var wipPatch []byte
		if forkState.HasWIP && (strategy == WIPMove || strategy == WIPCopy) {
			wipHandler := worktree.NewWIPHandler(source.Path)
			var err error
			wipPatch, err = wipHandler.CreatePatch()
			if err != nil {
				return forkCompleteMsg{err: fmt.Errorf("failed to capture changes: %w", err)}
			}
		}

		// Create branch from source HEAD
		if output, err := cmdexec.CombinedOutput(context.TODO(), "git", []string{"-C", source.Path, "branch", newBranchName, "HEAD"}, "", cmdexec.GitLocal); err != nil {
			return forkCompleteMsg{err: fmt.Errorf("failed to create branch: %w\n%s", err, output)}
		}

		// Create worktree
		if err := mgr.CreateFromBranch(name, newBranchName); err != nil {
			// Cleanup: delete the branch we just created
			if cleanupErr := cmdexec.Run(context.TODO(), "git", []string{"-C", source.Path, "branch", "-D", newBranchName}, "", cmdexec.GitLocal); cleanupErr != nil {
				return forkCompleteMsg{err: fmt.Errorf("failed to create worktree: %w (orphaned branch %q may need manual cleanup)", err, newBranchName)}
			}
			return forkCompleteMsg{err: fmt.Errorf("failed to create worktree: %w", err)}
		}

		// Find the created worktree
		newTree, err := mgr.Find(name)
		if err != nil || newTree == nil {
			return forkCompleteMsg{err: errWorktreeNotFound}
		}

		// Apply WIP patch to new worktree if needed
		if len(wipPatch) > 0 {
			newWipHandler := worktree.NewWIPHandler(newTree.Path)
			if err := newWipHandler.ApplyPatch(wipPatch); err != nil {
				return forkCompleteMsg{name: name, path: newTree.Path, err: fmt.Errorf("worktree created but failed to apply changes: %w", err)}
			}
		}

		// Only clean source AFTER new worktree + patch succeeded (WIPMove)
		if forkState.HasWIP && strategy == WIPMove && len(wipPatch) > 0 {
			if output, err := cmdexec.CombinedOutput(context.TODO(), "git", []string{"-C", source.Path, "checkout", "--", "."}, "", cmdexec.GitLocal); err != nil {
				return forkCompleteMsg{name: name, path: newTree.Path, err: fmt.Errorf("forked but failed to clean source: %w\n%s", err, output)}
			}
			if output, err := cmdexec.CombinedOutput(context.TODO(), "git", []string{"-C", source.Path, "clean", "-fd"}, "", cmdexec.GitLocal); err != nil {
				return forkCompleteMsg{name: name, path: newTree.Path, err: fmt.Errorf("forked but failed to clean untracked files: %w\n%s", err, output)}
			}
		}

		// Register in state
		now := time.Now()
		wsState := &state.WorktreeState{
			Path:           newTree.Path,
			Branch:         newBranchName,
			CreatedAt:      now,
			LastAccessedAt: now,
			ParentWorktree: source.ShortName,
		}
		if err := stateMgr.AddWorktree(name, wsState); err != nil {
			tuilog.Printf("warning: failed to register forked worktree %q in state: %v", name, err)
		}

		// Create tmux session
		projectName := mgr.GetProjectName()
		if tmux.IsTmuxAvailable() {
			sessionName := worktree.TmuxSessionName(projectName, name)
			if err := tmux.CreateSession(sessionName, newTree.Path); err != nil {
				tuilog.Printf("warning: failed to create tmux session %q: %v", sessionName, err)
			}
		}

		return forkCompleteMsg{name: name, path: newTree.Path}
	}
}

// handleForkKey handles key input for the fork overlay.
func (m Model) handleForkKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.forkState == nil {
		m.activeView = ViewDashboard
		return m, nil
	}

	s := m.forkState

	if s.Forking {
		return m, nil
	}

	switch s.Step {
	case ForkStepName:
		switch {
		case key.Matches(msg, m.keys.Escape):
			m.activeView = ViewDashboard
			m.forkState = nil
			return m, nil
		case key.Matches(msg, m.keys.Enter):
			s.Name = s.NameInput.Value()
			if s.Name == "" {
				s.Err = fmt.Errorf("name cannot be empty")
				return m, nil
			}
			if errMsg := ValidateWorktreeName(s.Name); errMsg != "" {
				s.Err = fmt.Errorf("%s", errMsg)
				return m, nil
			}
			// Check for duplicates
			for _, item := range m.existingWorktreeItems() {
				if item.ShortName == s.Name {
					s.Err = fmt.Errorf("worktree %q already exists", s.Name)
					return m, nil
				}
			}
			s.Err = nil
			if s.HasWIP {
				s.Step = ForkStepWIP
				s.Stepper.Current = 1
				return m, nil
			}
			s.Step = ForkStepConfirm
			s.Stepper.Current = 2
			return m, nil
		default:
			// Route remaining keys through the name textinput
			var cmd tea.Cmd
			s.NameInput, cmd = s.NameInput.Update(msg)
			s.Name = s.NameInput.Value()
			s.Err = nil
			return m, cmd
		}

	case ForkStepWIP:
		switch {
		case key.Matches(msg, m.keys.Escape):
			m.activeView = ViewDashboard
			m.forkState = nil
			return m, nil
		case key.Matches(msg, m.keys.Back):
			s.Step = ForkStepName
			s.Stepper.Current = 0
			return m, nil
		case key.Matches(msg, m.keys.Up):
			if s.WIPCursor > 0 {
				s.WIPCursor--
			}
			return m, nil
		case key.Matches(msg, m.keys.Down):
			if s.WIPCursor < 2 {
				s.WIPCursor++
			}
			return m, nil
		case key.Matches(msg, m.keys.Enter):
			switch s.WIPCursor {
			case 0:
				s.WIPStrategy = WIPMove
				s.WIPChoice = "move"
			case 1:
				s.WIPStrategy = WIPCopy
				s.WIPChoice = "copy"
			case 2:
				s.WIPStrategy = WIPLeave
				s.WIPChoice = "leave"
			}
			s.Step = ForkStepConfirm
			s.Stepper.Current = 2
			return m, nil
		}

	case ForkStepConfirm:
		switch {
		case key.Matches(msg, m.keys.Escape):
			m.activeView = ViewDashboard
			m.forkState = nil
			return m, nil

		case key.Matches(msg, m.keys.Back):
			if s.HasWIP {
				s.Step = ForkStepWIP
				s.Stepper.Current = 1
				return m, nil
			}
			s.Step = ForkStepName
			s.Stepper.Current = 0
			return m, nil

		case key.Matches(msg, m.keys.Enter):
			if m.worktreeMgr == nil || m.stateMgr == nil {
				return m, nil
			}
			s.Forking = true
			return m, tea.Batch(m.spinner.Tick, forkWorktreeCmd(m.worktreeMgr, m.stateMgr, s))
		}
	}

	return m, nil
}

// renderFork renders the fork overlay.
func renderFork(s *ForkState, width int) string {
	overlayWidth := calcOverlayWidth(width)
	contentWidth := overlayWidth - 6
	indent := overlayIndent
	innerWidth := contentWidth - len(indent)*2

	var b strings.Builder

	// Stepper
	b.WriteString(indentBlock(s.Stepper.View(innerWidth), indent) + "\n\n")

	if s.Forking {
		b.WriteString(indent + "⏳ Forking worktree " + Styles.DetailValue.Render(s.Name) + "...\n")
		if s.Err != nil {
			b.WriteString("\n" + indent + Styles.ErrorText.Render(s.Err.Error()) + "\n")
		}
		b.WriteString("\n" + Styles.Footer.Render(indent+"Please wait..."))
		return Styles.OverlayBorderSuccess.Width(overlayWidth).Render(
			Styles.OverlayTitle.Render("Fork Worktree") + "\n\n" + b.String(),
		)
	}

	if s.Err != nil {
		b.WriteString(indent + Styles.ErrorText.Render("Error: "+s.Err.Error()) + "\n\n")
	}

	switch s.Step {
	case ForkStepName:
		// Source info
		b.WriteString(indent + Styles.DetailLabel.Render("Source: ") + Styles.DetailValue.Render(s.Source.ShortName) + "\n")
		b.WriteString(indent + Styles.DetailLabel.Render("Branch: ") + Styles.DetailValue.Render(s.Source.Branch) + "\n\n")

		// Name input with textinput component
		b.WriteString(indent + s.NameInput.View() + "\n")
		b.WriteString("\n" + Styles.Footer.Render(indent+"[enter] next  [esc] cancel"))

	case ForkStepWIP:
		// Context summary
		b.WriteString(indent + Styles.DetailLabel.Render("Source: ") + Styles.DetailValue.Render(s.Source.ShortName) + "\n")
		b.WriteString(indent + Styles.DetailLabel.Render("Name:   ") + Styles.DetailValue.Render(s.Name) + "\n")
		b.WriteString(indent + Styles.DetailLabel.Render("WIP:    ") + Styles.WarningText.Render(fmt.Sprintf("%d files changed", len(s.WIPFiles))) + "\n\n")

		// Manual WIP strategy selector
		b.WriteString(indent + "Handle Uncommitted Changes\n\n")
		wipOptions := []string{
			"Move to fork (current becomes clean)",
			"Copy to fork (keep in both)",
			"Leave in current (fork starts clean)",
		}
		for i, opt := range wipOptions {
			cursor := "  "
			if i == s.WIPCursor {
				cursor = Styles.ListCursor.Render("❯ ")
			}
			b.WriteString(indent + cursor + opt + "\n")
		}
		b.WriteString("\n" + Styles.Footer.Render(indent+"[enter] next  [backspace] back  [esc] cancel"))

	case ForkStepConfirm:
		// Full summary
		newBranch := fmt.Sprintf("%s-%s", s.Source.Branch, s.Name)
		b.WriteString(indent + Styles.DetailLabel.Render("Source:  ") + Styles.DetailValue.Render(s.Source.ShortName) + "\n")
		b.WriteString(indent + Styles.DetailLabel.Render("Name:    ") + Styles.DetailValue.Render(s.Name) + "\n")
		b.WriteString(indent + Styles.DetailLabel.Render("Branch:  ") + Styles.DetailValue.Render(newBranch) + "\n")

		if s.HasWIP {
			wipDesc := "leave in current"
			switch s.WIPStrategy {
			case WIPMove:
				wipDesc = fmt.Sprintf("%d files → move to new worktree", len(s.WIPFiles))
			case WIPCopy:
				wipDesc = fmt.Sprintf("%d files → copy to both", len(s.WIPFiles))
			}
			b.WriteString(indent + Styles.DetailLabel.Render("WIP:     ") + Styles.DetailValue.Render(wipDesc) + "\n")
		}

		b.WriteString("\n" + Styles.SuccessText.Render(indent+"Ready to fork.") + "\n")
		b.WriteString("\n" + Styles.Footer.Render(indent+"[enter] fork  [backspace] back  [esc] cancel"))
	}

	return Styles.OverlayBorderSuccess.Width(overlayWidth).Render(
		Styles.OverlayTitle.Render("Fork Worktree") + "\n\n" + b.String(),
	)
}

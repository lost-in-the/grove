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

// forkCompleteMsg is sent after fork completes.
type forkCompleteMsg struct {
	name string
	path string
	err  error
}

// forkWorktreeCmd creates a forked worktree with optional WIP handling.
func forkWorktreeCmd(mgr *worktree.Manager, stateMgr *state.Manager, forkState *ForkState) tea.Cmd {
	return func() tea.Msg {
		source := forkState.Source
		name := forkState.Name
		newBranchName := fmt.Sprintf("%s-%s", source.Branch, name)

		// Check if branch already exists
		if err := cmdexec.Run(context.TODO(), "git", []string{"-C", source.Path, "show-ref", "--verify", "--quiet", "refs/heads/" + newBranchName}, "", cmdexec.GitLocal); err == nil {
			return forkCompleteMsg{err: fmt.Errorf("branch %q already exists", newBranchName)}
		}

		// Capture WIP patch before any destructive operations
		wipPatch, err := forkCaptureWIP(forkState)
		if err != nil {
			return forkCompleteMsg{err: err}
		}

		// Create branch and worktree
		newTree, err := forkCreateBranchAndWorktree(mgr, source.Path, name, newBranchName)
		if err != nil {
			return forkCompleteMsg{err: err}
		}

		// Apply WIP and clean source if needed
		if err := forkApplyWIP(forkState, newTree.Path, wipPatch); err != nil {
			return forkCompleteMsg{name: name, path: newTree.Path, err: err}
		}

		// Register in state and create tmux session
		forkRegister(mgr, stateMgr, name, newBranchName, newTree.Path, source.ShortName)

		return forkCompleteMsg{name: name, path: newTree.Path}
	}
}

// forkCaptureWIP captures a WIP patch from the source worktree if needed.
func forkCaptureWIP(forkState *ForkState) ([]byte, error) {
	if !forkState.HasWIP || (forkState.WIPStrategy != WIPMove && forkState.WIPStrategy != WIPCopy) {
		return nil, nil
	}
	wipHandler := worktree.NewWIPHandler(forkState.Source.Path)
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
func forkApplyWIP(forkState *ForkState, newPath string, wipPatch []byte) error {
	if len(wipPatch) == 0 {
		return nil
	}

	newWipHandler := worktree.NewWIPHandler(newPath)
	if err := newWipHandler.ApplyPatch(wipPatch); err != nil {
		return fmt.Errorf("worktree created but failed to apply changes: %w", err)
	}

	if forkState.WIPStrategy != WIPMove {
		return nil
	}

	sourcePath := forkState.Source.Path
	if output, err := cmdexec.CombinedOutput(context.TODO(), "git", []string{"-C", sourcePath, "checkout", "--", "."}, "", cmdexec.GitLocal); err != nil {
		return fmt.Errorf("forked but failed to clean source: %w\n%s", err, output)
	}
	if output, err := cmdexec.CombinedOutput(context.TODO(), "git", []string{"-C", sourcePath, "clean", "-fd"}, "", cmdexec.GitLocal); err != nil {
		return fmt.Errorf("forked but failed to clean untracked files: %w\n%s", err, output)
	}
	return nil
}

// forkRegister registers the new worktree in state and creates a tmux session.
func forkRegister(mgr *worktree.Manager, stateMgr *state.Manager, name, branchName, path, parentName string) {
	now := time.Now()
	wsState := &state.WorktreeState{
		Path:           path,
		Branch:         branchName,
		CreatedAt:      now,
		LastAccessedAt: now,
		ParentWorktree: parentName,
	}
	if err := stateMgr.AddWorktree(name, wsState); err != nil {
		tuilog.Printf("warning: failed to register forked worktree %q in state: %v", name, err)
	}

	projectName := mgr.GetProjectName()
	if tmux.IsTmuxAvailable() {
		sessionName := worktree.TmuxSessionName(projectName, name)
		if err := tmux.CreateSession(sessionName, path); err != nil {
			tuilog.Printf("warning: failed to create tmux session %q: %v", sessionName, err)
		}
	}
}

// handleForkKey handles key input for the fork overlay.
func (m Model) handleForkKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.forkState == nil {
		m.activeView = ViewDashboard
		return m, nil
	}

	if m.forkState.Forking {
		return m, nil
	}

	// Escape dismisses from any step
	if key.Matches(msg, m.keys.Escape) {
		m.activeView = ViewDashboard
		m.forkState = nil
		return m, nil
	}

	switch m.forkState.Step {
	case ForkStepName:
		return m.handleForkNameKey(msg)
	case ForkStepWIP:
		return m.handleForkWIPKey(msg)
	case ForkStepConfirm:
		return m.handleForkConfirmKey(msg)
	}

	return m, nil
}

// handleForkNameKey handles key input for the fork name step.
func (m Model) handleForkNameKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := m.forkState

	if !key.Matches(msg, m.keys.Enter) {
		var cmd tea.Cmd
		s.NameInput, cmd = s.NameInput.Update(msg)
		s.Name = s.NameInput.Value()
		s.Err = nil
		return m, cmd
	}

	s.Name = s.NameInput.Value()
	if s.Name == "" {
		s.Err = fmt.Errorf("name cannot be empty")
		return m, nil
	}
	if errMsg := ValidateWorktreeName(s.Name); errMsg != "" {
		s.Err = fmt.Errorf("%s", errMsg)
		return m, nil
	}
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
}

// handleForkWIPKey handles key input for the WIP strategy step.
func (m Model) handleForkWIPKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := m.forkState

	switch {
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

	return m, nil
}

// handleForkConfirmKey handles key input for the fork confirm step.
func (m Model) handleForkConfirmKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := m.forkState

	switch {
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

	return m, nil
}

// renderFork renders the fork overlay.
func renderFork(s *ForkState, width int) string {
	d := calcOverlayDims(width)

	var b strings.Builder

	// Stepper
	b.WriteString(indentBlock(s.Stepper.View(d.inner), d.indent) + "\n\n")

	if s.Forking {
		b.WriteString(d.indent + "⏳ Forking worktree " + Styles.DetailValue.Render(s.Name) + "...\n")
		if s.Err != nil {
			b.WriteString("\n" + d.indent + Styles.ErrorText.Render(s.Err.Error()) + "\n")
		}
		b.WriteString("\n" + Styles.Footer.Render(d.indent+"Please wait..."))
		return Styles.OverlayBorderSuccess.Width(d.overlay).Render(
			Styles.OverlayTitle.Render("Fork Worktree") + "\n\n" + b.String(),
		)
	}

	if s.Err != nil {
		b.WriteString(d.indent + Styles.ErrorText.Render("Error: "+s.Err.Error()) + "\n\n")
	}

	var footer string

	switch s.Step {
	case ForkStepName:
		// Source info
		b.WriteString(d.indent + Styles.DetailLabel.Render("Source: ") + Styles.DetailValue.Render(s.Source.ShortName) + "\n")
		b.WriteString(d.indent + Styles.DetailLabel.Render("Branch: ") + Styles.DetailValue.Render(s.Source.Branch) + "\n\n")

		// Name input with textinput component
		b.WriteString(d.indent + s.NameInput.View() + "\n")
		footer = "\n" + Styles.Footer.Render(d.indent+"[enter] next  [esc] cancel")

	case ForkStepWIP:
		// Context summary
		b.WriteString(d.indent + Styles.DetailLabel.Render("Source: ") + Styles.DetailValue.Render(s.Source.ShortName) + "\n")
		b.WriteString(d.indent + Styles.DetailLabel.Render("Name:   ") + Styles.DetailValue.Render(s.Name) + "\n")
		b.WriteString(d.indent + Styles.DetailLabel.Render("WIP:    ") + Styles.WarningText.Render(fmt.Sprintf("%d files changed", len(s.WIPFiles))) + "\n\n")

		// Manual WIP strategy selector
		b.WriteString(d.indent + "Handle Uncommitted Changes\n\n")
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
			b.WriteString(d.indent + cursor + opt + "\n")
		}
		footer = "\n" + Styles.Footer.Render(d.indent+"[enter] next  [backspace] back  [esc] cancel")

	case ForkStepConfirm:
		// Full summary
		newBranch := fmt.Sprintf("%s-%s", s.Source.Branch, s.Name)
		b.WriteString(d.indent + Styles.DetailLabel.Render("Source:  ") + Styles.DetailValue.Render(s.Source.ShortName) + "\n")
		b.WriteString(d.indent + Styles.DetailLabel.Render("Name:    ") + Styles.DetailValue.Render(s.Name) + "\n")
		b.WriteString(d.indent + Styles.DetailLabel.Render("Branch:  ") + Styles.DetailValue.Render(newBranch) + "\n")

		if s.HasWIP {
			wipDesc := "leave in current"
			switch s.WIPStrategy {
			case WIPMove:
				wipDesc = fmt.Sprintf("%d files → move to new worktree", len(s.WIPFiles))
			case WIPCopy:
				wipDesc = fmt.Sprintf("%d files → copy to both", len(s.WIPFiles))
			}
			b.WriteString(d.indent + Styles.DetailLabel.Render("WIP:     ") + Styles.DetailValue.Render(wipDesc) + "\n")
		}

		b.WriteString("\n" + Styles.SuccessText.Render(d.indent+"Ready to fork.") + "\n")
		footer = "\n" + Styles.Footer.Render(d.indent+"[enter] fork  [backspace] back  [esc] cancel")
	}

	content := b.String()

	return Styles.OverlayBorderSuccess.Width(d.overlay).Render(
		Styles.OverlayTitle.Render("Fork Worktree") + "\n\n" + padToHeight(content, forkOverlayMinLines) + footer,
	)
}

// forkOverlayMinLines is the fixed content height for the fork wizard.
// Set to accommodate the tallest step (WIP strategy selection).
const forkOverlayMinLines = 15

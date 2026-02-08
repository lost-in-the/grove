package tui

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"github.com/LeahArmstrong/grove-cli/internal/state"
	"github.com/LeahArmstrong/grove-cli/internal/tmux"
	"github.com/LeahArmstrong/grove-cli/internal/worktree"
)

// ForkStep represents the current step in the fork wizard.
type ForkStep int

const (
	ForkStepName    ForkStep = iota
	ForkStepWIP              // WIP strategy selection (skipped if no WIP)
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
	WIPStrategy WIPStrategy
	HasWIP      bool
	WIPFiles    []string
	WIPChoice   string // persists WIP form selection across back navigation
	Err         error
	Forking     bool
	Stepper     *Stepper
	Form        *huh.Form // active Huh form (name input or WIP choice)
}

// NewForkState creates a new ForkState for the given source worktree.
func NewForkState(source WorktreeItem) *ForkState {
	return &ForkState{
		Step:    ForkStepName,
		Source:  source,
		Stepper: NewStepper("Name", "WIP", "Confirm"),
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
		checkCmd := exec.Command("git", "-C", source.Path, "show-ref", "--verify", "--quiet", "refs/heads/"+newBranchName)
		if err := checkCmd.Run(); err == nil {
			return forkCompleteMsg{err: fmt.Errorf("branch %q already exists", newBranchName)}
		}

		// Handle WIP
		var wipPatch []byte
		if forkState.HasWIP && (strategy == WIPMove || strategy == WIPCopy) {
			wipHandler := worktree.NewWIPHandler(source.Path)
			var err error
			wipPatch, err = wipHandler.CreatePatch()
			if err != nil {
				return forkCompleteMsg{err: fmt.Errorf("failed to capture changes: %w", err)}
			}

			if strategy == WIPMove {
				resetCmd := exec.Command("git", "-C", source.Path, "checkout", "--", ".")
				if output, err := resetCmd.CombinedOutput(); err != nil {
					return forkCompleteMsg{err: fmt.Errorf("failed to reset working tree: %w\n%s", err, output)}
				}
				cleanCmd := exec.Command("git", "-C", source.Path, "clean", "-fd")
				if output, err := cleanCmd.CombinedOutput(); err != nil {
					return forkCompleteMsg{err: fmt.Errorf("failed to clean untracked files: %w\n%s", err, output)}
				}
			}
		}

		// Create branch from source HEAD
		createBranchCmd := exec.Command("git", "-C", source.Path, "branch", newBranchName, "HEAD")
		if output, err := createBranchCmd.CombinedOutput(); err != nil {
			return forkCompleteMsg{err: fmt.Errorf("failed to create branch: %w\n%s", err, output)}
		}

		// Create worktree
		if err := mgr.CreateFromBranch(name, newBranchName); err != nil {
			// Cleanup: delete the branch
			exec.Command("git", "-C", source.Path, "branch", "-D", newBranchName).Run()
			return forkCompleteMsg{err: fmt.Errorf("failed to create worktree: %w", err)}
		}

		// Find the created worktree
		newTree, err := mgr.Find(name)
		if err != nil || newTree == nil {
			return forkCompleteMsg{err: fmt.Errorf("worktree created but not found")}
		}

		// Apply WIP patch to new worktree if needed
		if len(wipPatch) > 0 {
			newWipHandler := worktree.NewWIPHandler(newTree.Path)
			if err := newWipHandler.ApplyPatch(wipPatch); err != nil {
				return forkCompleteMsg{name: name, path: newTree.Path, err: fmt.Errorf("worktree created but failed to apply changes: %w", err)}
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
		_ = stateMgr.AddWorktree(name, wsState)

		// Create tmux session
		projectName := mgr.GetProjectName()
		if tmux.IsTmuxAvailable() {
			sessionName := worktree.TmuxSessionName(projectName, name)
			_ = tmux.CreateSession(sessionName, newTree.Path)
		}

		return forkCompleteMsg{name: name, path: newTree.Path}
	}
}

// NewForkNameForm creates a Huh form for the fork name input.
func NewForkNameForm(nameValue *string, projectName string, existingItems []WorktreeItem) *huh.Form {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Fork Name").
				Placeholder("feature-name").
				Validate(createNameValidator(existingItems)).
				Value(nameValue),
		),
	).WithTheme(huh.ThemeCharm()).WithShowHelp(false).WithAccessible(isHighContrast())

	return form
}

// NewForkWIPForm creates a Huh form for selecting WIP strategy.
func NewForkWIPForm(strategy *string) *huh.Form {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Handle Uncommitted Changes").
				Description("How should uncommitted changes be handled?").
				Options(
					huh.NewOption("Move to fork (current becomes clean)", "move"),
					huh.NewOption("Copy to fork (keep in both)", "copy"),
					huh.NewOption("Leave in current (fork starts clean)", "leave"),
				).
				Value(strategy),
		),
	).WithTheme(huh.ThemeCharm()).WithShowHelp(false).WithAccessible(isHighContrast())

	return form
}

// handleForkKey handles key input for the fork overlay.
func (m Model) handleForkKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		if key.Matches(msg, m.keys.Escape) {
			m.activeView = ViewDashboard
			m.forkState = nil
			return m, nil
		}
		if s.Form == nil {
			// Initialize form on first key
			s.Form = NewForkNameForm(&s.Name, m.projectName, m.existingWorktreeItems())
			return m, s.Form.Init()
		}
		model, cmd := s.Form.Update(msg)
		s.Form = model.(*huh.Form)

		if s.Form.State == huh.StateAborted {
			m.activeView = ViewDashboard
			m.forkState = nil
			return m, nil
		}
		if s.Form.State == huh.StateCompleted {
			if s.HasWIP {
				s.Step = ForkStepWIP
				s.Stepper.Current = 1
				s.Form = NewForkWIPForm(&s.WIPChoice)
				return m, s.Form.Init()
			}
			s.Step = ForkStepConfirm
			s.Stepper.Current = 2
			s.Form = nil
			return m, nil
		}
		return m, cmd

	case ForkStepWIP:
		if key.Matches(msg, m.keys.Back) {
			s.Step = ForkStepName
			s.Stepper.Current = 0
			s.Form = NewForkNameForm(&s.Name, m.projectName, m.existingWorktreeItems())
			return m, s.Form.Init()
		}
		if s.Form == nil {
			return m, nil
		}
		model, cmd := s.Form.Update(msg)
		s.Form = model.(*huh.Form)

		if s.Form.State == huh.StateAborted {
			m.activeView = ViewDashboard
			m.forkState = nil
			return m, nil
		}
		if s.Form.State == huh.StateCompleted {
			// Extract WIP strategy from form value
			switch {
			case strings.Contains(s.WIPChoice, "move") || s.WIPChoice == "move":
				s.WIPStrategy = WIPMove
			case strings.Contains(s.WIPChoice, "copy") || s.WIPChoice == "copy":
				s.WIPStrategy = WIPCopy
			default:
				s.WIPStrategy = WIPLeave
			}
			s.Step = ForkStepConfirm
			s.Stepper.Current = 2
			s.Form = nil
			return m, nil
		}
		return m, cmd

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
				s.Form = NewForkWIPForm(&s.WIPChoice)
				return m, s.Form.Init()
			}
			s.Step = ForkStepName
			s.Stepper.Current = 0
			s.Form = NewForkNameForm(&s.Name, m.projectName, m.existingWorktreeItems())
			return m, s.Form.Init()

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

// forwardToForkHuhForm forwards non-key messages to the active fork Huh form.
func (m Model) forwardToForkHuhForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	s := m.forkState
	if s.Form == nil {
		return m, nil
	}
	model, cmd := s.Form.Update(msg)
	s.Form = model.(*huh.Form)
	return m, cmd
}

// renderFork renders the fork overlay.
func renderFork(s *ForkState, width int) string {
	overlayWidth := width * 50 / 100
	if overlayWidth < 50 {
		overlayWidth = 50
	}
	if overlayWidth > 70 {
		overlayWidth = 70
	}
	contentWidth := overlayWidth - 6
	indent := huhOverlayIndent
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

		if s.Form != nil {
			b.WriteString(s.Form.View())
		}
		b.WriteString("\n" + Styles.Footer.Render(indent+"[enter] next  [esc] cancel"))

	case ForkStepWIP:
		// Context summary
		b.WriteString(indent + Styles.DetailLabel.Render("Source: ") + Styles.DetailValue.Render(s.Source.ShortName) + "\n")
		b.WriteString(indent + Styles.DetailLabel.Render("Name:   ") + Styles.DetailValue.Render(s.Name) + "\n")
		b.WriteString(indent + Styles.DetailLabel.Render("WIP:    ") + Styles.WarningText.Render(fmt.Sprintf("%d files changed", len(s.WIPFiles))) + "\n\n")

		if s.Form != nil {
			b.WriteString(s.Form.View())
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

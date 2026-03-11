package tui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/lost-in-the/grove/internal/cmdexec"
	"github.com/lost-in-the/grove/internal/git"
	"github.com/lost-in-the/grove/internal/worktree"
)

// CheckoutStep represents the current step in the checkout wizard.
type CheckoutStep int

const (
	CheckoutStepBranch  CheckoutStep = iota // Step 1: select target branch
	CheckoutStepWIP                         // Step 2: WIP handling (conditional)
	CheckoutStepConfirm                     // Step 3: confirm switch
)

// CheckoutState holds the state for the checkout branch overlay.
type CheckoutState struct {
	Step              CheckoutStep
	Item              WorktreeItem
	Branches          []string // available branches (filtered: excludes used-by-other-worktrees)
	SelectedBranch    string
	BranchCursor      int
	BranchFilterInput textinput.Model
	HasWIP            bool
	WIPCheckDone      bool // true after checkoutWIPCheckMsg is received
	WIPFiles          []string
	Stash             bool // true = stash before switching
	WIPCursor         int  // cursor for WIP options (0=stash, 1=cancel)
	Switching         bool
	Err               error
	Stepper           *Stepper
}

// NewCheckoutState creates a new CheckoutState for the given worktree item.
func NewCheckoutState(item WorktreeItem) *CheckoutState {
	ti := newBranchFilterInput()
	ti.Placeholder = "type to filter branches"
	return &CheckoutState{
		Step:              CheckoutStepBranch,
		Item:              item,
		BranchFilterInput: ti,
		Stepper:           NewStepper("Branch", "WIP", "Confirm"),
	}
}

// --- Messages ---

// checkoutBranchesMsg is sent after listing branches for checkout.
type checkoutBranchesMsg struct {
	branches     []string
	usedBranches map[string]bool // branches used by other worktrees
	err          error
}

// checkoutWIPCheckMsg is sent after checking for WIP in the worktree.
type checkoutWIPCheckMsg struct {
	hasWIP bool
	files  []string
	err    error
}

// checkoutCompleteMsg is sent after the branch switch completes.
type checkoutCompleteMsg struct {
	branch string
	err    error
}

// --- Commands ---

// listCheckoutBranchesCmd lists local branches and identifies which are used by worktrees.
func listCheckoutBranchesCmd(projectRoot string, currentWorktreePath string) tea.Cmd {
	return func() tea.Msg {
		branches, err := git.ListLocalBranches(projectRoot)
		if err != nil {
			return checkoutBranchesMsg{err: fmt.Errorf("failed to list branches: %w", err)}
		}

		// Get worktree list to find used branches
		output, err := cmdexec.Output(context.TODO(), "git", []string{"-C", projectRoot, "worktree", "list", "--porcelain"}, "", cmdexec.GitLocal)
		if err != nil {
			// If we can't list worktrees, return branches without filtering
			return checkoutBranchesMsg{branches: branches}
		}

		usedBranches := make(map[string]bool)
		var currentWT string
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if path, found := strings.CutPrefix(line, "worktree "); found {
				currentWT = path
			} else if ref, found := strings.CutPrefix(line, "branch refs/heads/"); found {
				// Mark this branch as used by a worktree (but not the current one)
				if currentWT != currentWorktreePath {
					usedBranches[ref] = true
				}
			} else if line == "" {
				currentWT = ""
			}
		}

		return checkoutBranchesMsg{branches: branches, usedBranches: usedBranches}
	}
}

// checkoutWIPCmd checks for uncommitted changes in the worktree.
func checkoutWIPCmd(item WorktreeItem) tea.Cmd {
	return func() tea.Msg {
		wip := worktree.NewWIPHandler(item.Path)
		hasWIP, err := wip.HasWIP()
		if err != nil {
			return checkoutWIPCheckMsg{err: err}
		}
		var files []string
		if hasWIP {
			files, err = wip.ListWIPFiles()
			if err != nil {
				return checkoutWIPCheckMsg{hasWIP: hasWIP, err: fmt.Errorf("failed to list WIP files: %w", err)}
			}
		}
		return checkoutWIPCheckMsg{hasWIP: hasWIP, files: files}
	}
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

// --- Key Handler ---

// handleCheckoutKey handles key input for the checkout overlay.
func (m Model) handleCheckoutKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.checkoutState == nil {
		m.activeView = ViewDashboard
		return m, nil
	}

	s := m.checkoutState

	if s.Switching {
		return m, nil
	}

	switch s.Step {
	case CheckoutStepBranch:
		return m.handleCheckoutBranchKey(msg)
	case CheckoutStepWIP:
		return m.handleCheckoutWIPKey(msg)
	case CheckoutStepConfirm:
		return m.handleCheckoutConfirmKey(msg)
	}

	return m, nil
}

func (m Model) handleCheckoutBranchKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := m.checkoutState

	switch {
	case key.Matches(msg, m.keys.Escape):
		m.activeView = ViewDashboard
		m.checkoutState = nil
		return m, nil

	case key.Matches(msg, m.keys.Up):
		if s.BranchCursor > 0 {
			s.BranchCursor--
		}
		return m, nil

	case key.Matches(msg, m.keys.Down):
		filter := s.BranchFilterInput.Value()
		filtered := filteredBranches(s.Branches, filter)
		maxIdx := len(filtered) - 1
		if maxIdx < 0 {
			maxIdx = 0
		}
		if s.BranchCursor < maxIdx {
			s.BranchCursor++
		}
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		// Block until WIP check completes to prevent bypassing stash prompt
		if !s.WIPCheckDone {
			return m, nil
		}
		filter := s.BranchFilterInput.Value()
		filtered := filteredBranches(s.Branches, filter)
		if len(filtered) == 0 {
			return m, nil
		}
		if s.BranchCursor >= len(filtered) {
			s.BranchCursor = len(filtered) - 1
		}
		s.SelectedBranch = filtered[s.BranchCursor]
		s.Err = nil

		if s.HasWIP {
			s.Step = CheckoutStepWIP
			s.Stepper.Current = 1
		} else {
			s.Step = CheckoutStepConfirm
			s.Stepper.Current = 2
		}
		return m, nil

	default:
		// Route remaining keys through the filter textinput
		prevVal := s.BranchFilterInput.Value()
		var cmd tea.Cmd
		s.BranchFilterInput, cmd = s.BranchFilterInput.Update(msg)
		if s.BranchFilterInput.Value() != prevVal {
			s.BranchCursor = 0
		}
		return m, cmd
	}
}

func (m Model) handleCheckoutWIPKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := m.checkoutState

	switch {
	case key.Matches(msg, m.keys.Escape):
		m.activeView = ViewDashboard
		m.checkoutState = nil
		return m, nil

	case key.Matches(msg, m.keys.Back):
		s.Step = CheckoutStepBranch
		s.Stepper.Current = 0
		return m, nil

	case key.Matches(msg, m.keys.Up):
		if s.WIPCursor > 0 {
			s.WIPCursor--
		}
		return m, nil

	case key.Matches(msg, m.keys.Down):
		if s.WIPCursor < 1 { // only 2 options: stash (0), cancel (1)
			s.WIPCursor++
		}
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		switch s.WIPCursor {
		case 0: // stash
			s.Stash = true
			s.Step = CheckoutStepConfirm
			s.Stepper.Current = 2
		case 1: // cancel
			m.activeView = ViewDashboard
			m.checkoutState = nil
		}
		return m, nil
	}

	return m, nil
}

func (m Model) handleCheckoutConfirmKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := m.checkoutState

	switch {
	case key.Matches(msg, m.keys.Escape):
		m.activeView = ViewDashboard
		m.checkoutState = nil
		return m, nil

	case key.Matches(msg, m.keys.Back):
		s.Stash = false // reset stash decision on back-navigation
		if s.HasWIP {
			s.Step = CheckoutStepWIP
			s.Stepper.Current = 1
		} else {
			s.Step = CheckoutStepBranch
			s.Stepper.Current = 0
		}
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		s.Switching = true
		return m, tea.Batch(m.spinner.Tick, checkoutBranchCmd(s.Item.Path, s.SelectedBranch, s.Stash))
	}

	return m, nil
}

// --- Render ---

// renderCheckout renders the checkout overlay.
func renderCheckout(s *CheckoutState, width int) string {
	overlayWidth := calcOverlayWidth(width)
	contentWidth := overlayWidth - 6
	indent := overlayIndent
	innerWidth := contentWidth - len(indent)*2

	var b strings.Builder

	// Stepper
	b.WriteString(indentBlock(s.Stepper.View(innerWidth), indent) + "\n\n")

	if s.Switching {
		b.WriteString(indent + "Switching to " + Styles.DetailValue.Render(s.SelectedBranch) + "...\n")
		if s.Stash {
			b.WriteString(indent + Styles.DetailDim.Render("(changes stashed)") + "\n")
		}
		if s.Err != nil {
			b.WriteString("\n" + indent + Styles.ErrorText.Render(s.Err.Error()) + "\n")
		}
		b.WriteString("\n" + Styles.Footer.Render(indent+"Please wait..."))
		return Styles.OverlayBorderSuccess.Width(overlayWidth).Render(
			Styles.OverlayTitle.Render("Switch Branch") + "\n\n" + b.String(),
		)
	}

	if s.Err != nil {
		b.WriteString(indent + Styles.ErrorText.Render("Error: "+s.Err.Error()) + "\n\n")
	}

	switch s.Step {
	case CheckoutStepBranch:
		// Worktree context
		b.WriteString(indent + Styles.DetailLabel.Render("Worktree: ") + Styles.DetailValue.Render(s.Item.ShortName) + "\n")
		b.WriteString(indent + Styles.DetailLabel.Render("Current:  ") + Styles.DetailValue.Render(s.Item.Branch) + "\n\n")

		if s.Branches == nil {
			b.WriteString(indent + "Loading branches...\n")
		} else if len(s.Branches) == 0 {
			b.WriteString(indent + Styles.DetailDim.Render("(no other branches available)") + "\n")
		} else {
			// Filter input
			filter := s.BranchFilterInput.Value()
			if filter != "" {
				b.WriteString(indent + s.BranchFilterInput.View() + "\n\n")
			} else {
				b.WriteString(indent + "Select a branch to switch to\n\n")
			}

			filtered := filteredBranches(s.Branches, filter)
			if len(filtered) == 0 {
				b.WriteString(indent + Styles.DetailDim.Render("(no matching branches)") + "\n")
			} else {
				start, end := scrollWindow(len(filtered), s.BranchCursor, 10)
				for i := start; i < end; i++ {
					cursor := "  "
					if i == s.BranchCursor {
						cursor = Styles.ListCursor.Render("❯ ")
					}
					b.WriteString(indent + cursor + filtered[i] + "\n")
				}
				if end < len(filtered) {
					b.WriteString(indent + Styles.DetailDim.Render(fmt.Sprintf("… and %d more", len(filtered)-end)) + "\n")
				}
			}
		}

		if !s.WIPCheckDone {
			b.WriteString("\n" + indent + Styles.DetailDim.Render("Checking for uncommitted changes...") + "\n")
		}
		b.WriteString("\n" + Styles.Footer.Render(indent+"[enter] select  [esc] cancel  type to filter"))

	case CheckoutStepWIP:
		// Context summary
		b.WriteString(indent + Styles.DetailLabel.Render("Worktree: ") + Styles.DetailValue.Render(s.Item.ShortName) + "\n")
		b.WriteString(indent + Styles.DetailLabel.Render("Switch:   ") + Styles.DetailValue.Render(s.Item.Branch+" → "+s.SelectedBranch) + "\n")
		b.WriteString(indent + Styles.DetailLabel.Render("WIP:      ") + Styles.WarningText.Render(fmt.Sprintf("%d files changed", len(s.WIPFiles))) + "\n\n")

		b.WriteString(indent + "Handle Uncommitted Changes\n\n")
		wipOptions := []string{
			"Stash changes before switching",
			"Cancel (keep current branch)",
		}
		for i, opt := range wipOptions {
			cursor := "  "
			if i == s.WIPCursor {
				cursor = Styles.ListCursor.Render("❯ ")
			}
			b.WriteString(indent + cursor + opt + "\n")
		}
		b.WriteString("\n" + Styles.Footer.Render(indent+"[enter] select  [backspace] back  [esc] cancel"))

	case CheckoutStepConfirm:
		b.WriteString(indent + Styles.DetailLabel.Render("Worktree: ") + Styles.DetailValue.Render(s.Item.ShortName) + "\n")
		b.WriteString(indent + Styles.DetailLabel.Render("Current:  ") + Styles.DetailValue.Render(s.Item.Branch) + "\n")
		b.WriteString(indent + Styles.DetailLabel.Render("Target:   ") + Styles.DetailValue.Render(s.SelectedBranch) + "\n")

		if s.HasWIP && s.Stash {
			b.WriteString(indent + Styles.DetailLabel.Render("WIP:      ") + Styles.DetailValue.Render(fmt.Sprintf("%d files → stash before switching", len(s.WIPFiles))) + "\n")
		}

		b.WriteString("\n" + Styles.SuccessText.Render(indent+"Ready to switch.") + "\n")
		b.WriteString("\n" + Styles.Footer.Render(indent+"[enter] switch  [backspace] back  [esc] cancel"))
	}

	return Styles.OverlayBorderSuccess.Width(overlayWidth).Render(
		Styles.OverlayTitle.Render("Switch Branch") + "\n\n" + b.String(),
	)
}

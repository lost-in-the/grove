package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
)

// CreateStep represents the current step in the create wizard.
type CreateStep int

const (
	CreateStepBranch       CreateStep = 0 // unified branch selector
	CreateStepBranchAction CreateStep = 1 // split/fork (conditional)
	CreateStepName         CreateStep = 2 // name with suggestion
	CreateStepConfirm      CreateStep = 3
)

// CreateState holds the state for the new worktree wizard.
type CreateState struct {
	Step           CreateStep
	Name           string
	NameSuggestion string // derived from branch, shown as placeholder
	ProjectName    string
	BaseBranch     string // set when using existing branch (split)
	NewBranchName  string // set when creating new branch via selector
	Error          string

	// Branch selector state (unified)
	Branches          []string
	BranchCursor      int
	BranchFilterInput textinput.Model

	// Name input
	NameInput textinput.Model

	// Branch action state (split vs fork)
	ActionChoice  int // 0 = split (use as-is), 1 = fork (new branch from it)
	DontShowAgain bool

	// Duplicate validation
	ExistingWorktree *WorktreeItem // populated if name conflicts with existing worktree

	// Creating state
	Creating    bool
	ActivityLog *ActivityLog // streaming creation progress
}

// newBranchFilterInput creates a configured textinput for branch filtering.
func newBranchFilterInput() textinput.Model {
	ti := textinput.New()
	ti.Prompt = "Filter: "
	ti.Placeholder = "type to filter or create new"
	ti.CharLimit = 100
	return ti
}

// newNameInput creates a configured textinput for worktree naming.
func newNameInput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Prompt = "Name: "
	ti.Placeholder = placeholder
	ti.CharLimit = 100
	return ti
}

func renderCreate(s *CreateState, width int, spinnerView string) string {
	if s.Creating {
		return renderCreateSpinner(s, spinnerView)
	}
	switch s.Step {
	case CreateStepBranch:
		return renderCreateBranch(s)
	case CreateStepBranchAction:
		return renderCreateBranchAction(s)
	case CreateStepName:
		return renderCreateName(s)
	case CreateStepConfirm:
		return renderCreateConfirm(s)
	}
	return ""
}

func renderCreateConfirm(s *CreateState) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Step 3 of 3: Confirm\n\n")

	fmt.Fprintf(&b, "  Name:     %s\n", s.Name)
	if s.ProjectName != "" {
		fmt.Fprintf(&b, "  Full:     %s-%s\n", s.ProjectName, s.Name)
	}
	if s.BaseBranch != "" {
		fmt.Fprintf(&b, "  Branch:   from %s\n", s.BaseBranch)
	} else if s.NewBranchName != "" {
		fmt.Fprintf(&b, "  Branch:   new %s\n", s.NewBranchName)
	} else {
		b.WriteString("  Branch:   new branch\n")
	}

	if s.Error != "" {
		b.WriteString("\n" + Styles.ErrorText.Render("  "+s.Error) + "\n")
		b.WriteString("\n" + Styles.Footer.Render("[enter] retry  [backspace] back  [esc] cancel"))
	} else {
		b.WriteString("\n" + Styles.SuccessText.Render("  Ready to create worktree.") + "\n")
		b.WriteString("\n" + Styles.Footer.Render("[enter] create  [backspace] back  [esc] cancel"))
	}

	return Styles.OverlayBorder.Render(
		Styles.OverlayTitle.Render("New Worktree") + "\n\n" + b.String(),
	)
}

func renderCreateSpinner(s *CreateState, spinnerView string) string {
	var b strings.Builder
	b.WriteString(spinnerView + " Creating worktree " + Styles.DetailValue.Render(s.Name) + "...\n")
	if s.Error != "" {
		b.WriteString("\n" + Styles.ErrorText.Render(s.Error) + "\n")
	}
	return Styles.OverlayBorder.Render(
		Styles.OverlayTitle.Render("New Worktree") + "\n\n" + b.String(),
	)
}

func renderCreateName(s *CreateState) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Step 2 of 3: Name\n\n")

	b.WriteString(s.NameInput.View() + "\n")

	effectiveName := s.NameInput.Value()
	if effectiveName == "" {
		effectiveName = s.NameSuggestion
	}
	if s.ProjectName != "" && effectiveName != "" {
		b.WriteString(Styles.DetailDim.Render(fmt.Sprintf("→ %s-%s", s.ProjectName, effectiveName)) + "\n")
	}

	if s.Error != "" {
		b.WriteString("\n" + Styles.ErrorText.Render(s.Error) + "\n")
	} else if effectiveName != "" {
		b.WriteString("\n" + Styles.SuccessText.Render("✓ valid name") + "\n")
	}

	b.WriteString("\n" + Styles.Footer.Render("[enter] next  [backspace] back  [esc] cancel"))

	return Styles.OverlayBorder.Render(
		Styles.OverlayTitle.Render("New Worktree") + "\n\n" + b.String(),
	)
}

func renderCreateBranch(s *CreateState) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Step 1 of 3: Branch\n\n")

	filter := s.BranchFilterInput.Value()
	if filter != "" {
		b.WriteString(s.BranchFilterInput.View() + "\n\n")
	}

	filtered := filteredBranches(s.Branches, filter)
	showCreateNew := filter != "" && !exactBranchMatch(s.Branches, filter)
	totalItems := len(filtered)
	if showCreateNew {
		totalItems++
	}

	if totalItems == 0 {
		b.WriteString(Styles.DetailDim.Render("  (no branches found)") + "\n")
	} else {
		start, end := scrollWindow(totalItems, s.BranchCursor, 10)
		for i := start; i < end; i++ {
			cursor := "  "
			if i == s.BranchCursor {
				cursor = Styles.ListCursor.Render("❯ ")
			}
			if i < len(filtered) {
				b.WriteString(cursor + filtered[i] + "\n")
			} else {
				b.WriteString(cursor + "Create new branch: \"" + filter + "\"\n")
			}
		}
		if end < totalItems {
			b.WriteString(Styles.DetailDim.Render(fmt.Sprintf("  … and %d more", totalItems-end)) + "\n")
		}
	}

	b.WriteString("\n" + Styles.Footer.Render("[enter] select  [esc] cancel  type to filter"))

	return Styles.OverlayBorder.Render(
		Styles.OverlayTitle.Render("New Worktree") + "\n\n" + b.String(),
	)
}

func renderCreateBranchAction(s *CreateState) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Branch %q already exists\n\n", s.BaseBranch)

	options := []string{
		"Use branch as-is (split)",
		"Create new branch from it (fork)",
	}
	for i, opt := range options {
		cursor := "  "
		if i == s.ActionChoice {
			cursor = Styles.ListCursor.Render("❯ ")
		}
		b.WriteString(cursor + opt + "\n")
	}

	b.WriteString("\n")
	checkbox := "[ ]"
	if s.DontShowAgain {
		checkbox = "[x]"
	}
	b.WriteString(checkbox + " Don't show this again\n")

	b.WriteString("\n" + Styles.Footer.Render("[enter] confirm  [backspace] back  [esc] cancel  [space] toggle"))

	return Styles.OverlayBorder.Render(
		Styles.OverlayTitle.Render("New Worktree") + "\n\n" + b.String(),
	)
}

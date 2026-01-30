package tui

import (
	"fmt"
	"strings"
)

// CreateStep represents the current step in the create wizard.
type CreateStep int

const (
	CreateStepName         CreateStep = 0
	CreateStepBranch       CreateStep = 1
	CreateStepPickBranch   CreateStep = 2
	CreateStepBranchAction CreateStep = 3
)

// BranchOption represents a branch creation choice.
type BranchOption int

const (
	BranchNewFromCurrent BranchOption = 0
	BranchFromExisting   BranchOption = 1
)

// CreateState holds the state for the new worktree wizard.
type CreateState struct {
	Step         CreateStep
	Name         string
	ProjectName  string
	BranchChoice BranchOption
	Error        string

	// Branch picker state (for "From existing branch")
	Branches     []string
	BranchCursor int
	BranchFilter string
	BaseBranch   string

	// Branch action state (split vs fork)
	ActionChoice  int // 0 = split (use as-is), 1 = fork (new branch from it)
	DontShowAgain bool

	// Creating state
	Creating bool
}

func renderCreate(s *CreateState, width int, spinnerView string) string {
	if s.Creating {
		return renderCreateSpinner(s, spinnerView)
	}
	switch s.Step {
	case CreateStepName:
		return renderCreateName(s)
	case CreateStepBranch:
		return renderCreateBranch(s)
	case CreateStepPickBranch:
		return renderCreatePickBranch(s)
	case CreateStepBranchAction:
		return renderCreateBranchAction(s)
	}
	return ""
}

func renderCreateSpinner(s *CreateState, spinnerView string) string {
	var b strings.Builder
	b.WriteString(spinnerView + " Creating worktree " + Theme.DetailValue.Render(s.Name) + "...\n")
	if s.Error != "" {
		b.WriteString("\n" + Theme.ErrorText.Render(s.Error) + "\n")
	}
	return Theme.OverlayBorder.Render(
		Theme.OverlayTitle.Render("New Worktree") + "\n\n" + b.String(),
	)
}

func renderCreateName(s *CreateState) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Step 1 of 2: Name\n\n")
	fmt.Fprintf(&b, "Name: %s█\n", s.Name)

	if s.ProjectName != "" && s.Name != "" {
		b.WriteString(Theme.DetailDim.Render(fmt.Sprintf("→ %s-%s", s.ProjectName, s.Name)) + "\n")
	}

	if s.Error != "" {
		b.WriteString("\n" + Theme.ErrorText.Render(s.Error) + "\n")
	} else if s.Name != "" {
		b.WriteString("\n" + Theme.SuccessText.Render("✓ valid name") + "\n")
	}

	b.WriteString("\n" + Theme.Footer.Render("[enter] next  [esc] cancel"))

	return Theme.OverlayBorder.Render(
		Theme.OverlayTitle.Render("New Worktree") + "\n\n" + b.String(),
	)
}

func renderCreateBranch(s *CreateState) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Step 2 of 2: Branch\n\n")

	options := []string{"Create new branch", "From existing branch..."}
	for i, opt := range options {
		cursor := "  "
		if i == int(s.BranchChoice) {
			cursor = Theme.ListCursor.String()
		}
		b.WriteString(cursor + opt + "\n")
	}

	b.WriteString("\n" + Theme.Footer.Render("[enter] create  [backspace] back  [esc] cancel"))

	return Theme.OverlayBorder.Render(
		Theme.OverlayTitle.Render("New Worktree") + "\n\n" + b.String(),
	)
}

func renderCreatePickBranch(s *CreateState) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Select branch\n\n")

	if s.BranchFilter != "" {
		fmt.Fprintf(&b, "Filter: %s█\n\n", s.BranchFilter)
	}

	filtered := filteredBranches(s.Branches, s.BranchFilter)
	if len(filtered) == 0 {
		b.WriteString(Theme.DetailDim.Render("  (no matching branches)") + "\n")
	} else {
		maxShow := 10
		start := 0
		if s.BranchCursor >= maxShow {
			start = s.BranchCursor - maxShow + 1
		}
		end := start + maxShow
		if end > len(filtered) {
			end = len(filtered)
		}
		for i := start; i < end; i++ {
			cursor := "  "
			if i == s.BranchCursor {
				cursor = Theme.ListCursor.String()
			}
			b.WriteString(cursor + filtered[i] + "\n")
		}
		if end < len(filtered) {
			b.WriteString(Theme.DetailDim.Render(fmt.Sprintf("  … and %d more", len(filtered)-end)) + "\n")
		}
	}

	b.WriteString("\n" + Theme.Footer.Render("[enter] select  [backspace] back/filter  [esc] cancel  type to filter"))

	return Theme.OverlayBorder.Render(
		Theme.OverlayTitle.Render("New Worktree") + "\n\n" + b.String(),
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
			cursor = Theme.ListCursor.String()
		}
		b.WriteString(cursor + opt + "\n")
	}

	b.WriteString("\n")
	checkbox := "[ ]"
	if s.DontShowAgain {
		checkbox = "[x]"
	}
	b.WriteString(checkbox + " Don't show this again\n")

	b.WriteString("\n" + Theme.Footer.Render("[enter] confirm  [backspace] back  [esc] cancel  [space] toggle"))

	return Theme.OverlayBorder.Render(
		Theme.OverlayTitle.Render("New Worktree") + "\n\n" + b.String(),
	)
}

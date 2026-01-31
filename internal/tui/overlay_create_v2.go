package tui

import (
	"fmt"
	"strings"
)

// renderCreateV2 is the V2 dispatcher for the create wizard overlay.
// It integrates the Stepper component and context summary for multi-step flows.
func renderCreateV2(s *CreateState, width int, spinnerView string) string {
	if s.Creating {
		return renderCreateSpinnerV2(s, spinnerView)
	}
	switch s.Step {
	case CreateStepName:
		return renderCreateNameV2(s, width)
	case CreateStepBranch:
		return renderCreateBranchV2(s, width)
	case CreateStepPickBranch:
		return renderCreatePickBranchV2(s, width)
	case CreateStepBranchAction:
		return renderCreateBranchActionV2(s, width)
	}
	return ""
}

func renderCreateSpinnerV2(s *CreateState, spinnerView string) string {
	var b strings.Builder
	b.WriteString(spinnerView + " Creating worktree " + Theme.DetailValue.Render(s.Name) + "...\n")
	if s.Error != "" {
		b.WriteString("\n" + Theme.ErrorText.Render(s.Error) + "\n")
	}
	return Theme.OverlayBorder.Render(
		Theme.OverlayTitle.Render("New Worktree") + "\n\n" + b.String(),
	)
}

func renderCreateNameV2(s *CreateState, width int) string {
	var b strings.Builder

	// Stepper
	stepper := NewStepper("Name", "Branch")
	stepper.Current = 0
	b.WriteString(stepper.View(width-8) + "\n\n")

	// Input
	fmt.Fprintf(&b, "Name: %s█\n", s.Name)

	if s.ProjectName != "" && s.Name != "" {
		b.WriteString(Theme.DetailDim.Render(fmt.Sprintf("→ %s-%s", s.ProjectName, s.Name)) + "\n")
	}

	if s.Error != "" {
		b.WriteString("\n" + Theme.ErrorText.Render(s.Error) + "\n")
	} else if s.ExistingWorktree != nil {
		ex := s.ExistingWorktree
		b.WriteString("\n" + Theme.ErrorText.Render("✗ Worktree \""+ex.ShortName+"\" already exists") + "\n")
		b.WriteString("\n  Existing worktree:\n")
		fmt.Fprintf(&b, "    Path:     %s\n", ex.Path)
		fmt.Fprintf(&b, "    Branch:   %s\n", ex.Branch)
		if ex.IsDirty {
			fmt.Fprintf(&b, "    Status:   ● dirty (%d files)\n", len(ex.DirtyFiles))
		} else {
			b.WriteString("    Status:   ● clean\n")
		}
	} else if s.Name != "" {
		b.WriteString("\n" + Theme.SuccessText.Render("✓ valid name") + "\n")
	}

	if s.ExistingWorktree != nil {
		b.WriteString("\n" + Theme.Footer.Render("[enter] Switch to existing  [tab] edit name  [esc] cancel"))
	} else {
		b.WriteString("\n" + Theme.Footer.Render("[enter] next  [esc] cancel"))
	}

	return Theme.OverlayBorder.Render(
		Theme.OverlayTitle.Render("New Worktree") + "\n\n" + b.String(),
	)
}

func renderCreateBranchV2(s *CreateState, width int) string {
	var b strings.Builder

	// Stepper - on step 2 (index 1)
	stepper := NewStepper("Name", "Branch")
	stepper.Current = 1
	b.WriteString(stepper.View(width-8) + "\n\n")

	// Context summary from step 1
	b.WriteString(renderContextSummary(s, width-8) + "\n\n")

	// Branch options
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

func renderCreatePickBranchV2(s *CreateState, width int) string {
	var b strings.Builder

	// Stepper
	stepper := NewStepper("Name", "Branch")
	stepper.Current = 1
	b.WriteString(stepper.View(width-8) + "\n\n")

	// Context summary
	b.WriteString(renderContextSummary(s, width-8) + "\n\n")

	// Filter
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
		end = min(end, len(filtered))
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

func renderCreateBranchActionV2(s *CreateState, width int) string {
	var b strings.Builder

	// Stepper
	stepper := NewStepper("Name", "Branch")
	stepper.Current = 1
	b.WriteString(stepper.View(width-8) + "\n\n")

	// Context summary
	b.WriteString(renderContextSummary(s, width-8) + "\n\n")

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

// renderContextSummary renders a bordered summary box showing choices from previous steps.
func renderContextSummary(s *CreateState, width int) string {
	var lines []string

	if s.Name != "" {
		lines = append(lines, fmt.Sprintf("  Name:     %s", s.Name))
		if s.ProjectName != "" {
			lines = append(lines, fmt.Sprintf("  Full:     %s-%s", s.ProjectName, s.Name))
		}
	}

	if s.BaseBranch != "" {
		lines = append(lines, fmt.Sprintf("  Branch:   %s", s.BaseBranch))
	}

	header := Theme.DetailDim.Render("┌─ Summary ") +
		Theme.DetailDim.Render(strings.Repeat("─", max(width-14, 0))) +
		Theme.DetailDim.Render("┐")
	footer := Theme.DetailDim.Render("└" + strings.Repeat("─", max(width-4, 0)) + "┘")

	body := strings.Join(lines, "\n")

	return header + "\n" + body + "\n" + footer
}

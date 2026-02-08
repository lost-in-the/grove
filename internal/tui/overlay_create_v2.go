package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderCreateV2 is the V2 dispatcher for the create wizard overlay.
// It integrates the Stepper component and context summary for multi-step flows.
// When UseHuhForms is true, it delegates rendering to Huh forms for applicable steps.
func renderCreateV2(s *CreateState, width int, spinnerView string) string {
	if s.Creating {
		return renderCreateSpinnerV2(s, spinnerView)
	}

	if s.UseHuhForms {
		return renderCreateHuh(s, width)
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
	case CreateStepConfirm:
		return renderCreateConfirmV2(s, width)
	}
	return ""
}

// huhOverlayIndent is the consistent left-padding for all content inside the create overlay.
const huhOverlayIndent = "  "

// indentBlock prepends indent to every line of a multi-line string.
func indentBlock(s, indent string) string {
	return indent + strings.ReplaceAll(s, "\n", "\n"+indent)
}

// renderCreateHuh renders the create wizard using Huh forms.
func renderCreateHuh(s *CreateState, width int) string {
	// Cap overlay width for visual consistency
	overlayWidth := width * 50 / 100
	if overlayWidth < 50 {
		overlayWidth = 50
	}
	if overlayWidth > 70 {
		overlayWidth = 70
	}
	contentWidth := overlayWidth - 6 // border + padding
	indent := huhOverlayIndent
	innerWidth := contentWidth - len(indent)*2

	var b strings.Builder

	// Stepper
	stepLabels := []string{"Name", "Branch", "Confirm"}
	stepper := NewStepper(stepLabels...)
	switch s.Step {
	case CreateStepName:
		stepper.Current = 0
	case CreateStepConfirm:
		stepper.Current = 2
	default:
		stepper.Current = 1
	}
	b.WriteString(indentBlock(stepper.View(innerWidth), indent) + "\n\n")

	// Context summary for steps beyond name
	if s.Step > CreateStepName && s.Name != "" {
		b.WriteString(indentBlock(renderContextSummary(s, innerWidth), indent) + "\n\n")
	}

	// Render the active Huh form
	switch s.Step {
	case CreateStepName:
		if s.NameForm != nil {
			b.WriteString(s.NameForm.View())
			if s.Name != "" && s.ProjectName != "" {
				b.WriteString("\n" + Styles.DetailDim.Render(indent+"Will create: "+s.ProjectName+"-"+s.Name))
			}
		}
		b.WriteString("\n" + Styles.Footer.Render(indent+"[enter] next  [esc] cancel"))
	case CreateStepBranch:
		if s.BranchForm != nil {
			b.WriteString(s.BranchForm.View())
		}
		b.WriteString("\n" + Styles.Footer.Render(indent+"[enter] next  [backspace] back  [esc] cancel"))
	case CreateStepPickBranch:
		if s.BranchPickForm != nil {
			b.WriteString(s.BranchPickForm.View())
		}
		b.WriteString("\n" + Styles.Footer.Render(indent+"[enter] select  [backspace] back  [esc] cancel"))
	case CreateStepBranchAction:
		// BranchAction still uses manual rendering
		return renderCreateBranchActionV2(s, width)
	case CreateStepConfirm:
		return renderCreateConfirmV2(s, width)
	}

	return Styles.OverlayBorderSuccess.Width(overlayWidth).Render(
		Styles.OverlayTitle.Render("New Worktree") + "\n\n" + b.String(),
	)
}

func renderCreateSpinnerV2(s *CreateState, spinnerView string) string {
	overlayWidth := 60

	var b strings.Builder

	// Completed stepper
	stepLabels := []string{"Name", "Branch", "Confirm"}
	stepper := NewStepper(stepLabels...)
	stepper.Current = len(stepLabels) // all complete
	b.WriteString(stepper.View(overlayWidth-10) + "\n\n")

	// Context summary
	if s.Name != "" {
		b.WriteString(renderContextSummary(s, overlayWidth-10) + "\n\n")
	}

	b.WriteString(spinnerView + " Creating worktree " + Styles.DetailValue.Render(s.Name) + "...\n")
	if s.Error != "" {
		b.WriteString("\n" + Styles.ErrorText.Render(s.Error) + "\n")
	}
	b.WriteString("\n" + Styles.Footer.Render("Please wait..."))

	return Styles.OverlayBorderSuccess.Width(overlayWidth).Render(
		Styles.OverlayTitle.Render("New Worktree") + "\n\n" + b.String(),
	)
}

func renderCreateNameV2(s *CreateState, width int) string {
	var b strings.Builder

	// Stepper
	stepper := NewStepper("Name", "Branch", "Confirm")
	stepper.Current = 0
	b.WriteString(stepper.View(width-8) + "\n\n")

	// Input
	fmt.Fprintf(&b, "Name: %s█\n", s.Name)

	if s.ProjectName != "" && s.Name != "" {
		b.WriteString(Styles.DetailDim.Render(fmt.Sprintf("→ %s-%s", s.ProjectName, s.Name)) + "\n")
	}

	if s.Error != "" {
		b.WriteString("\n" + Styles.ErrorText.Render(s.Error) + "\n")
	} else if s.ExistingWorktree != nil {
		ex := s.ExistingWorktree
		b.WriteString("\n" + Styles.ErrorText.Render("✗ Worktree \""+ex.ShortName+"\" already exists") + "\n")
		b.WriteString("\n  Existing worktree:\n")
		fmt.Fprintf(&b, "    Path:     %s\n", ex.Path)
		fmt.Fprintf(&b, "    Branch:   %s\n", ex.Branch)
		if ex.IsDirty {
			fmt.Fprintf(&b, "    Status:   ● dirty (%d files)\n", len(ex.DirtyFiles))
		} else {
			b.WriteString("    Status:   ● clean\n")
		}
	} else if s.Name != "" {
		b.WriteString("\n" + Styles.SuccessText.Render("✓ valid name") + "\n")
	}

	if s.ExistingWorktree != nil {
		b.WriteString("\n" + Styles.Footer.Render("[enter] Switch to existing  [tab] edit name  [esc] cancel"))
	} else {
		b.WriteString("\n" + Styles.Footer.Render("[enter] next  [esc] cancel"))
	}

	return Styles.OverlayBorderSuccess.Render(
		Styles.OverlayTitle.Render("New Worktree") + "\n\n" + b.String(),
	)
}

func renderCreateBranchV2(s *CreateState, width int) string {
	var b strings.Builder

	// Stepper - on step 2 (index 1)
	stepper := NewStepper("Name", "Branch", "Confirm")
	stepper.Current = 1
	b.WriteString(stepper.View(width-8) + "\n\n")

	// Context summary from step 1
	b.WriteString(renderContextSummary(s, width-8) + "\n\n")

	// Branch options
	options := []string{"Create new branch", "From existing branch..."}
	for i, opt := range options {
		cursor := "  "
		if i == int(s.BranchChoice) {
			cursor = Styles.ListCursor.String()
		}
		b.WriteString(cursor + opt + "\n")
	}

	b.WriteString("\n" + Styles.Footer.Render("[enter] create  [backspace] back  [esc] cancel"))

	return Styles.OverlayBorderSuccess.Render(
		Styles.OverlayTitle.Render("New Worktree") + "\n\n" + b.String(),
	)
}

func renderCreatePickBranchV2(s *CreateState, width int) string {
	var b strings.Builder

	// Stepper
	stepper := NewStepper("Name", "Branch", "Confirm")
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
		b.WriteString(Styles.DetailDim.Render("  (no matching branches)") + "\n")
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
				cursor = Styles.ListCursor.String()
			}
			b.WriteString(cursor + filtered[i] + "\n")
		}
		if end < len(filtered) {
			b.WriteString(Styles.DetailDim.Render(fmt.Sprintf("  … and %d more", len(filtered)-end)) + "\n")
		}
	}

	b.WriteString("\n" + Styles.Footer.Render("[enter] select  [backspace] back/filter  [esc] cancel  type to filter"))

	return Styles.OverlayBorderSuccess.Render(
		Styles.OverlayTitle.Render("New Worktree") + "\n\n" + b.String(),
	)
}

func renderCreateBranchActionV2(s *CreateState, width int) string {
	var b strings.Builder

	// Stepper
	stepper := NewStepper("Name", "Branch", "Confirm")
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
			cursor = Styles.ListCursor.String()
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

	return Styles.OverlayBorderSuccess.Render(
		Styles.OverlayTitle.Render("New Worktree") + "\n\n" + b.String(),
	)
}

// renderContextSummary renders a bordered summary box showing choices from previous steps.
func renderContextSummary(s *CreateState, width int) string {
	var lines []string

	if s.Name != "" {
		lines = append(lines, fmt.Sprintf("Name:     %s", s.Name))
		if s.ProjectName != "" {
			lines = append(lines, fmt.Sprintf("Full:     %s-%s", s.ProjectName, s.Name))
		}
	}

	if s.BaseBranch != "" {
		lines = append(lines, fmt.Sprintf("Branch:   %s", s.BaseBranch))
	}

	body := Styles.TextMuted.Render(strings.Join(lines, "\n"))

	boxWidth := width - 2
	if boxWidth < 20 {
		boxWidth = 20
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Colors.SurfaceDim).
		Padding(0, 1).
		Width(boxWidth).
		Render(Styles.DetailLabel.Render("Summary") + "\n" + body)
}

// renderCreateConfirmV2 renders the confirmation step with full summary.
func renderCreateConfirmV2(s *CreateState, width int) string {
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

	// Stepper at step 3 (index 2)
	stepper := NewStepper("Name", "Branch", "Confirm")
	stepper.Current = 2
	b.WriteString(indentBlock(stepper.View(innerWidth), indent) + "\n\n")

	// Full summary
	b.WriteString(indentBlock(renderContextSummary(s, innerWidth), indent) + "\n\n")

	// Branch strategy detail
	if s.BaseBranch != "" {
		b.WriteString(indent + "Strategy: " + Styles.DetailValue.Render("from existing branch") + "\n")
	} else {
		b.WriteString(indent + "Strategy: " + Styles.DetailValue.Render("create new branch") + "\n")
	}

	b.WriteString("\n" + Styles.SuccessText.Render(indent+"Ready to create worktree.") + "\n")
	b.WriteString("\n" + Styles.Footer.Render(indent+"[enter] create  [backspace] back  [esc] cancel"))

	return Styles.OverlayBorderSuccess.Width(overlayWidth).Render(
		Styles.OverlayTitle.Render("New Worktree") + "\n\n" + b.String(),
	)
}

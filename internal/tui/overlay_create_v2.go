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
	case CreateStepBranch:
		return renderCreateBranchSelectorV2(s, width)
	case CreateStepBranchAction:
		return renderCreateBranchActionV2(s, width)
	case CreateStepName:
		return renderCreateNameV2(s, width)
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

// calcOverlayWidth computes the standard overlay width from terminal width.
// The overlay is 50% of terminal width, clamped to [50, 70].
func calcOverlayWidth(width int) int {
	w := width * 50 / 100
	if w < 50 {
		w = 50
	}
	if w > 70 {
		w = 70
	}
	return w
}

// renderCreateHuh renders the create wizard using Huh forms for applicable steps.
func renderCreateHuh(s *CreateState, width int) string {
	// Steps that don't use Huh forms delegate to their manual V2 renderers
	switch s.Step {
	case CreateStepBranch:
		return renderCreateBranchSelectorV2(s, width)
	case CreateStepBranchAction:
		return renderCreateBranchActionV2(s, width)
	case CreateStepConfirm:
		return renderCreateConfirmV2(s, width)
	}

	// Name step uses Huh form
	overlayWidth := calcOverlayWidth(width)
	contentWidth := overlayWidth - 6 // border + padding
	indent := huhOverlayIndent
	innerWidth := contentWidth - len(indent)*2

	var b strings.Builder

	// Stepper
	stepLabels := []string{"Branch", "Name", "Confirm"}
	stepper := NewStepper(stepLabels...)
	stepper.Current = 1
	b.WriteString(indentBlock(stepper.View(innerWidth), indent) + "\n\n")

	// Context summary
	b.WriteString(indentBlock(renderContextSummary(s, innerWidth), indent) + "\n\n")

	// Render the Huh form for name input
	if s.NameForm != nil {
		b.WriteString(s.NameForm.View())
		effectiveName := s.Name
		if effectiveName == "" {
			effectiveName = s.NameSuggestion
		}
		if effectiveName != "" && s.ProjectName != "" {
			b.WriteString("\n" + Styles.DetailDim.Render(indent+"Will create: "+s.ProjectName+"-"+effectiveName))
		}
	}
	b.WriteString("\n" + Styles.Footer.Render(indent+"[enter] next  [backspace] back  [esc] cancel"))

	return Styles.OverlayBorderSuccess.Width(overlayWidth).Render(
		Styles.OverlayTitle.Render("New Worktree") + "\n\n" + b.String(),
	)
}

func renderCreateSpinnerV2(s *CreateState, spinnerView string) string {
	overlayWidth := 60

	var b strings.Builder

	// Completed stepper
	stepLabels := []string{"Branch", "Name", "Confirm"}
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

// renderCreateBranchSelectorV2 renders the unified branch selector (Step 1).
// Shows a filterable list of existing branches with a "Create new branch" option
// when the filter text doesn't exactly match an existing branch.
func renderCreateBranchSelectorV2(s *CreateState, width int) string {
	overlayWidth := calcOverlayWidth(width)
	contentWidth := overlayWidth - 6
	indent := huhOverlayIndent
	innerWidth := contentWidth - len(indent)*2

	var b strings.Builder

	// Stepper — on step 1 (index 0)
	stepper := NewStepper("Branch", "Name", "Confirm")
	stepper.Current = 0
	b.WriteString(indentBlock(stepper.View(innerWidth), indent) + "\n\n")

	// Filter input
	if s.BranchFilter != "" {
		b.WriteString(indent + fmt.Sprintf("Filter: %s█\n\n", s.BranchFilter))
	} else {
		b.WriteString(indent + "Select a branch or type to create new\n\n")
	}

	// Build visible items
	filtered := filteredBranches(s.Branches, s.BranchFilter)
	showCreateNew := s.BranchFilter != "" && !exactBranchMatch(s.Branches, s.BranchFilter)
	totalItems := len(filtered)
	if showCreateNew {
		totalItems++
	}

	if totalItems == 0 {
		b.WriteString(indent + Styles.DetailDim.Render("(no branches found)") + "\n")
	} else {
		maxShow := 10
		start := 0
		if s.BranchCursor >= maxShow {
			start = s.BranchCursor - maxShow + 1
		}
		end := start + maxShow
		if end > totalItems {
			end = totalItems
		}
		for i := start; i < end; i++ {
			cursor := "  "
			if i == s.BranchCursor {
				cursor = Styles.ListCursor.String()
			}
			if i < len(filtered) {
				b.WriteString(indent + cursor + filtered[i] + "\n")
			} else {
				b.WriteString(indent + cursor + Styles.DetailValue.Render("Create new branch: \""+s.BranchFilter+"\"") + "\n")
			}
		}
		if end < totalItems {
			b.WriteString(indent + Styles.DetailDim.Render(fmt.Sprintf("… and %d more", totalItems-end)) + "\n")
		}
	}

	b.WriteString("\n" + Styles.Footer.Render(indent+"[enter] select  [esc] cancel  type to filter"))

	return Styles.OverlayBorderSuccess.Width(overlayWidth).Render(
		Styles.OverlayTitle.Render("New Worktree") + "\n\n" + b.String(),
	)
}

func renderCreateNameV2(s *CreateState, width int) string {
	overlayWidth := calcOverlayWidth(width)
	contentWidth := overlayWidth - 6
	indent := huhOverlayIndent
	innerWidth := contentWidth - len(indent)*2

	var b strings.Builder

	// Stepper — on step 2 (index 1)
	stepper := NewStepper("Branch", "Name", "Confirm")
	stepper.Current = 1
	b.WriteString(indentBlock(stepper.View(innerWidth), indent) + "\n\n")

	// Context summary from previous steps
	b.WriteString(indentBlock(renderContextSummary(s, innerWidth), indent) + "\n\n")

	// Input with placeholder
	if s.Name == "" && s.NameSuggestion != "" {
		b.WriteString(indent + "Name: " + Styles.DetailDim.Render(s.NameSuggestion) + "\n")
	} else {
		b.WriteString(indent + fmt.Sprintf("Name: %s█\n", s.Name))
	}

	effectiveName := s.Name
	if effectiveName == "" {
		effectiveName = s.NameSuggestion
	}
	if s.ProjectName != "" && effectiveName != "" {
		b.WriteString(indent + Styles.DetailDim.Render(fmt.Sprintf("→ %s-%s", s.ProjectName, effectiveName)) + "\n")
	}

	if s.Error != "" {
		b.WriteString("\n" + indent + Styles.ErrorText.Render(s.Error) + "\n")
	} else if s.ExistingWorktree != nil {
		ex := s.ExistingWorktree
		b.WriteString("\n" + indent + Styles.ErrorText.Render("✗ Worktree \""+ex.ShortName+"\" already exists") + "\n")
		b.WriteString("\n" + indent + "Existing worktree:\n")
		b.WriteString(indent + fmt.Sprintf("  Path:     %s\n", ex.Path))
		b.WriteString(indent + fmt.Sprintf("  Branch:   %s\n", ex.Branch))
		if ex.IsDirty {
			b.WriteString(indent + fmt.Sprintf("  Status:   ● dirty (%d files)\n", len(ex.DirtyFiles)))
		} else {
			b.WriteString(indent + "  Status:   ● clean\n")
		}
	} else if effectiveName != "" {
		b.WriteString("\n" + indent + Styles.SuccessText.Render("✓ valid name") + "\n")
	}

	if s.ExistingWorktree != nil {
		b.WriteString("\n" + Styles.Footer.Render(indent+"[enter] Switch to existing  [tab] edit name  [esc] cancel"))
	} else {
		b.WriteString("\n" + Styles.Footer.Render(indent+"[enter] next  [backspace] back  [esc] cancel"))
	}

	return Styles.OverlayBorderSuccess.Width(overlayWidth).Render(
		Styles.OverlayTitle.Render("New Worktree") + "\n\n" + b.String(),
	)
}

func renderCreateBranchActionV2(s *CreateState, width int) string {
	overlayWidth := calcOverlayWidth(width)
	contentWidth := overlayWidth - 6
	indent := huhOverlayIndent
	innerWidth := contentWidth - len(indent)*2

	var b strings.Builder

	// Stepper — still on branch phase (index 0)
	stepper := NewStepper("Branch", "Name", "Confirm")
	stepper.Current = 0
	b.WriteString(indentBlock(stepper.View(innerWidth), indent) + "\n\n")

	// Context summary
	b.WriteString(indentBlock(renderContextSummary(s, innerWidth), indent) + "\n\n")

	b.WriteString(indent + fmt.Sprintf("Branch %q already exists\n\n", s.BaseBranch))

	options := []string{
		"Use branch as-is (split)",
		"Create new branch from it (fork)",
	}
	for i, opt := range options {
		cursor := "  "
		if i == s.ActionChoice {
			cursor = Styles.ListCursor.String()
		}
		b.WriteString(indent + cursor + opt + "\n")
	}

	b.WriteString("\n")
	checkbox := "[ ]"
	if s.DontShowAgain {
		checkbox = "[x]"
	}
	b.WriteString(indent + checkbox + " Don't show this again\n")

	b.WriteString("\n" + Styles.Footer.Render(indent+"[enter] confirm  [backspace] back  [esc] cancel  [space] toggle"))

	return Styles.OverlayBorderSuccess.Width(overlayWidth).Render(
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
		lines = append(lines, fmt.Sprintf("Branch:   %s (existing)", s.BaseBranch))
	} else if s.NewBranchName != "" {
		lines = append(lines, fmt.Sprintf("Branch:   %s (new)", s.NewBranchName))
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
	overlayWidth := calcOverlayWidth(width)
	contentWidth := overlayWidth - 6
	indent := huhOverlayIndent
	innerWidth := contentWidth - len(indent)*2

	var b strings.Builder

	// Stepper at step 3 (index 2)
	stepper := NewStepper("Branch", "Name", "Confirm")
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

	if s.Error != "" {
		b.WriteString("\n" + Styles.ErrorText.Render(indent+s.Error) + "\n")
		b.WriteString("\n" + Styles.Footer.Render(indent+"[enter] retry  [backspace] back  [esc] cancel"))
	} else {
		b.WriteString("\n" + Styles.SuccessText.Render(indent+"Ready to create worktree.") + "\n")
		b.WriteString("\n" + Styles.Footer.Render(indent+"[enter] create  [backspace] back  [esc] cancel"))
	}

	return Styles.OverlayBorderSuccess.Width(overlayWidth).Render(
		Styles.OverlayTitle.Render("New Worktree") + "\n\n" + b.String(),
	)
}

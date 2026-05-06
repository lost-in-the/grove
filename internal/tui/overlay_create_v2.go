package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// renderCreateV2 is the V2 dispatcher for the create wizard overlay.
// It integrates the Stepper component and context summary for multi-step flows.
func renderCreateV2(s *CreateState, width int, spinnerView string) string {
	if s.Creating {
		return renderCreateSpinnerV2(s, spinnerView)
	}

	switch s.Step {
	case CreateStepBranchChoice:
		return renderCreateBranchChoiceV2(s, width)
	case CreateStepBranchSelect:
		return renderCreateBranchSelectV2(s, width)
	case CreateStepBranchCreate:
		return renderCreateBranchCreateV2(s, width)
	case CreateStepBranchAction:
		return renderCreateBranchActionV2(s, width)
	case CreateStepName:
		return renderCreateNameV2(s, width)
	case CreateStepConfirm:
		return renderCreateConfirmV2(s, width)
	}
	return ""
}

// overlayIndent is the consistent left-padding for all content inside overlays.
const overlayIndent = "  "

// padToHeight pads content with blank lines so it reaches at least minLines.
// This prevents overlay height jitter when switching between wizard steps.
func padToHeight(content string, minLines int) string {
	lines := strings.Count(content, "\n")
	if lines >= minLines {
		return content
	}
	return content + strings.Repeat("\n", minLines-lines)
}

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

// overlayDims holds precomputed overlay layout dimensions.
type overlayDims struct {
	overlay int // total overlay width (border + padding + content)
	content int // usable width inside the border
	inner   int // content minus indent on both sides
	indent  string
}

// calcOverlayDims computes standard overlay dimensions from terminal width.
func calcOverlayDims(termWidth int) overlayDims {
	ow := calcOverlayWidth(termWidth)
	cw := ow - 6
	return overlayDims{
		overlay: ow,
		content: cw,
		inner:   cw - len(overlayIndent)*2,
		indent:  overlayIndent,
	}
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

	// Activity log (streaming progress) or fallback spinner
	if s.ActivityLog != nil {
		b.WriteString(s.ActivityLog.View(spinnerView))
	} else {
		b.WriteString(spinnerView + " Creating worktree " + Styles.DetailValue.Render(s.Name) + "...\n")
	}

	if s.Error != "" {
		b.WriteString("\n" + Styles.ErrorText.Render(s.Error) + "\n")
	}
	b.WriteString("\n" + Styles.Footer.Render("Please wait..."))

	return Styles.OverlayBorderSuccess.Width(overlayWidth).Render(
		Styles.OverlayTitle.Render("New Worktree") + "\n\n" + b.String(),
	)
}

// renderCreateBranchChoiceV2 renders the initial choice: select existing or create new.
func renderCreateBranchChoiceV2(s *CreateState, width int) string {
	d := calcOverlayDims(width)

	var b strings.Builder

	stepper := NewStepper("Branch", "Name", "Confirm")
	stepper.Current = 0
	b.WriteString(indentBlock(stepper.View(d.inner), d.indent) + "\n\n")

	b.WriteString(d.indent + "How would you like to set up the branch?\n\n")

	options := []string{
		"Select an existing branch",
		"Create a new branch",
	}
	for i, opt := range options {
		cursor := "  "
		if i == s.BranchChoice {
			cursor = Styles.ListCursor.Render("❯ ")
		}
		b.WriteString(d.indent + cursor + opt + "\n")
	}

	content := b.String()
	var footer string
	if s.Source != "" {
		footer = "\n" + Styles.Footer.Render(d.indent+"[enter] select  [shift+tab] back  [esc] cancel")
	} else {
		footer = "\n" + Styles.Footer.Render(d.indent+"[enter] select  [esc] cancel")
	}

	return Styles.OverlayBorderSuccess.Width(d.overlay).Render(
		Styles.OverlayTitle.Render("New Worktree") + "\n\n" + padToHeight(content, createOverlayMinLines) + footer,
	)
}

// renderCreateBranchSelectV2 renders the filterable branch list.
func renderCreateBranchSelectV2(s *CreateState, width int) string {
	d := calcOverlayDims(width)

	var b strings.Builder

	stepper := NewStepper("Branch", "Name", "Confirm")
	stepper.Current = 0
	b.WriteString(indentBlock(stepper.View(d.inner), d.indent) + "\n\n")

	// Show filter input when active
	filter := s.BranchFilterInput.Value()
	if s.BranchFilterMode == BranchFilterOn {
		b.WriteString(d.indent + s.BranchFilterInput.View() + "\n\n")
	} else if filter != "" {
		b.WriteString(d.indent + Styles.DetailDim.Render("Filter: "+filter) + "  " + Styles.Footer.Render("[/] edit") + "\n\n")
	} else {
		b.WriteString(d.indent + "Select a branch\n\n")
	}

	filtered := filteredBranches(s.Branches, filter)
	totalItems := len(filtered)

	if totalItems == 0 {
		b.WriteString(d.indent + Styles.DetailDim.Render("(no matching branches)") + "\n")
	} else {
		start, end := scrollWindow(totalItems, s.BranchCursor, 10)
		for i := start; i < end; i++ {
			cursor := "  "
			if i == s.BranchCursor {
				cursor = Styles.ListCursor.Render("❯ ")
			}
			branchName := filtered[i]
			badge := ""
			if wtName, inUse := s.WorktreeBranches[branchName]; inUse {
				badge = " " + Styles.DetailDim.Render("["+wtName+"]")
			}
			b.WriteString(d.indent + cursor + branchName + badge + "\n")
		}
		if end < totalItems {
			b.WriteString(d.indent + Styles.DetailDim.Render(fmt.Sprintf("… and %d more", totalItems-end)) + "\n")
		}
	}

	content := b.String()
	var footer string
	if s.BranchFilterMode == BranchFilterOn {
		footer = "\n" + Styles.Footer.Render(d.indent+"[enter] accept filter  [esc] clear filter")
	} else {
		footer = "\n" + Styles.Footer.Render(d.indent+"[enter] select  [/] filter  [shift+tab] back  [esc] cancel")
	}

	return Styles.OverlayBorderSuccess.Width(d.overlay).Render(
		Styles.OverlayTitle.Render("New Worktree") + "\n\n" + padToHeight(content, createOverlayMinLines) + footer,
	)
}

// renderCreateBranchCreateV2 renders the new branch name text input.
func renderCreateBranchCreateV2(s *CreateState, width int) string {
	d := calcOverlayDims(width)

	var b strings.Builder

	stepper := NewStepper("Branch", "Name", "Confirm")
	stepper.Current = 0
	b.WriteString(indentBlock(stepper.View(d.inner), d.indent) + "\n\n")

	b.WriteString(d.indent + "Enter a name for the new branch:\n\n")
	b.WriteString(d.indent + s.BranchNameInput.View() + "\n")

	content := b.String()
	footer := "\n" + Styles.Footer.Render(d.indent+"[enter] next  [shift+tab] back  [esc] cancel")

	return Styles.OverlayBorderSuccess.Width(d.overlay).Render(
		Styles.OverlayTitle.Render("New Worktree") + "\n\n" + padToHeight(content, createOverlayMinLines) + footer,
	)
}

// createOverlayMinLines is the fixed content height for the create wizard.
// Set to accommodate the tallest step (branch selector with scroll window).
const createOverlayMinLines = 18

func renderCreateNameV2(s *CreateState, width int) string {
	d := calcOverlayDims(width)

	var b strings.Builder

	// Stepper — on step 2 (index 1)
	stepper := NewStepper("Branch", "Name", "Confirm")
	stepper.Current = 1
	b.WriteString(indentBlock(stepper.View(d.inner), d.indent) + "\n\n")

	// Context summary from previous steps
	b.WriteString(indentBlock(renderContextSummary(s, d.inner), d.indent) + "\n\n")

	// Input with textinput component
	b.WriteString(d.indent + s.NameInput.View() + "\n")

	effectiveName := s.NameInput.Value()
	if effectiveName == "" {
		effectiveName = s.NameSuggestion
	}
	if s.ProjectName != "" && effectiveName != "" {
		b.WriteString(d.indent + Styles.DetailDim.Render(fmt.Sprintf("→ %s-%s", s.ProjectName, effectiveName)) + "\n")
	}

	if s.Error != "" {
		b.WriteString("\n" + d.indent + Styles.ErrorText.Render(s.Error) + "\n")
	} else if s.ExistingWorktree != nil {
		ex := s.ExistingWorktree
		b.WriteString("\n" + d.indent + Styles.ErrorText.Render("✗ Worktree \""+ex.ShortName+"\" already exists") + "\n")
		b.WriteString("\n" + d.indent + "Existing worktree:\n")
		b.WriteString(d.indent + fmt.Sprintf("  Path:     %s\n", ex.Path))
		b.WriteString(d.indent + fmt.Sprintf("  Branch:   %s\n", ex.Branch))
		if ex.IsDirty {
			b.WriteString(d.indent + fmt.Sprintf("  Status:   ● dirty (%d files)\n", len(ex.DirtyFiles)))
		} else {
			b.WriteString(d.indent + "  Status:   ● clean\n")
		}
	} else if effectiveName != "" {
		b.WriteString("\n" + d.indent + Styles.SuccessText.Render("✓ valid name") + "\n")
	}

	content := b.String()
	var footer string
	if s.ExistingWorktree != nil {
		footer = "\n" + Styles.Footer.Render(d.indent+"[enter] Switch to existing  [tab] edit name  [esc] cancel")
	} else {
		footer = "\n" + Styles.Footer.Render(d.indent+"[enter] next  [shift+tab] back  [esc] cancel")
	}

	return Styles.OverlayBorderSuccess.Width(d.overlay).Render(
		Styles.OverlayTitle.Render("New Worktree") + "\n\n" + padToHeight(content, createOverlayMinLines) + footer,
	)
}

func renderCreateBranchActionV2(s *CreateState, width int) string {
	d := calcOverlayDims(width)

	var b strings.Builder

	// Stepper — still on branch phase (index 0)
	stepper := NewStepper("Branch", "Name", "Confirm")
	stepper.Current = 0
	b.WriteString(indentBlock(stepper.View(d.inner), d.indent) + "\n\n")

	// Context summary
	b.WriteString(indentBlock(renderContextSummary(s, d.inner), d.indent) + "\n\n")

	b.WriteString(d.indent + fmt.Sprintf("Branch %q already exists\n\n", s.BaseBranch))

	options := []string{
		"Use branch as-is (split)",
		"Create new branch from it (fork)",
	}
	for i, opt := range options {
		cursor := "  "
		if i == s.ActionChoice {
			cursor = Styles.ListCursor.Render("❯ ")
		}
		b.WriteString(d.indent + cursor + opt + "\n")
	}

	b.WriteString("\n")
	checkbox := checkboxUnchecked
	if s.DontShowAgain {
		checkbox = checkboxChecked
	}
	b.WriteString(d.indent + checkbox + " Don't show this again\n")

	content := b.String()
	footer := "\n" + Styles.Footer.Render(d.indent+"[enter] confirm  [shift+tab] back  [esc] cancel  [space] toggle")

	return Styles.OverlayBorderSuccess.Width(d.overlay).Render(
		Styles.OverlayTitle.Render("New Worktree") + "\n\n" + padToHeight(content, createOverlayMinLines) + footer,
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
	d := calcOverlayDims(width)

	var b strings.Builder

	// Stepper at step 3 (index 2)
	stepper := NewStepper("Branch", "Name", "Confirm")
	stepper.Current = 2
	b.WriteString(indentBlock(stepper.View(d.inner), d.indent) + "\n\n")

	// Full summary
	b.WriteString(indentBlock(renderContextSummary(s, d.inner), d.indent) + "\n\n")

	// Branch strategy detail
	if s.BaseBranch != "" {
		b.WriteString(d.indent + "Strategy: " + Styles.DetailValue.Render("from existing branch") + "\n")
	} else {
		b.WriteString(d.indent + "Strategy: " + Styles.DetailValue.Render("create new branch") + "\n")
	}

	var footer string
	if s.Error != "" {
		b.WriteString("\n" + Styles.ErrorText.Render(d.indent+s.Error) + "\n")
		footer = "\n" + Styles.Footer.Render(d.indent+"[enter] retry  [shift+tab] back  [esc] cancel")
	} else {
		b.WriteString("\n" + Styles.SuccessText.Render(d.indent+"Ready to create worktree.") + "\n")
		footer = "\n" + Styles.Footer.Render(d.indent+"[enter] create  [shift+tab] back  [esc] cancel")
	}

	content := b.String()

	return Styles.OverlayBorderSuccess.Width(d.overlay).Render(
		Styles.OverlayTitle.Render("New Worktree") + "\n\n" + padToHeight(content, createOverlayMinLines) + footer,
	)
}

package tui

import (
	"fmt"
	"strings"
)

// renderDeleteV2 renders an enhanced delete confirmation overlay with
// worktree details, warning box for dirty worktrees, and visual checkbox.
func renderDeleteV2(s *DeleteState, width int) string {
	if s == nil || s.Item == nil {
		return ""
	}

	overlayWidth := width * 2 / 3
	if overlayWidth < 40 {
		overlayWidth = 40
	}
	if overlayWidth > 72 {
		overlayWidth = 72
	}
	innerWidth := overlayWidth - 6

	var b strings.Builder

	// Worktree name header
	b.WriteString(Styles.TextBright.Render(fmt.Sprintf("Delete %q?", s.Item.ShortName)) + "\n\n")

	// Warning box (only for dirty or risky worktrees)
	if len(s.Warnings) > 0 {
		var warningLines []string
		for _, w := range s.Warnings {
			warningLines = append(warningLines, "⚠ "+w)
		}
		if s.Item.IsDirty && len(s.Item.DirtyFiles) > 0 {
			warningLines = append(warningLines,
				fmt.Sprintf("  %d unsaved file(s) will be lost", len(s.Item.DirtyFiles)))
		}
		warningBox := Styles.StatusWarning.Render(strings.Join(warningLines, "\n"))
		b.WriteString(warningBox + "\n\n")
	}

	// Details section
	b.WriteString(renderDeleteDetails(s.Item, innerWidth))
	b.WriteString("\n\n")

	// Checkbox for branch deletion
	checkbox := "[ ]"
	if s.DeleteBranch {
		checkbox = "[x]"
	}
	checkboxLine := fmt.Sprintf("%s Also delete branch %s",
		checkbox, Styles.DetailValue.Render(s.Item.Branch))
	b.WriteString(checkboxLine + "\n\n")

	// Footer with action consequences
	footer := Styles.HelpKey.Render("y") + Styles.HelpDesc.Render(" confirm") + "  " +
		Styles.HelpKey.Render("n") + Styles.HelpDesc.Render(" cancel") + "  " +
		Styles.HelpKey.Render("space") + Styles.HelpDesc.Render(" toggle branch")
	b.WriteString(footer)

	return Styles.OverlayBorderDanger.Width(overlayWidth).Render(
		Styles.OverlayTitle.Render("Delete Worktree") + "\n\n" + b.String(),
	)
}

// renderDeleteDetails renders the worktree detail section for delete confirmation.
func renderDeleteDetails(item *WorktreeItem, width int) string {
	const labelWidth = 10

	label := func(s string) string {
		return Styles.DetailLabel.Render(padRight(s, labelWidth))
	}

	var rows []string

	// Path
	pathVal := truncate(item.Path, width-labelWidth-2)
	rows = append(rows, label("Path")+Styles.DetailValue.Render(pathVal))

	// Branch
	branchVal := truncate(item.Branch, width-labelWidth-2)
	rows = append(rows, label("Branch")+Styles.DetailValue.Render(branchVal))

	// Last commit
	if item.Commit != "" {
		commitVal := Styles.DetailValue.Render(item.Commit)
		if item.CommitAge != "" {
			commitVal += Styles.DetailDim.Render(" · " + item.CommitAge)
		}
		rows = append(rows, label("Commit")+commitVal)
	}

	// Status
	rows = append(rows, label("Status")+renderStatusValue(item))

	return strings.Join(rows, "\n")
}

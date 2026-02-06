package tui

import (
	"fmt"
	"strings"
)

// DeleteState holds the state for the delete confirmation overlay.
type DeleteState struct {
	Item         *WorktreeItem
	Warnings     []string
	DeleteBranch bool
}

func renderDelete(s *DeleteState, width int) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Delete %q?\n\n", s.Item.ShortName)

	for _, w := range s.Warnings {
		b.WriteString(Styles.WarningText.Render("⚠ "+w) + "\n")
	}
	if len(s.Warnings) > 0 {
		b.WriteString("\n")
	}

	checkbox := "[ ]"
	if s.DeleteBranch {
		checkbox = "[x]"
	}
	fmt.Fprintf(&b, "%s Also delete branch %s\n", checkbox, s.Item.Branch)

	b.WriteString("\n" + Styles.Footer.Render("[y] confirm  [n] cancel  [space] toggle branch"))

	return Styles.OverlayBorder.Render(
		Styles.OverlayTitle.Render("Delete Worktree") + "\n\n" + b.String(),
	)
}

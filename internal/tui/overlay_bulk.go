package tui

import (
	"fmt"
	"strings"
)

// BulkState holds the state for the bulk delete overlay.
type BulkState struct {
	Items    []WorktreeItem // merged/deletable worktrees
	Selected []bool         // toggle state per item
	Cursor   int
	Deleting bool   // true while deletions in progress
	Progress string // e.g. "Deleting 2/5..."
}

// SelectedCount returns how many items are selected.
func (b *BulkState) SelectedCount() int {
	count := 0
	for _, s := range b.Selected {
		if s {
			count++
		}
	}
	return count
}

// SelectedItems returns the worktree items that are selected.
func (b *BulkState) SelectedItems() []WorktreeItem {
	var result []WorktreeItem
	for i, s := range b.Selected {
		if s {
			result = append(result, b.Items[i])
		}
	}
	return result
}

func renderBulk(s *BulkState, width int) string {
	var b strings.Builder

	if s.Deleting {
		b.WriteString(s.Progress + "\n")
		return Styles.OverlayBorder.Render(
			Styles.OverlayTitle.Render("Bulk Delete") + "\n\n" + b.String(),
		)
	}

	if len(s.Items) == 0 {
		b.WriteString(Styles.DetailDim.Render("No merged worktrees to clean up.") + "\n")
		b.WriteString("\n" + Styles.Footer.Render("[esc] close"))
		return Styles.OverlayBorder.Render(
			Styles.OverlayTitle.Render("Bulk Delete") + "\n\n" + b.String(),
		)
	}

	fmt.Fprintf(&b, "Select merged worktrees to delete (%d/%d selected)\n\n",
		s.SelectedCount(), len(s.Items))

	maxShow := 12
	start := 0
	if s.Cursor >= maxShow {
		start = s.Cursor - maxShow + 1
	}
	end := start + maxShow
	if end > len(s.Items) {
		end = len(s.Items)
	}

	for i := start; i < end; i++ {
		item := s.Items[i]
		cursor := "  "
		if i == s.Cursor {
			cursor = Styles.ListCursor.String()
		}

		checkbox := "[ ]"
		if s.Selected[i] {
			checkbox = "[x]"
		}

		name := item.ShortName
		branch := Styles.DetailDim.Render(" (" + item.Branch + ")")
		b.WriteString(cursor + checkbox + " " + name + branch + "\n")
	}

	if end < len(s.Items) {
		b.WriteString(Styles.DetailDim.Render(fmt.Sprintf("  … and %d more", len(s.Items)-end)) + "\n")
	}

	b.WriteString("\n" + Styles.Footer.Render("[space] toggle  [enter] delete selected  [esc] cancel"))

	return Styles.OverlayBorder.Render(
		Styles.OverlayTitle.Render("Bulk Delete") + "\n\n" + b.String(),
	)
}

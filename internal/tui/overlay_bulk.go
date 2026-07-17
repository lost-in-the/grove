package tui

import (
	"fmt"
	"strings"
)

// BulkState holds the state for the bulk delete overlay.
type BulkState struct {
	Items    []WorktreeItem // deletable worktrees (excludes main, protected, current)
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

// SelectedDirtyCount returns how many selected items have uncommitted changes.
func (b *BulkState) SelectedDirtyCount() int {
	count := 0
	for i, s := range b.Selected {
		if s && b.Items[i].IsDirty {
			count++
		}
	}
	return count
}

func renderBulk(s *BulkState) string {
	var b strings.Builder

	if s.Deleting {
		b.WriteString(s.Progress + "\n")
		return Styles.OverlayBorderDanger.Render(
			Styles.OverlayTitle.Render("Bulk Delete") + "\n\n" + b.String(),
		)
	}

	if len(s.Items) == 0 {
		b.WriteString(Styles.DetailDim.Render("No worktrees to clean up.") + "\n")
		b.WriteString("\n" + Styles.Footer.Render("[esc] close"))
		return Styles.OverlayBorderDanger.Render(
			Styles.OverlayTitle.Render("Bulk Delete") + "\n\n" + b.String(),
		)
	}

	fmt.Fprintf(&b, "Select worktrees to delete (%d/%d selected)\n\n",
		s.SelectedCount(), len(s.Items))

	start, end := scrollWindow(len(s.Items), s.Cursor, 12)

	for i := start; i < end; i++ {
		item := s.Items[i]
		cursor := "  "
		if i == s.Cursor {
			cursor = Styles.ListCursor.Render("❯ ")
		}

		checkbox := checkboxUnchecked
		if s.Selected[i] {
			checkbox = checkboxChecked
		}

		name := item.ShortName
		branch := Styles.DetailDim.Render(" (" + item.Branch + ")")
		row := cursor + checkbox + " " + name + branch
		// Deletion is forced (no second chance at git's dirty refusal), so
		// dirty candidates must be visible before the single confirm —
		// parity with the single-delete overlay's warning box.
		if item.IsDirty {
			row += Styles.StatusWarning.Render(" ⚠ uncommitted changes")
		}
		b.WriteString(row + "\n")
	}

	if end < len(s.Items) {
		b.WriteString(Styles.DetailDim.Render(fmt.Sprintf("  … and %d more", len(s.Items)-end)) + "\n")
	}

	if n := s.SelectedDirtyCount(); n > 0 {
		b.WriteString("\n" + Styles.StatusWarning.Render(
			fmt.Sprintf("⚠ %d selected worktree(s) have uncommitted changes — deleting discards them", n)) + "\n")
	}

	b.WriteString("\n" + Styles.Footer.Render("[space] toggle  [enter] delete selected  [esc] cancel"))

	return Styles.OverlayBorderDanger.Render(
		Styles.OverlayTitle.Render("Bulk Delete") + "\n\n" + b.String(),
	)
}

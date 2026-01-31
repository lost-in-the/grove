package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// delegateColumns holds responsive column widths for list rendering.
type delegateColumns struct {
	Name   int
	Branch int
	Age    int
}

// delegateColumnsV2 calculates column widths based on terminal width.
func delegateColumnsV2(width int) delegateColumns {
	switch {
	case width > 120:
		return delegateColumns{Name: 24, Branch: 20, Age: 8}
	case width > 90:
		return delegateColumns{Name: 20, Branch: 16, Age: 8}
	default:
		return delegateColumns{Name: 16, Branch: 12, Age: 6}
	}
}

// worktreeIndicator returns the leading indicator for a worktree item.
// Priority: current (green ●) > dirty (yellow ●) > stale (✗) > selected (❯) > normal (space).
func worktreeIndicator(item WorktreeItem, selected bool) string {
	switch {
	case item.IsCurrent:
		return Styles.StatusSuccess.Render("●")
	case item.IsDirty:
		return Styles.StatusWarning.Render("●")
	case item.IsPrunable:
		return Styles.StatusDanger.Render("✗")
	case selected:
		return Styles.ListCursor.Render("❯")
	default:
		return " "
	}
}

// worktreeStatusTextV2 returns a status string for the item.
func worktreeStatusTextV2(item WorktreeItem) string {
	if item.IsPrunable {
		return Styles.StatusDanger.Render("stale")
	}
	if item.IsDirty {
		count := len(item.DirtyFiles)
		if count > 0 {
			return Styles.StatusWarning.Render(fmt.Sprintf("dirty (%d)", count))
		}
		return Styles.StatusWarning.Render("dirty")
	}
	return Styles.StatusSuccess.Render("clean")
}

// worktreeTmuxBadgeV2 returns a tmux badge if a session exists.
func worktreeTmuxBadgeV2(item WorktreeItem) string {
	switch item.TmuxStatus {
	case "attached":
		return Styles.TmuxBadge.Render("⬡ tmux")
	case "detached":
		return Styles.TmuxBadge.Render("⬡ tmux")
	default:
		return ""
	}
}

// worktreeSyncBadgeV2 returns a compact sync status badge for the list.
func worktreeSyncBadgeV2(item WorktreeItem) string {
	if !item.HasRemote {
		return ""
	}
	if item.AheadCount == 0 && item.BehindCount == 0 {
		return ""
	}
	var parts []string
	if item.AheadCount > 0 {
		parts = append(parts, Styles.StatusSuccess.Render(fmt.Sprintf("↑%d", item.AheadCount)))
	}
	if item.BehindCount > 0 {
		parts = append(parts, Styles.StatusWarning.Render(fmt.Sprintf("↓%d", item.BehindCount)))
	}
	return strings.Join(parts, "")
}

// WorktreeDelegateV2 implements list.ItemDelegate with visual indicators.
type WorktreeDelegateV2 struct{}

// NewWorktreeDelegateV2 creates a new V2 delegate.
func NewWorktreeDelegateV2() WorktreeDelegateV2 {
	return WorktreeDelegateV2{}
}

func (d WorktreeDelegateV2) Height() int                             { return 1 }
func (d WorktreeDelegateV2) Spacing() int                            { return 0 }
func (d WorktreeDelegateV2) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d WorktreeDelegateV2) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(WorktreeItem)
	if !ok {
		return
	}

	selected := index == m.Index()
	width := m.Width()
	cols := delegateColumnsV2(width)

	// Number prefix for quick-switch (1-9)
	numPrefix := "  "
	if index < 9 {
		numPrefix = Styles.TextMuted.Render(fmt.Sprintf("%d ", index+1))
	}

	indicator := worktreeIndicator(item, selected) + " "

	// Name styling
	nameStyle := Styles.NormalItem
	if selected {
		nameStyle = Styles.SelectedItem
	}
	if item.IsCurrent {
		nameStyle = Styles.CurrentItem
		if selected {
			nameStyle = nameStyle.Bold(true)
		}
	}
	name := nameStyle.Render(fmt.Sprintf("%-*s", cols.Name, truncate(item.ShortName, cols.Name)))

	branch := Styles.TextMuted.Render(fmt.Sprintf("%-*s", cols.Branch, truncate(item.Branch, cols.Branch)))

	age := ""
	if item.CommitAge != "" {
		age = Styles.TextMuted.Render(fmt.Sprintf("%-*s", cols.Age, compactAge(item.CommitAge)))
	} else {
		age = fmt.Sprintf("%-*s", cols.Age, "")
	}

	status := worktreeStatusTextV2(item)
	syncBadge := worktreeSyncBadgeV2(item)
	tmuxBadge := worktreeTmuxBadgeV2(item)

	line := numPrefix + indicator + name + "  " + branch + "  " + age + "  " + status
	if syncBadge != "" {
		line += "  " + syncBadge
	}
	if tmuxBadge != "" {
		line += "  " + tmuxBadge
	}

	rendered := lipgloss.NewStyle().MaxWidth(width).Render(line)
	fmt.Fprint(w, rendered)
}

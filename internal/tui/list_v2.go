package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/LeahArmstrong/grove-cli/internal/plugins"
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
	case width >= 80:
		return delegateColumns{Name: 16, Branch: 12, Age: 6}
	case width >= 60:
		return delegateColumns{Name: 14, Branch: 10, Age: 0}
	default:
		// Very narrow: name only, no branch or age
		name := 12
		if width < 50 {
			name = 10
		}
		return delegateColumns{Name: name, Branch: 0, Age: 0}
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
// Uses ⬢ (filled) for attached, ⬡ (unfilled) for detached — consistent
// with the symbol convention in the table-style list delegate.
func worktreeTmuxBadgeV2(item WorktreeItem) string {
	switch item.TmuxStatus {
	case "attached":
		return Styles.TmuxBadge.Render("⬢ tmux")
	case "detached":
		return Styles.TmuxBadge.Render("⬡ tmux")
	default:
		return ""
	}
}

// worktreeContainerBadgeV2 returns a container status badge from plugin statuses.
func worktreeContainerBadgeV2(item WorktreeItem) string {
	for _, s := range item.PluginStatuses {
		if s.Short == "" {
			continue
		}
		switch s.Level {
		case plugins.StatusActive:
			return Styles.ContainerBadgeActive.Render("◆ " + s.Short)
		case plugins.StatusWarning:
			return Styles.ContainerBadgeWarn.Render("◆ " + s.Short)
		case plugins.StatusInfo:
			return Styles.ContainerBadge.Render("◇ " + s.Short)
		default:
			return Styles.TextMuted.Render("◇ " + s.Short)
		}
	}
	return ""
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
		parts = append(parts, Styles.StatusDanger.Render(fmt.Sprintf("↓%d", item.BehindCount)))
	}
	return strings.Join(parts, "")
}

// WorktreeDelegateV2 implements list.ItemDelegate with visual indicators.
type WorktreeDelegateV2 struct{}

// NewWorktreeDelegateV2 creates a new V2 delegate.
func NewWorktreeDelegateV2() WorktreeDelegateV2 {
	return WorktreeDelegateV2{}
}

func (d WorktreeDelegateV2) Height() int                             { return 2 }
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
	name := nameStyle.Render(truncate(item.ShortName, cols.Name))

	// Line 1: numPrefix + indicator + name
	line1 := numPrefix + indicator + name

	// Line 2: metadata row (indented to align with name)
	prefixPad := "     " // align under name (2 num + 1 indicator + 1 space + 1)
	var metaParts []string

	if cols.Branch > 0 {
		metaParts = append(metaParts, truncate(item.Branch, cols.Branch))
	}

	if cols.Age > 0 && item.CommitAge != "" {
		metaParts = append(metaParts, compactAge(item.CommitAge))
	}

	metaParts = append(metaParts, cleanAnsi(worktreeStatusTextV2(item)))

	syncBadge := worktreeSyncBadgeV2(item)
	if syncBadge != "" {
		metaParts = append(metaParts, cleanAnsi(syncBadge))
	}
	tmuxBadge := worktreeTmuxBadgeV2(item)
	if tmuxBadge != "" {
		metaParts = append(metaParts, cleanAnsi(tmuxBadge))
	}
	containerBadge := worktreeContainerBadgeV2(item)
	if containerBadge != "" {
		metaParts = append(metaParts, cleanAnsi(containerBadge))
	}

	line2 := prefixPad + Styles.TextMuted.Render(strings.Join(metaParts, " · "))

	// Apply selection background to both lines
	if selected {
		line1 = padToWidth(line1, width)
		line2 = padToWidth(line2, width)
		line1 = Styles.SelectionRow.MaxWidth(width).Render(line1)
		line2 = Styles.SelectionRow.MaxWidth(width).Render(line2)
	} else {
		line1 = lipgloss.NewStyle().MaxWidth(width).Render(line1)
		line2 = lipgloss.NewStyle().MaxWidth(width).Render(line2)
	}

	_, _ = fmt.Fprint(w, line1+"\n"+line2)
}

// padToWidth pads a string with spaces to reach the target width.
func padToWidth(s string, width int) string {
	w := lipgloss.Width(s)
	if w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}

// cleanAnsi removes ANSI escape sequences from a string for plain-text display.
func cleanAnsi(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

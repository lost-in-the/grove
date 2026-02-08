package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// WorktreeDelegate implements list.ItemDelegate for rendering worktree items.
type WorktreeDelegate struct{}

// NewWorktreeDelegate creates a new delegate for rendering worktree list items.
func NewWorktreeDelegate() WorktreeDelegate {
	return WorktreeDelegate{}
}

func (d WorktreeDelegate) Height() int                             { return 1 }
func (d WorktreeDelegate) Spacing() int                            { return 0 }
func (d WorktreeDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d WorktreeDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(WorktreeItem)
	if !ok {
		return
	}

	selected := index == m.Index()
	width := m.Width()

	// Dynamic column widths based on available space
	nameWidth := 16
	branchWidth := 12
	ageWidth := 8
	if width > 100 {
		nameWidth = 24
		branchWidth = 20
	} else if width > 80 {
		nameWidth = 20
		branchWidth = 16
	}

	// Number prefix for quick-switch (1-9)
	numPrefix := "  "
	if index < 9 {
		numPrefix = Styles.DetailDim.Render(fmt.Sprintf("%d ", index+1))
	}

	var cursor string
	if selected {
		cursor = Styles.ListCursor.String()
	} else {
		cursor = Styles.ListCursorDim.String()
	}

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
	name := nameStyle.Render(fmt.Sprintf("%-*s", nameWidth, truncate(item.ShortName, nameWidth)))

	branch := Styles.DetailDim.Render(fmt.Sprintf("%-*s", branchWidth, truncate(item.Branch, branchWidth)))

	age := strings.Repeat(" ", ageWidth)
	if item.CommitAge != "" {
		age = Styles.DetailDim.Render(fmt.Sprintf("%-*s", ageWidth, compactAge(item.CommitAge)))
	}

	status := item.StatusText()
	tmuxText := item.TmuxText()

	line := numPrefix + cursor + name + "  " + branch + "  " + age + "  " + status
	if tmuxText != "" {
		line += "  " + tmuxText
	}

	// Truncate to width to avoid wrapping
	rendered := lipgloss.NewStyle().MaxWidth(width).Render(line)
	fmt.Fprint(w, rendered)
}

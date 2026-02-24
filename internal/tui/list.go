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

// WorktreeDelegate implements list.ItemDelegate for rendering worktree items.
// Column widths are pre-computed by computeColumnWidths() and stored here
// so Render() can lay out rows without re-scanning items each frame.
type WorktreeDelegate struct {
	NameWidth   int
	BranchWidth int
	AgeWidth    int
	SyncWidth   int
}

// Fixed column widths that don't change with content.
const (
	colNumWidth       = 2 // "1 " or "  "
	colIndicatorWidth = 2 // "● " or "  "
	colGitWidth       = 1 // "●", "✓", or "✗"
	colTmuxWidth      = 1 // "⬢", "⬡", or " "
	colContainerWidth = 3 // up to 3 symbols side-by-side
	colGapCount       = 7 // single-space gaps between columns
)

// colFixedOverhead is the total width consumed by fixed-width columns and gaps.
const colFixedOverhead = colNumWidth + colIndicatorWidth + colGitWidth + colTmuxWidth + colContainerWidth + colGapCount

// NewWorktreeDelegate creates a new delegate with default column widths.
func NewWorktreeDelegate() WorktreeDelegate {
	return WorktreeDelegate{
		NameWidth:   16,
		BranchWidth: 12,
		AgeWidth:    7,
		SyncWidth:   4,
	}
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

	// Number prefix (2 chars: digit + space, or 2 spaces)
	numPrefix := "  "
	if index < 9 {
		numPrefix = Styles.DetailDim.Render(fmt.Sprintf("%d", index+1)) + " "
	}

	// Indicator (2 chars: symbol + space)
	indicator := worktreeIndicator(item, selected) + " "

	// Name
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
	name := nameStyle.Render(padRight(truncate(item.ShortName, d.NameWidth), d.NameWidth))

	// Branch
	branch := Styles.DetailDim.Render(padRight(truncate(item.Branch, d.BranchWidth), d.BranchWidth))

	// Age
	age := strings.Repeat(" ", d.AgeWidth)
	if item.CommitAge != "" {
		age = Styles.DetailDim.Render(padRight(compactAge(item.CommitAge), d.AgeWidth))
	}

	// Git status symbol (1 char)
	gitSym := gitStatusSymbol(item)

	// Sync symbol (padded to SyncWidth)
	syncSym := syncStatusSymbol(item, d.SyncWidth)

	// Tmux symbol (1 char)
	tmuxSym := tmuxStatusSymbol(item)

	// Container symbols (up to 3 chars)
	containerSym := containerStatusSymbols(item)

	// Assemble with single-space gaps between all columns
	line := numPrefix + indicator +
		name + " " +
		branch + " " +
		age + " " +
		gitSym + " " +
		syncSym + " " +
		tmuxSym + " " +
		containerSym

	// Apply selection background
	if selected {
		line = padToWidth(line, width)
		line = Styles.SelectionRow.Width(width).Render(line)
	} else {
		line = lipgloss.NewStyle().MaxWidth(width).Render(line)
	}

	_, _ = fmt.Fprint(w, line)
}

// renderListHeader renders a column header + separator line using the
// delegate's pre-computed widths. Returns 2 lines (header + separator).
func renderListHeader(d WorktreeDelegate, width int) string {
	dim := Styles.TextMuted

	line := "    " + // num(2) + indicator(2)
		dim.Render(padRight("NAME", d.NameWidth)) + " " +
		dim.Render(padRight("BRANCH", d.BranchWidth)) + " " +
		dim.Render(padRight("AGE", d.AgeWidth)) + " " +
		dim.Render("±") + " " +
		dim.Render(padRight("↕", d.SyncWidth)) + " " +
		dim.Render("⬡") + " " +
		dim.Render("◆")

	sepWidth := width
	if sepWidth < 0 {
		sepWidth = 0
	}
	sep := dim.Render(strings.Repeat("─", sepWidth))
	return line + "\n" + sep
}

// ComputeDelegateWidths computes content-adaptive column widths from a slice
// of list items and total available width. Exported for use by the snapshot tool.
func ComputeDelegateWidths(items []list.Item, totalWidth int) WorktreeDelegate {
	maxName, maxBranch := 0, 0
	for _, li := range items {
		item, ok := li.(WorktreeItem)
		if !ok {
			continue
		}
		if n := len([]rune(item.ShortName)); n > maxName {
			maxName = n
		}
		if n := len([]rune(item.Branch)); n > maxBranch {
			maxBranch = n
		}
	}

	d := NewWorktreeDelegate()

	available := totalWidth - colFixedOverhead - d.AgeWidth - d.SyncWidth
	if available < 20 {
		available = 20
	}

	nameW := maxName
	branchW := maxBranch

	if nameW+branchW <= available {
		d.NameWidth = nameW
		d.BranchWidth = branchW
	} else {
		nameW = available * 60 / 100
		branchW = available - nameW
		if nameW > maxName {
			surplus := nameW - maxName
			nameW = maxName
			branchW += surplus
		}
		if branchW > maxBranch {
			surplus := branchW - maxBranch
			branchW = maxBranch
			nameW += surplus
			if nameW > maxName {
				nameW = maxName
			}
		}
		d.NameWidth = nameW
		d.BranchWidth = branchW
	}

	return d
}

// RenderListHeader is an exported wrapper around renderListHeader for
// use by the snapshot tool.
func RenderListHeader(d WorktreeDelegate, width int) string {
	return renderListHeader(d, width)
}

// --- Symbol rendering helpers ---

// gitStatusSymbol returns a single-char git status indicator.
func gitStatusSymbol(item WorktreeItem) string {
	switch {
	case item.IsPrunable:
		return Styles.StatusDanger.Render("✗")
	case item.IsDirty:
		return Styles.StatusWarning.Render("●")
	default:
		return Styles.StatusSuccess.Render("✓")
	}
}

// syncStatusSymbol returns a sync indicator padded to syncWidth.
// Green ↑N = ahead, red ↓N = behind, blank = synced or no remote.
func syncStatusSymbol(item WorktreeItem, syncWidth int) string {
	if !item.HasRemote {
		return padRight(" ", syncWidth)
	}
	if item.AheadCount > 0 && item.BehindCount > 0 {
		return padRight(
			Styles.StatusSuccess.Render(fmt.Sprintf("↑%d", item.AheadCount))+
				Styles.StatusDanger.Render(fmt.Sprintf("↓%d", item.BehindCount)), syncWidth)
	}
	if item.AheadCount > 0 {
		return padRight(Styles.StatusSuccess.Render(fmt.Sprintf("↑%d", item.AheadCount)), syncWidth)
	}
	if item.BehindCount > 0 {
		return padRight(Styles.StatusDanger.Render(fmt.Sprintf("↓%d", item.BehindCount)), syncWidth)
	}
	return padRight(" ", syncWidth)
}

// tmuxStatusSymbol returns a single-char tmux indicator.
// ⬢ purple = attached, ⬡ purple = detached, space = none.
func tmuxStatusSymbol(item WorktreeItem) string {
	switch item.TmuxStatus {
	case "attached":
		return Styles.TmuxBadge.Render("⬢")
	case "detached":
		return Styles.TmuxBadge.Render("⬡")
	default:
		return " "
	}
}

// containerStatusSymbols returns up to 3 container status symbols.
// ◆ = active/warning, ◇ = info/other.
func containerStatusSymbols(item WorktreeItem) string {
	if len(item.PluginStatuses) == 0 {
		return " "
	}
	var parts []string
	for _, s := range item.PluginStatuses {
		if s.Short == "" {
			continue
		}
		switch s.Level {
		case plugins.StatusActive:
			parts = append(parts, Styles.ContainerBadgeActive.Render("◆"))
		case plugins.StatusWarning:
			parts = append(parts, Styles.ContainerBadgeWarn.Render("◆"))
		case plugins.StatusInfo:
			parts = append(parts, Styles.ContainerBadge.Render("◇"))
		default:
			parts = append(parts, Styles.TextMuted.Render("◇"))
		}
	}
	if len(parts) == 0 {
		return " "
	}
	return strings.Join(parts, "")
}

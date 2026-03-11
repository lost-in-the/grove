package tui

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/lost-in-the/grove/internal/plugins"
)

// ComputeDelegateWidthsV2 computes content-adaptive column widths for the V2
// delegate by scanning items for max name/branch rune lengths.
func ComputeDelegateWidthsV2(items []list.Item, width int) WorktreeDelegateV2 {
	maxName, maxBranch := 0, 0
	for _, li := range items {
		item, ok := li.(WorktreeItem)
		if !ok {
			continue
		}
		if n := lipgloss.Width(item.ShortName); n > maxName {
			maxName = n
		}
		if n := lipgloss.Width(item.Branch); n > maxBranch {
			maxBranch = n
		}
	}

	d := WorktreeDelegateV2{}
	d.initStyles()

	// Scale caps with available width so ultrawide terminals use the space
	// Branch is the primary identifier (larger cap), directory is secondary
	branchCap := 40
	nameCap := 30
	if width >= 120 {
		branchCap = 55
		nameCap = 40
	} else if width >= 90 {
		branchCap = 45
		nameCap = 35
	}

	// Branch: use actual max, capped and floored (primary identifier)
	d.BranchWidth = maxBranch
	if d.BranchWidth > branchCap {
		d.BranchWidth = branchCap
	}
	if d.BranchWidth < 10 {
		d.BranchWidth = 10
	}

	// Name (directory): use actual max, capped; hidden at narrow widths
	if width < 60 {
		d.NameWidth = 0
	} else {
		d.NameWidth = maxName
		if d.NameWidth > nameCap {
			d.NameWidth = nameCap
		}
	}

	return d
}

// worktreeIndicator returns the leading indicator for a worktree item.
// Selected always shows ❯. Non-selected shows status: ● (current/dirty), ✗ (stale), ○ (clean).
func worktreeIndicator(item WorktreeItem, selected bool) string {
	return worktreeIndicatorBg(item, selected)
}

// worktreeIndicatorBg returns the indicator with optional selection background.
func worktreeIndicatorBg(item WorktreeItem, selected bool) string {
	withBg := func(s lipgloss.Style) lipgloss.Style {
		if selected {
			return s.Background(Colors.SelectionBg)
		}
		return s
	}
	if selected {
		return withBg(Styles.ListCursor).Render("❯")
	}
	switch {
	case item.IsCurrent:
		return withBg(Styles.StatusSuccess).Render("●")
	case item.IsDirty:
		return withBg(Styles.StatusWarning).Render("●")
	case item.IsPrunable:
		return withBg(Styles.StatusDanger).Render("✗")
	default:
		return withBg(Styles.TextMuted).Render("○")
	}
}

// compactIndicators returns compact status indicators for line 1.
// Format: ↑N ↓N ~N (sync ahead/behind, dirty count). Stale shows ✗.
func compactIndicators(item WorktreeItem) string {
	return compactIndicatorsBg(item, false)
}

// compactIndicatorsBg returns compact indicators with optional selection background.
func compactIndicatorsBg(item WorktreeItem, selected bool) string {
	withBg := func(s lipgloss.Style) lipgloss.Style {
		if selected {
			return s.Background(Colors.SelectionBg)
		}
		return s
	}
	if item.IsPrunable {
		return withBg(Styles.StatusDanger).Render("✗")
	}
	var parts []string
	if item.AheadCount > 0 {
		parts = append(parts, withBg(Styles.StatusSuccess).Render(fmt.Sprintf("↑%d", item.AheadCount)))
	}
	if item.BehindCount > 0 {
		parts = append(parts, withBg(Styles.StatusDanger).Render(fmt.Sprintf("↓%d", item.BehindCount)))
	}
	if len(item.DirtyFiles) > 0 {
		parts = append(parts, withBg(Styles.StatusWarning).Render(fmt.Sprintf("~%d", len(item.DirtyFiles))))
	}
	if selected && len(parts) > 1 {
		spacer := lipgloss.NewStyle().Background(Colors.SelectionBg).Render(" ")
		return strings.Join(parts, spacer)
	}
	return strings.Join(parts, " ")
}

// renderBadgesV2 returns right-aligned badges for line 2.
// Order: container statuses first, tmux last (fixed-width, most common).
func renderBadgesV2(item WorktreeItem) string {
	return renderBadgesV2Bg(item, false)
}

// renderBadgesV2Bg returns badges with optional selection background.
func renderBadgesV2Bg(item WorktreeItem, selected bool) string {
	withBg := func(s lipgloss.Style) lipgloss.Style {
		if selected {
			return s.Background(Colors.SelectionBg)
		}
		return s
	}

	var parts []string

	// Container badges first
	for _, s := range item.PluginStatuses {
		if s.Short == "" {
			continue
		}
		switch s.Level {
		case plugins.StatusActive:
			parts = append(parts, withBg(Styles.ContainerBadgeActive).Render("◆ "+s.Short))
		case plugins.StatusWarning:
			parts = append(parts, withBg(Styles.ContainerBadgeWarn).Render("◆ "+s.Short))
		case plugins.StatusInfo:
			parts = append(parts, withBg(Styles.ContainerBadge).Render("◇ "+s.Short))
		default:
			parts = append(parts, withBg(Styles.TextMuted).Render("◇ "+s.Short))
		}
	}

	// Tmux badge last (fixed-width text, most frequently present)
	switch item.TmuxStatus {
	case "attached":
		parts = append(parts, withBg(Styles.TmuxBadgeActive).Render("⬢ tmux"))
	case "detached":
		parts = append(parts, withBg(Styles.TmuxBadge).Render("⬡ tmux"))
	}

	if selected && len(parts) > 1 {
		spacer := lipgloss.NewStyle().Background(Colors.SelectionBg).Render(" ")
		return strings.Join(parts, spacer)
	}
	return strings.Join(parts, " ")
}

// WorktreeDelegateV2 implements list.ItemDelegate with visual indicators.
type WorktreeDelegateV2 struct {
	NameWidth   int
	BranchWidth int

	// Cached styles (depend only on static Colors, computed once)
	divStyle       lipgloss.Style
	divStyleSel    lipgloss.Style
	branchStyle    lipgloss.Style
	branchStyleSel lipgloss.Style
	selBgStyle     lipgloss.Style
}

// initStyles pre-computes the static styles used in the render hot path.
func (d *WorktreeDelegateV2) initStyles() {
	d.divStyle = lipgloss.NewStyle().Foreground(Colors.SurfaceDim)
	d.divStyleSel = d.divStyle.Background(Colors.SelectionBg)
	d.branchStyle = lipgloss.NewStyle().Foreground(Colors.Primary)
	d.branchStyleSel = d.branchStyle.Background(Colors.SelectionBg)
	d.selBgStyle = lipgloss.NewStyle().Background(Colors.SelectionBg)
}

// NewWorktreeDelegateV2 creates a new V2 delegate with default widths.
func NewWorktreeDelegateV2() WorktreeDelegateV2 {
	d := WorktreeDelegateV2{NameWidth: 20, BranchWidth: 16}
	d.initStyles()
	return d
}

func (d WorktreeDelegateV2) Height() int                             { return 2 }
func (d WorktreeDelegateV2) Spacing() int                            { return 1 }
func (d WorktreeDelegateV2) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d WorktreeDelegateV2) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(WorktreeItem)
	if !ok {
		return
	}

	selected := index == m.Index()
	width := m.Width()

	// When selected, add SelectionBg to every style so the background is
	// continuous across the row. Nested ANSI resets ([m) break outer
	// backgrounds, so each segment must carry its own background.
	withBg := func(s lipgloss.Style) lipgloss.Style {
		if selected {
			return s.Background(Colors.SelectionBg)
		}
		return s
	}
	// bgSpace renders plain spaces with the selection background.
	bgSpace := func(n int) string {
		if n <= 0 {
			return ""
		}
		s := strings.Repeat(" ", n)
		if selected {
			return d.selBgStyle.Render(s)
		}
		return s
	}

	mutedStyle := withBg(Styles.TextMuted)
	// Dim style for commit messages — readable but subordinate
	dimStyle := withBg(Styles.DimmedItem)
	// Cached styles for structural elements and branch text
	divStyle := d.divStyle
	branchStyle := d.branchStyle
	if selected {
		divStyle = d.divStyleSel
		branchStyle = d.branchStyleSel
	}

	sep := divStyle.Render(" │ ")
	const sepLen = 3 // visible width of " │ "

	// === LINE 1: num indicator branch │ directory │ indicators ──── age ===

	// Number prefix (2 chars: "N " or "  ")
	var numPrefix string
	numLen := 2
	if index < 9 {
		numPrefix = divStyle.Render(fmt.Sprintf("%d ", index+1))
	} else {
		numPrefix = bgSpace(2)
	}

	// Status indicator (2 chars: "● " or "❯ ")
	indicator := worktreeIndicatorBg(item, selected) + bgSpace(1)
	const indicatorLen = 2

	// Branch (primary identifier, rendered first without separator)
	branchText := truncate(item.Branch, d.BranchWidth)
	branch := branchStyle.Render(branchText)
	branchLen := lipgloss.Width(branchText)

	// Directory name (secondary, with separator; hidden at narrow widths)
	nameStyle := withBg(Styles.NormalItem)
	if selected {
		nameStyle = withBg(Styles.SelectedItem)
	}
	if item.IsCurrent {
		nameStyle = withBg(Styles.CurrentItem)
		if selected {
			nameStyle = nameStyle.Bold(true)
		}
	}
	var namePart string
	nameLen := 0
	if d.NameWidth > 0 {
		nameText := truncate(item.ShortName, d.NameWidth)
		namePart = sep + nameStyle.Render(nameText)
		nameLen = sepLen + lipgloss.Width(nameText)
	}

	// Compact indicators (↑N ↓N ~N)
	indicators := compactIndicatorsBg(item, selected)
	indVisLen := lipgloss.Width(indicators)
	var indPart string
	indPartLen := 0
	if indVisLen > 0 {
		indPart = sep + indicators
		indPartLen = sepLen + indVisLen
	} else {
		indPart = ""
		indPartLen = 0
	}

	// Age (right-aligned after rule fill)
	age := ""
	ageLen := 0
	if item.CommitAge != "" {
		age = compactAge(item.CommitAge)
		ageLen = lipgloss.Width(age)
	}

	// Rule fill: bridges gap between indicators and age
	usedLen := numLen + indicatorLen + branchLen + nameLen + indPartLen
	ruleSpace := width - usedLen - ageLen
	if ageLen > 0 {
		ruleSpace -= 2 // spaces around rule/before age
	}

	var rulePart string
	if ruleSpace > 0 {
		rulePart = bgSpace(1) + divStyle.Render(strings.Repeat("─", ruleSpace))
		if ageLen > 0 {
			rulePart += bgSpace(1)
		}
	} else if ageLen > 0 {
		rulePart = bgSpace(1)
	}

	agePart := ""
	if ageLen > 0 {
		agePart = mutedStyle.Render(age)
	}

	line1 := numPrefix + indicator + branch + namePart + indPart + rulePart + agePart

	// === LINE 2: commit message (left) + badges (right-aligned) ===
	const line2Pad = 6 // indent to align under name (num:2 + indicator:2 + 2 spaces)
	padStr := bgSpace(line2Pad)

	badges := renderBadgesV2Bg(item, selected)
	badgesVisLen := lipgloss.Width(badges)
	commitText := item.CommitMessage
	availL2 := width - line2Pad

	var line2 string
	if badgesVisLen > 0 && commitText != "" {
		// Both: commit left, badges right
		msgSpace := availL2 - badgesVisLen - 1
		if msgSpace > 10 {
			msg := dimStyle.Render(truncate(commitText, msgSpace))
			msgVisLen := lipgloss.Width(msg)
			gap := availL2 - msgVisLen - badgesVisLen
			if gap < 1 {
				gap = 1
			}
			line2 = padStr + msg + bgSpace(gap) + badges
		} else {
			// Not enough room for commit, just badges right-aligned
			gap := availL2 - badgesVisLen
			if gap < 0 {
				gap = 0
			}
			line2 = padStr + bgSpace(gap) + badges
		}
	} else if badgesVisLen > 0 {
		// Badges only, right-aligned
		gap := availL2 - badgesVisLen
		if gap < 0 {
			gap = 0
		}
		line2 = padStr + bgSpace(gap) + badges
	} else if commitText != "" {
		// Commit message only (fallback)
		line2 = padStr + dimStyle.Render(truncate(commitText, availL2))
	} else {
		line2 = padStr
	}

	// Pad remaining width so selection background covers the full row
	line1 = line1 + bgSpace(width-lipgloss.Width(line1))
	line2 = line2 + bgSpace(width-lipgloss.Width(line2))

	line1 = lipgloss.NewStyle().MaxWidth(width).Render(line1)
	line2 = lipgloss.NewStyle().MaxWidth(width).Render(line2)

	_, _ = fmt.Fprint(w, lipgloss.JoinVertical(lipgloss.Left, line1, line2))
}

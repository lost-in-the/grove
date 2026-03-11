package tui

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/lost-in-the/grove/internal/plugins"
)

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// renderDetailV2 builds an enhanced detail panel with bordered card,
// metadata grid, sync status, changed files list, and tmux indicator.
func renderDetailV2(item *WorktreeItem, width int) string {
	if item == nil || width < 20 {
		return ""
	}

	// Reserve space for border padding
	innerWidth := max(width-6, 16)

	var sections []string

	// Metadata grid: label-value pairs
	sections = append(sections, renderMetadataGrid(item, innerWidth))

	// Changed files section (only if dirty)
	if len(item.DirtyFiles) > 0 {
		sections = append(sections, renderChangesSection(item.DirtyFiles, innerWidth))
	}

	body := strings.Join(sections, "\n\n")

	// Wrap in bordered card with title
	card := Styles.DetailBorder.
		Width(width - 2).
		Render(body)

	// Insert title into top border
	title := " " + Styles.DetailTitle.Render(item.ShortName) + " "
	card = injectBorderTitle(card, title)

	return card
}

// renderSectionHeader renders a thin ruled header like "── Git ──────────".
func renderSectionHeader(title string, width int) string {
	label := " " + title + " "
	labelLen := len(label)
	leftLen := 2
	rightLen := width - leftLen - labelLen
	if rightLen < 0 {
		rightLen = 0
	}
	return Styles.DetailDim.Render(strings.Repeat("─", leftLen)) +
		Styles.DetailLabel.Render(label) +
		Styles.DetailDim.Render(strings.Repeat("─", rightLen))
}

// renderMetadataGrid renders label: value rows for the detail panel.
func renderMetadataGrid(item *WorktreeItem, width int) string {
	const labelWidth = 10

	label := func(s string) string {
		return Styles.DetailLabel.Render(padRight(s, labelWidth))
	}

	var rows []string

	// Git section
	rows = append(rows, renderSectionHeader("Git", width))

	// Branch
	branchVal := Styles.DetailValue.Render(truncate(item.Branch, width-labelWidth-2))
	rows = append(rows, label("Branch")+branchVal)

	// Commit
	if item.Commit != "" {
		commitVal := Styles.DetailValue.Render(item.Commit)
		if item.CommitAge != "" {
			commitVal += Styles.DetailDim.Render(" · " + item.CommitAge)
		}
		rows = append(rows, label("Commit")+commitVal)
	}

	// Status section
	rows = append(rows, "")
	rows = append(rows, renderSectionHeader("Status", width))

	// Working status
	statusVal := renderStatusValue(item)
	rows = append(rows, label("Working")+statusVal)

	// Sync (ahead/behind/synced) — only show when remote is tracked
	if item.HasRemote {
		syncVal := renderSyncValue(item)
		rows = append(rows, label("Sync")+syncVal)
	}

	// Tmux (only if session exists)
	if item.TmuxStatus != "none" && item.TmuxStatus != "" {
		tmuxVal := renderTmuxValue(item)
		rows = append(rows, label("Tmux")+tmuxVal)
	}

	// Plugin statuses (containers, etc.)
	for _, s := range item.PluginStatuses {
		if s.Detail != "" {
			mainDetail, pointedDetail := splitContainerDetail(s.Detail)
			entry := s
			entry.Detail = mainDetail
			rows = append(rows, label("Docker")+renderContainerValue(&entry))
			if pointedDetail != "" {
				rows = append(rows, label("")+Styles.DetailDim.Render(pointedDetail))
			}
		}
	}

	return strings.Join(rows, "\n")
}

// renderStatusValue returns styled status text for the detail panel.
func renderStatusValue(item *WorktreeItem) string {
	switch {
	case item.IsPrunable:
		return Styles.StatusDanger.Render("✗ stale")
	case item.IsDirty:
		count := len(item.DirtyFiles)
		return Styles.StatusWarning.Render(fmt.Sprintf("● dirty (%d files)", count))
	default:
		return Styles.StatusSuccess.Render("● clean")
	}
}

// renderSyncValue returns styled sync status (ahead/behind/synced/no remote).
func renderSyncValue(item *WorktreeItem) string {
	if !item.HasRemote {
		return Styles.StatusWarning.Render("⚠ no remote")
	}
	if item.AheadCount == 0 && item.BehindCount == 0 {
		return Styles.StatusSuccess.Render("● synced")
	}
	var parts []string
	if item.AheadCount > 0 {
		parts = append(parts, Styles.StatusSuccess.Render(fmt.Sprintf("↑%d", item.AheadCount)))
	}
	if item.BehindCount > 0 {
		parts = append(parts, Styles.StatusDanger.Render(fmt.Sprintf("↓%d", item.BehindCount)))
	}
	return strings.Join(parts, " ")
}

// renderContainerValue returns styled container/plugin status for the detail panel.
func renderContainerValue(s *plugins.StatusEntry) string {
	switch s.Level {
	case plugins.StatusActive:
		return Styles.ContainerBadgeActive.Render("◆ " + s.Detail)
	case plugins.StatusWarning:
		return Styles.ContainerBadgeWarn.Render("◆ " + s.Detail)
	case plugins.StatusInfo:
		return Styles.ContainerBadge.Render("◇ " + s.Detail)
	default:
		return Styles.TextMuted.Render("◇ " + s.Detail)
	}
}

// splitContainerDetail splits a container detail string on ", pointed"
// into a main status line and a secondary "pointed" line.
func splitContainerDetail(detail string) (main, pointed string) {
	if idx := strings.Index(detail, ", pointed"); idx >= 0 {
		return detail[:idx], detail[idx+2:]
	}
	return detail, ""
}

// renderTmuxValue returns styled tmux session indicator.
func renderTmuxValue(item *WorktreeItem) string {
	switch item.TmuxStatus {
	case "attached":
		return Styles.TmuxBadgeActive.Render("⬢ active session")
	case "detached":
		return Styles.TmuxBadge.Render("⬡ detached session")
	default:
		return ""
	}
}

const maxChangesShown = 15

// fileEntry holds a parsed git status entry for tree rendering.
type fileEntry struct {
	prefix string // git status prefix (e.g. " M", "??", " D")
	dir    string // directory path (empty for root-level files)
	base   string // filename
}

// buildFileTree parses git status lines into directory-grouped entries.
func buildFileTree(files []string) []fileEntry {
	var entries []fileEntry
	for _, f := range files {
		f = strings.TrimSpace(f)
		if len(f) < 3 {
			continue
		}
		prefix := f[:2]
		filePath := strings.TrimSpace(f[2:])
		dir := path.Dir(filePath)
		base := path.Base(filePath)
		if dir == "." {
			dir = ""
		}
		entries = append(entries, fileEntry{prefix: prefix, dir: dir, base: base})
	}
	// Stable sort by directory to group files together
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].dir == entries[j].dir {
			return entries[i].base < entries[j].base
		}
		// Root-level files sort first
		if entries[i].dir == "" {
			return true
		}
		if entries[j].dir == "" {
			return false
		}
		return entries[i].dir < entries[j].dir
	})
	return entries
}

// renderChangesSection renders the changed files list grouped by directory.
func renderChangesSection(files []string, width int) string {
	header := Styles.DetailLabel.Render("── Changes ") +
		Styles.DetailDim.Render(strings.Repeat("─", max(0, width-12)))

	var lines []string
	lines = append(lines, header)

	entries := buildFileTree(files)

	overflow := 0
	if len(entries) > maxChangesShown {
		overflow = len(entries) - maxChangesShown
		entries = entries[:maxChangesShown]
	}

	lastDir := "\x00" // sentinel so first dir always triggers header
	for _, e := range entries {
		if e.dir != lastDir {
			lastDir = e.dir
			if e.dir != "" {
				dirDisplay := truncate(e.dir+"/", width-2)
				lines = append(lines, Styles.DetailDim.Render(dirDisplay))
			}
		}

		indent := " "
		nameWidth := width - 4
		if e.dir != "" {
			indent = "   "
			nameWidth = width - 6
		}
		baseName := truncate(e.base, nameWidth)

		var styled string
		switch {
		case strings.Contains(e.prefix, "?"):
			styled = Styles.DetailFileAdd.Render("+ " + baseName)
		case strings.Contains(e.prefix, "D"):
			styled = Styles.DetailFileDel.Render("- " + baseName)
		default:
			styled = Styles.DetailFileMod.Render("M " + baseName)
		}
		lines = append(lines, indent+styled)
	}

	if overflow > 0 {
		lines = append(lines, " "+Styles.StatusInfo.Render(fmt.Sprintf("… and %d more files", overflow)))
	}

	return strings.Join(lines, "\n")
}

// injectBorderTitle replaces the top border of a rounded-border box
// with the title inset after the corner character.
// It strips ANSI codes before rune-slicing to avoid cutting inside escape sequences.
func injectBorderTitle(rendered, title string) string {
	lines := strings.Split(rendered, "\n")
	if len(lines) == 0 {
		return rendered
	}

	// Strip ANSI from the top line so rune indexing is safe
	clean := ansiRegex.ReplaceAllString(lines[0], "")
	cleanRunes := []rune(clean)
	titleWidth := lipgloss.Width(title)

	if len(cleanRunes) <= titleWidth+3 {
		return rendered
	}

	// Re-apply the border color to the spliced segments
	borderColor := Styles.DetailBorder.GetBorderTopForeground()
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)

	newTop := borderStyle.Render(string(cleanRunes[:2])) + title +
		borderStyle.Render(string(cleanRunes[2+titleWidth:]))
	lines[0] = newTop

	return strings.Join(lines, "\n")
}

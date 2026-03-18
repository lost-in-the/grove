package tui

import (
	"fmt"
	"image/color"
	"path"
	"regexp"
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
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

// metadataLabel returns a styled, padded label for the metadata grid.
func metadataLabel(s string) string {
	const labelWidth = 10
	return Styles.DetailLabel.Render(padRight(s, labelWidth))
}

// renderGitSection renders the Git metadata rows (branch, commit, ahead, remote).
func renderGitSection(item *WorktreeItem, width int) []string {
	const labelWidth = 10
	rows := []string{renderSectionHeader("Git", width)}

	branchVal := Styles.DetailValue.Render(truncate(item.Branch, width-labelWidth-2))
	rows = append(rows, metadataLabel("Branch")+branchVal)

	if item.Commit != "" {
		commitVal := Styles.DetailValue.Render(item.Commit)
		if item.CommitAge != "" {
			commitVal += Styles.DetailDim.Render(" · " + item.CommitAge)
		}
		rows = append(rows, metadataLabel("Commit")+commitVal)
	}

	if item.CommitCount > 0 {
		rows = append(rows, metadataLabel("Ahead")+Styles.DetailValue.Render(fmt.Sprintf("%d commits", item.CommitCount)))
	}

	if item.TrackingBranch != "" {
		rows = append(rows, metadataLabel("Remote")+Styles.DetailValue.Render(truncate(item.TrackingBranch, width-labelWidth-2)))
	} else if !item.HasRemote && !item.IsMain {
		rows = append(rows, metadataLabel("Remote")+Styles.StatusWarning.Render("not tracking"))
	}

	return rows
}

// renderStatusSection renders the Status metadata rows (working, sync, stash, tmux, docker).
func renderStatusSection(item *WorktreeItem, width int) []string {
	rows := []string{"", renderSectionHeader("Status", width)}

	rows = append(rows, metadataLabel("Working")+renderStatusValue(item))

	if item.HasRemote {
		syncVal := renderSyncValue(item)
		if item.AheadCount > 0 {
			syncVal += Styles.DetailDim.Render(" unpushed")
		}
		rows = append(rows, metadataLabel("Sync")+syncVal)
	}

	if item.StashCount > 0 {
		rows = append(rows, metadataLabel("Stash")+Styles.StatusWarning.Render(fmt.Sprintf("%d stashed", item.StashCount)))
	}

	if item.TmuxStatus != "none" && item.TmuxStatus != "" {
		rows = append(rows, metadataLabel("Tmux")+renderTmuxValue(item))
	}

	for _, s := range item.PluginStatuses {
		if s.Detail != "" {
			mainDetail, pointedDetail := splitContainerDetail(s.Detail)
			entry := s
			entry.Detail = mainDetail
			rows = append(rows, metadataLabel("Docker")+renderContainerValue(&entry))
			if pointedDetail != "" {
				rows = append(rows, metadataLabel("")+Styles.DetailDim.Render(pointedDetail))
			}
		}
	}

	return rows
}

// renderRecentCommitsSection renders the Recent commits rows.
func renderRecentCommitsSection(item *WorktreeItem, width int) []string {
	if len(item.RecentCommits) == 0 {
		return nil
	}
	rows := []string{"", renderSectionHeader("Recent", width)}
	for _, c := range item.RecentCommits {
		msg := truncate(c.Message, width-12)
		rows = append(rows, "  "+Styles.DetailDim.Render(c.SHA)+" "+msg)
	}
	return rows
}

// renderAssociatedPRSection renders the Associated PR rows.
func renderAssociatedPRSection(item *WorktreeItem, width int) []string {
	if item.AssociatedPR == nil {
		return nil
	}
	pr := item.AssociatedPR
	rows := []string{"", renderSectionHeader("PR", width)}
	title := truncate(fmt.Sprintf("#%d %s", pr.Number, pr.Title), width-4)
	rows = append(rows, "  "+Styles.DetailValue.Render(title))

	if pr.ReviewDecision != "" {
		style := Styles.DetailDim
		switch pr.ReviewDecision {
		case reviewApproved:
			style = Styles.SuccessText
		case reviewChangesRequested:
			style = Styles.ErrorText
		}
		rows = append(rows, "  "+style.Render(formatReviewDecision(pr.ReviewDecision)))
	}
	return rows
}

// renderMetadataGrid renders label: value rows for the detail panel.
func renderMetadataGrid(item *WorktreeItem, width int) string {
	rows := make([]string, 0, 16)
	rows = append(rows, renderGitSection(item, width)...)
	rows = append(rows, renderStatusSection(item, width)...)
	rows = append(rows, renderRecentCommitsSection(item, width)...)
	rows = append(rows, renderAssociatedPRSection(item, width)...)
	return strings.Join(rows, "\n")
}

// formatReviewDecision returns a human-readable review status string.
func formatReviewDecision(decision string) string {
	switch decision {
	case reviewApproved:
		return "Approved"
	case reviewChangesRequested:
		return "Changes requested"
	case "REVIEW_REQUIRED":
		return "Review required"
	default:
		return decision
	}
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
	case tmuxStatusAttached:
		return Styles.TmuxBadgeActive.Render("⬢ active session")
	case tmuxStatusDetached:
		return Styles.TmuxBadge.Render("⬡ detached session")
	default:
		return ""
	}
}

// maxChangesShown is no longer limited — the detail panel is scrollable via
// Tab focus + j/k navigation, so all files are shown.

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
	var lines []string
	lines = append(lines, renderSectionHeader("Changes", width))

	entries := buildFileTree(files)

	lastDir := "\x00" // sentinel so first dir always triggers header
	for _, e := range entries {
		if e.dir != lastDir {
			lastDir = e.dir
			if e.dir != "" {
				dirDisplay := truncate(e.dir+"/", width-4)
				lines = append(lines, "  "+Styles.DetailDim.Render(dirDisplay))
			}
		}

		indent := "  "
		nameWidth := width - 6
		if e.dir != "" {
			indent = "    "
			nameWidth = width - 8
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

	return strings.Join(lines, "\n")
}

// injectBorderTitle replaces the top border of a rounded-border box
// with the title inset after the corner character.
// It strips ANSI codes before rune-slicing to avoid cutting inside escape sequences.
func injectBorderTitle(rendered, title string) string {
	return injectBorderTitleWithColor(rendered, title, Styles.DetailBorder.GetBorderTopForeground())
}

// injectBorderTitleWithColor is like injectBorderTitle but accepts a custom border color.
func injectBorderTitleWithColor(rendered, title string, borderColor color.Color) string {
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
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)

	newTop := borderStyle.Render(string(cleanRunes[:2])) + title +
		borderStyle.Render(string(cleanRunes[2+titleWidth:]))
	lines[0] = newTop

	return strings.Join(lines, "\n")
}

// detailViewportConfig holds the parameters for rendering a detail viewport card.
type detailViewportConfig struct {
	vp          *viewport.Model
	cursor      int
	lastCursor  *int
	focused     bool
	itemNumber  int
	contentFunc func(width int) string
	width       int
	height      int
}

// renderDetailViewportCard renders a bordered viewport card with title injection
// and focus-dependent styling. Shared by PR and Issue detail views.
func renderDetailViewportCard(cfg detailViewportConfig) string {
	vpWidth := cfg.width - 4
	if vpWidth < 16 {
		vpWidth = 16
	}
	vpHeight := cfg.height - 2
	if vpHeight < 1 {
		vpHeight = 1
	}
	cfg.vp.SetWidth(vpWidth)
	cfg.vp.SetHeight(vpHeight)

	if cfg.cursor != *cfg.lastCursor || cfg.vp.GetContent() == "" {
		content := cfg.contentFunc(cfg.width)
		cfg.vp.SetContent(content)
		cfg.vp.GotoTop()
		*cfg.lastCursor = cfg.cursor
	}

	borderStyle := Styles.DetailBorder
	if cfg.focused {
		borderStyle = Styles.DetailBorder.BorderForeground(Colors.Primary)
	}

	card := borderStyle.
		Width(cfg.width - 2).
		Render(cfg.vp.View())

	titleLabel := " " + Styles.DetailTitle.Render(fmt.Sprintf("#%d", cfg.itemNumber)) + " "
	if cfg.focused {
		card = injectBorderTitleWithColor(card, titleLabel, Colors.Primary)
	} else {
		card = injectBorderTitle(card, titleLabel)
	}

	return card
}

// handleDetailFocusedKey handles viewport scrolling keys when a detail panel is focused.
func handleDetailFocusedKey(msg tea.KeyPressMsg, keys KeyMap, vp *viewport.Model) {
	switch {
	case key.Matches(msg, keys.Up):
		vp.ScrollUp(1)
	case key.Matches(msg, keys.Down):
		vp.ScrollDown(1)
	case msg.String() == "g":
		vp.GotoTop()
	case msg.String() == "G":
		vp.GotoBottom()
	case msg.String() == "ctrl+u":
		vp.HalfPageUp()
	case msg.String() == "ctrl+d":
		vp.HalfPageDown()
	default:
	}
}

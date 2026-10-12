package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/lost-in-the/grove/internal/worktree"
)

// openDetail shows the selected worktree's detail: at 110+ columns it
// focuses the docked pane; below that it opens the full-screen sheet.
func (m *Model) openDetail() {
	if isWideLayout(m.width) {
		m.detailFocused = true
		m.refreshDetailForFocus()
		return
	}
	m.pushView(ViewDetailSheet)
	m.updateLayout()
	m.refreshDetailSheet()
}

// refreshDetailSheet renders the selected worktree into the detail
// viewport, from the top. The sheet is the whole screen, so it uses the
// unfocused card: there is nothing else to focus.
func (m *Model) refreshDetailSheet() {
	item, ok := m.selectedItem()
	if !ok {
		m.detail.SetContent("")
		return
	}
	m.detail.SetContent(renderDetailV2(&item, m.detail.Width()))
	m.detail.GotoTop()
}

func (m Model) handleDetailSheetKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Escape), key.Matches(msg, m.keys.Tab), msg.String() == "h":
		m.popView()
		m.updateLayout()
		m.updateDetailContent()
		return m, nil
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case msg.String() == "B":
		m.openSelectedPRURL()
		return m, nil
	}
	handleDetailFocusedKey(msg, m.keys, &m.detail)
	return m, nil
}

// renderDetailSheet draws the detail full screen: header, the detail card,
// and the scroll hints.
func (m Model) renderDetailSheet() string {
	statusBar := m.buildDashboardHeader().View(m.width)
	footer := m.helpFooter.RenderCompactWithHints(m.dashboardDetailFocusedHints(), m.width-4)
	body := lipgloss.NewStyle().Padding(0, 1).Render(m.detail.View())
	body = m.clampBodyToHeight(body, footer)
	return lipgloss.JoinVertical(lipgloss.Left, statusBar, body, footer)
}

// renderSelectionLine is the one-line summary of the selected worktree
// below 110 columns: alias, branch when it differs, and the commit subject.
func (m Model) renderSelectionLine(width int) string {
	item, ok := m.selectedItem()
	if !ok {
		return ""
	}
	parts := []string{item.ShortName}
	if item.Branch != "" && worktree.DeriveWorktreeName(item.Branch, "last_segment") != item.ShortName {
		parts = append(parts, "⎇ "+item.Branch)
	}
	if item.CommitMessage != "" {
		parts = append(parts, item.CommitMessage)
	}
	line := ansi.Truncate(strings.Join(parts, " · "), max(1, width), "…")
	return Styles.DetailDim.Render(line)
}

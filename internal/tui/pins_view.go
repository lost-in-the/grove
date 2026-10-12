package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/lost-in-the/grove/internal/tuilog"
)

// handlePinKey handles m and the digits (spec 8.1). m opens the pin strip;
// the next key pins (1-9), unpins (0), or closes it. A bare 1-9 switches to
// the worktree in that slot.
func (m Model) handlePinKey(msg tea.KeyPressMsg) (Model, tea.Cmd, bool) {
	k := msg.String()
	if m.pendingM {
		m.pendingM = false
		if len(k) == 1 && k[0] >= '0' && k[0] <= '9' {
			if item, ok := m.selectedItem(); ok {
				m.setPins(m.pins.pin(k, item.ShortName))
			}
		}
		m.updateLayout()
		if k == "ctrl+c" {
			return m, nil, false // still quits
		}
		return m, nil, true
	}
	switch {
	case k == "m":
		m.pendingM = true
		m.updateLayout()
		return m, nil, true
	case len(k) == 1 && k[0] >= '1' && k[0] <= '9':
		updated, cmd := m.switchToSlot(k)
		return updated, cmd, true
	}
	return m, nil, false
}

// switchToSlot switches to the worktree pinned to slot, or flashes that the
// slot is empty. Switching to the current worktree just quits.
func (m Model) switchToSlot(slot string) (Model, tea.Cmd) {
	alias := m.pins[slot]
	for _, li := range m.list.Items() {
		item, ok := li.(WorktreeItem)
		if !ok || alias == "" || item.ShortName != alias {
			continue
		}
		if !item.IsCurrent {
			m.switchTo = item.Path
			m.switchToDisplayName = item.displayName()
		}
		return m, tea.Quit
	}
	m.toast.Show(NewToast(fmt.Sprintf("slot %s is empty · m %s to pin", slot, slot), ToastInfo))
	return m, nil
}

// setPins replaces the pins, saves them, and redraws the pin column.
func (m *Model) setPins(p pinSet) {
	m.pins = p
	if err := updateUIPrefs(m.projectRoot, func(prefs *UIPrefs) { prefs.Pins = p }); err != nil {
		tuilog.Printf("warning: failed to save pins: %v", err)
	}
	m.refreshDelegate()
}

// pinStripLines is the m strip (spec 8.2): a title naming the selected
// worktree, then every slot with its alias, wrapped to the width.
func (m Model) pinStripLines() []string {
	width := max(20, m.width-2)
	selected := ""
	if item, ok := m.selectedItem(); ok {
		selected = " · " + item.ShortName + " is selected"
	}
	title := " " + Styles.HelpKey.Render("m") + Styles.HelpDesc.Render(" pin to slot"+selected)
	entry := func(key, desc string) string {
		return Styles.HelpKey.Render(key) + " " + Styles.HelpDesc.Render(desc)
	}
	var entries []string
	for d := '1'; d <= '9'; d++ {
		alias := m.pins[string(d)]
		if alias == "" {
			alias = "·"
		}
		entries = append(entries, entry(string(d), alias))
	}
	entries = append(entries, entry("0", "unpin"), entry("esc", "close"))

	lines := []string{title}
	line := " "
	for _, e := range entries {
		next := line + " " + e
		if line != " " {
			next = line + "   " + e
		}
		if lipgloss.Width(next) > width && line != " " {
			lines = append(lines, line)
			next = "  " + e
		}
		line = next
	}
	return append(lines, line)
}

func (m Model) renderPinStrip() string {
	return strings.Join(m.pinStripLines(), "\n")
}

// pinStripHeight is the strip's height while it is open, else 0.
func (m Model) pinStripHeight() int {
	if !m.pendingM {
		return 0
	}
	return len(m.pinStripLines())
}

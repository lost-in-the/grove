package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/lost-in-the/grove/internal/state"
	"github.com/lost-in-the/grove/internal/tuilog"
)

// handlePinStripKey takes the key after m (spec 8.1): a digit pins (1-9)
// or unpins (0) the selected worktree; any other key only closes the strip.
// ctrl+c still quits.
func (m Model) handlePinStripKey(msg tea.KeyPressMsg) (Model, tea.Cmd, bool) {
	k := msg.String()
	m.pendingM = false
	if len(k) == 1 && k[0] >= '0' && k[0] <= '9' {
		if item, ok := m.selectedItem(); ok {
			alias := item.ShortName
			m.changePins(func(p pinSet) pinSet { return p.pin(k, alias) })
		}
	}
	m.updateLayout()
	if k == "ctrl+c" {
		return m, nil, false
	}
	return m, nil, true
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

// changePins applies op to the pins saved on disk, not just this model's
// copy, so another dashboard on the same project keeps its pins. The model
// then adopts the saved result and redraws the pin column.
func (m *Model) changePins(op func(pinSet) pinSet) {
	next := op(m.pins)
	err := updateUIPrefs(m.projectRoot, func(prefs *UIPrefs) {
		next = op(pinSet(prefs.Pins).clone())
		prefs.Pins = next
	})
	if err != nil {
		tuilog.Printf("warning: failed to save pins: %v", err)
	}
	m.pins = next
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
	title = ansi.Truncate(title, width, "…")
	entry := func(key, desc string) string {
		return Styles.HelpKey.Render(key) + " " + Styles.HelpDesc.Render(desc)
	}
	var entries []string
	for d := '1'; d <= '9'; d++ {
		alias := middleTruncate(m.pins[string(d)], 20)
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

// switchToLastActive switches to the worktree used before this one, as
// grove last does. On the current worktree it just quits.
func (m Model) switchToLastActive() (Model, tea.Cmd) {
	for _, li := range m.list.Items() {
		item, ok := li.(WorktreeItem)
		if !ok || m.lastActive == "" || item.displayName() != m.lastActive {
			continue
		}
		if !item.IsCurrent {
			m.switchTo = item.Path
			m.switchToDisplayName = item.displayName()
		}
		return m, tea.Quit
	}
	m.toast.Show(NewToast("no previous worktree", ToastInfo))
	return m, nil
}

// lastWorktreeName reads state.LastWorktree, or "" when there is none.
func lastWorktreeName(stateMgr *state.Manager) string {
	if stateMgr == nil {
		return ""
	}
	name, err := stateMgr.GetLastWorktree()
	if err != nil {
		return ""
	}
	return name
}

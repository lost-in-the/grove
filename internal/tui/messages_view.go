package tui

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// openMessages shows the session's warnings and errors.
func (m Model) openMessages() (tea.Model, tea.Cmd) {
	m.messagesOffset = 0
	m.pushView(ViewMessages)
	return m, nil
}

func (m Model) handleMessagesKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch k := msg.String(); {
	case key.Matches(msg, m.keys.Escape), key.Matches(msg, m.keys.Quit):
		m.popView()
	case k == "down" || k == "j":
		m.messagesOffset = min(m.messagesOffset+1, max(0, len(m.toast.History())-1))
	case k == "up" || k == "k":
		m.messagesOffset = max(0, m.messagesOffset-1)
	}
	return m, nil
}

// renderMessages lists warnings and errors, newest first: age, level, text.
func (m Model) renderMessages() string {
	d := calcOverlayDims(m.width)
	inner := max(20, d.overlay-4)
	rows := max(3, min(m.height-8, 20))

	history := m.toast.History()
	var lines []string
	if len(history) == 0 {
		lines = append(lines, Styles.DetailDim.Render("no warnings or errors this session"))
	}
	now := m.now
	if now == nil {
		now = time.Now
	}
	for _, t := range history[min(m.messagesOffset, len(history)):] {
		age := shortAge(t.CreatedAt, now())
		icon := t.Level.style().Render(t.Level.Icon())
		text := ansi.Truncate(t.Message, inner-8, "…")
		lines = append(lines, Styles.DetailDim.Render(padCells(age, 4))+" "+icon+" "+text)
		if len(lines) == rows {
			break
		}
	}
	for len(lines) < rows {
		lines = append(lines, "")
	}
	footer := Styles.Footer.Render("↑↓ scroll · esc close")
	body := Styles.OverlayTitle.Render("Messages") + "\n\n" + strings.Join(lines, "\n") + "\n\n" + footer
	return Styles.OverlayBorder.Width(d.overlay).Render(body)
}

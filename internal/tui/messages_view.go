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
	// Each entry is "age(4) icon text": 7 cells before the text. The content
	// area is inner-2 wide (border and padding), and long text wraps under
	// itself so a whole message stays readable.
	const lead = 7
	textWidth := max(10, inner-2-lead)
entries:
	for _, t := range history[min(m.messagesOffset, len(history)):] {
		age := shortAge(t.CreatedAt, now())
		icon := t.Level.style().Render(t.Level.Icon())
		wrapped := strings.Split(ansi.Wrap(t.Message, textWidth, ""), "\n")
		for i, text := range wrapped {
			prefix := Styles.DetailDim.Render(padCells(age, 4)) + " " + icon + " "
			if i > 0 {
				prefix = strings.Repeat(" ", lead)
			}
			lines = append(lines, prefix+text)
			if len(lines) == rows {
				break entries
			}
		}
	}
	for len(lines) < rows {
		lines = append(lines, "")
	}
	footer := Styles.Footer.Render("↑↓ scroll · esc close")
	body := Styles.OverlayTitle.Render("Messages") + "\n\n" + strings.Join(lines, "\n") + "\n\n" + footer
	return Styles.OverlayBorder.Width(d.overlay).Render(body)
}

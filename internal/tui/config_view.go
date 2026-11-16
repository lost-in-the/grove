package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// configChromeRows are the config frame's rows besides the form and the
// footer: header, section tabs, a blank row, and the status line.
const configChromeRows = 4

// configSectionNames are the tab labels, in section order.
var configSectionNames = [ConfigTabCount]string{"general", "behavior", "plugins", "protection"}

func (m Model) configFooter() string {
	return m.helpFooter.RenderCompact(ViewConfig, m.width-4)
}

// configFormSize is the size every section's form is built at: the frame
// minus its chrome and the footer's real height, so all sections match and
// the frame fills the terminal exactly (spec 11).
func (m Model) configFormSize() (int, int) {
	footerH := lipgloss.Height(m.configFooter())
	return max(20, m.width-2), max(1, m.height-configChromeRows-footerH)
}

// renderConfigFrame draws the config editor as a full-screen frame.
func (m Model) renderConfigFrame() string {
	s := m.configState
	footer := m.configFooter()

	// Until the forms exist, a placeholder keeps the form's height; on a
	// load error it is blank, since the status line shows the error.
	placeholder := Styles.DetailDim.Render("Loading configuration…")
	if s.Err != nil {
		placeholder = ""
	}
	formBlock := placeholder + strings.Repeat("\n", max(0, s.formH-1))
	if s.form() != nil {
		formBlock = s.form().View()
	}
	body := strings.Join([]string{renderConfigTabs(s.Tab, m.width-2), "", formBlock, m.configStatus()}, "\n")
	body = lipgloss.NewStyle().Padding(0, 1).Render(body)
	body = m.clampBodyToHeight(body, footer)

	frame := lipgloss.JoinVertical(lipgloss.Left, m.configHeader(), body, footer)
	if s.Confirming {
		frame = composeOverlay(frame, renderConfigPrompt(), m.width, m.height)
	}
	return frame
}

// configHeader is "<project> ▸ settings", with "● n unsaved" on the right
// when anything changed.
func (m Model) configHeader() string {
	inner := max(0, m.width-2) // HeaderBar pads one cell each side
	left := Styles.Header.Render(m.projectName) + Styles.TextMuted.Render(" ▸ ") + Styles.TextBright.Render("settings")
	right := ""
	if n := m.configState.unsaved(); n > 0 {
		right = Styles.WarningText.Render(fmt.Sprintf("● %d unsaved", n))
	}
	left = ansi.Truncate(left, max(0, inner-lipgloss.Width(right)-1), "…")
	gap := max(1, inner-lipgloss.Width(left)-lipgloss.Width(right))
	content := ansi.Truncate(left+strings.Repeat(" ", gap)+right, inner, "")
	return headerBar(content, m.width)
}

// renderConfigTabs draws the section tabs with the active one raised.
func renderConfigTabs(active ConfigTab, width int) string {
	labels := make([]string, ConfigTabCount)
	for tab := range labels {
		labels[tab] = configSectionNames[tab]
	}
	return renderTabRow(labels, int(active), width)
}

// renderTabRow draws "[1] label  [2] label ..." with the active tab raised.
// When the full row does not fit, inactive tabs shrink to their number.
func renderTabRow(labels []string, active, width int) string {
	build := func(short bool) string {
		parts := make([]string, 0, len(labels))
		for i, l := range labels {
			label := fmt.Sprintf("[%d] %s", i+1, l)
			style := Styles.TabInactive
			if i == active {
				style = Styles.TabActive
			} else if short {
				label = fmt.Sprintf("[%d]", i+1)
			}
			parts = append(parts, style.Render(label))
		}
		return strings.Join(parts, " ")
	}
	row := build(false)
	if lipgloss.Width(row) > width {
		row = build(true)
	}
	return row
}

// configStatus is the frame's status line: a load or save error, else the
// active form's first validation error, else blank. huh shows neither
// (spec 11).
func (m Model) configStatus() string {
	s := m.configState
	if s.Err != nil {
		return Styles.ErrorText.Render("Error: " + s.Err.Error())
	}
	if f := s.form(); f != nil {
		if errs := f.Errors(); len(errs) > 0 {
			return Styles.ErrorText.Render(errs[0].Error())
		}
	}
	return ""
}

// renderConfigPrompt is the small save confirm drawn over the frame.
func renderConfigPrompt() string {
	keys := Styles.HelpKey.Render("⏎") + " " + Styles.HelpDesc.Render("save") +
		Styles.HelpSep.Render(" · ") +
		Styles.HelpKey.Render("esc") + " " + Styles.HelpDesc.Render("discard")
	return Styles.OverlayBorder.Render(Styles.WarningText.Render("Save changes?") + "\n\n" + keys)
}

// resize refits every section's form, for a terminal resize.
func (s *ConfigState) resize(width, height int) {
	s.formW, s.formH = width, height
	for _, f := range s.Forms {
		if f != nil {
			f.WithWidth(width).WithHeight(height)
		}
	}
}

package tui

import "strings"

func renderHelp(width, height int) string {
	cols := []struct {
		header string
		items  [][2]string
	}{
		{
			header: "Navigation",
			items: [][2]string{
				{"j/k ↑/↓", "move"},
				{"enter", "switch to worktree"},
				{"esc", "back / close"},
			},
		},
		{
			header: "Actions",
			items: [][2]string{
				{"n", "new worktree"},
				{"d", "delete worktree"},
				{"r", "refresh list"},
			},
		},
		{
			header: "Views",
			items: [][2]string{
				{"/", "filter / search"},
				{"?", "this help"},
				{"q", "quit"},
			},
		},
	}

	var sections []string
	for _, col := range cols {
		var lines []string
		lines = append(lines, Theme.DetailTitle.Render(col.header))
		for _, item := range col.items {
			key := Theme.HelpKey.Render(padRight(item[0], 12))
			desc := Theme.HelpDesc.Render(item[1])
			lines = append(lines, "  "+key+desc)
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}

	body := strings.Join(sections, "\n\n")
	body += "\n\n" + Theme.DetailDim.Render("Guided flows: follow on-screen prompts.")
	body += "\n" + Theme.DetailDim.Render("Backspace goes back. Esc cancels.")
	body += "\n\n" + Theme.Footer.Render("[any key to close]")

	return Theme.OverlayBorder.Render(
		Theme.OverlayTitle.Render("Keybindings") + "\n\n" + body,
	)
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

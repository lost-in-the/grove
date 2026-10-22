package tui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/lost-in-the/grove/internal/git"
)

// foldRun is the shortest run of removed lines that z folds (plan 8b
// decision 10).
const foldRun = 4

// safeText makes diff text safe to draw: the PR's author controls it, and
// gh hands it over verbatim. C0 controls become Unicode control pictures
// (ESC → ␛), DEL becomes ␡, C1 controls (U+009B is a one-rune CSI) are
// spelled out as \xNN, tabs expand to 4-column stops, and a trailing \r
// from a CRLF file is dropped.
func safeText(s string) string {
	s = strings.TrimSuffix(s, "\r")
	var b strings.Builder
	col := 0
	for _, r := range s {
		switch {
		case r == '\t':
			n := 4 - col%4
			b.WriteString(strings.Repeat(" ", n))
			col += n
			continue
		case r < 0x20:
			r = 0x2400 + r
		case r == 0x7f:
			r = '␡'
		case r >= 0x80 && r <= 0x9f:
			fmt.Fprintf(&b, `\x%02x`, r)
			col += 4
			continue
		}
		b.WriteRune(r)
		col += ansi.StringWidth(string(r))
	}
	return b.String()
}

// renderFileDiff draws one file's hunks: each @@ header, then every line as
// "OLD NEW │±text" with the numbers right-aligned, colored by kind, cut to
// width. folded collapses each run of foldRun or more removed lines.
func renderFileDiff(f git.FileDiff, width int, folded bool) []string {
	switch {
	case f.Binary:
		return []string{Styles.TextMuted.Render("binary file")}
	case len(f.Hunks) == 0 && f.Status == git.FileRenamed:
		return []string{Styles.TextMuted.Render("renamed from " + safeText(f.OldPath))}
	case len(f.Hunks) == 0:
		return []string{Styles.TextMuted.Render("no content change")}
	}
	numW := 1
	for _, h := range f.Hunks {
		numW = max(numW, len(strconv.Itoa(h.OldStart+h.OldLines)), len(strconv.Itoa(h.NewStart+h.NewLines)))
	}
	num := func(n int) string {
		if n == 0 {
			return strings.Repeat(" ", numW)
		}
		return fmt.Sprintf("%*d", numW, n)
	}
	cut := func(s string) string { return ansi.Truncate(s, width, "…") }

	var out []string
	for _, h := range f.Hunks {
		out = append(out, cut(lipgloss.NewStyle().Foreground(Colors.Info).Render(safeText(h.Header))))
		for i := 0; i < len(h.Lines); i++ {
			l := h.Lines[i]
			if folded && l.Kind == git.LineRemoved {
				run := 0
				for i+run < len(h.Lines) && h.Lines[i+run].Kind == git.LineRemoved {
					run++
				}
				if run >= foldRun {
					out = append(out, cut(Styles.TextMuted.Render(fmt.Sprintf("%s %s │ … %d removed lines", num(0), num(0), run))))
					i += run - 1
					continue
				}
			}
			mark, style := " ", Styles.TextNormal
			switch l.Kind {
			case git.LineAdded:
				mark, style = "+", Styles.SuccessText
			case git.LineRemoved:
				mark, style = "-", Styles.ErrorText
			case git.LineNoNewline:
				mark, style = `\`, Styles.TextMuted
			}
			gutter := Styles.TextMuted.Render(num(l.OldNo) + " " + num(l.NewNo) + " │")
			out = append(out, cut(gutter+style.Render(mark+safeText(l.Text))))
		}
	}
	return out
}

// renderFileHeader is the current file's title line in the hunk pane.
func renderFileHeader(f git.FileDiff, width int) string {
	stats := Styles.SuccessText.Render(fmt.Sprintf("+%d", f.Added)) + " " + Styles.ErrorText.Render(fmt.Sprintf("-%d", f.Removed))
	return ansi.Truncate(Styles.TextBright.Render(safeText(f.Path()))+"  "+stats, width, "…") + "\n" +
		lipgloss.NewStyle().Foreground(Colors.SurfaceBorder).Render(strings.Repeat("─", max(0, width)))
}

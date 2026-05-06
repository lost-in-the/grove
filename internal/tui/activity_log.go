package tui

import (
	"strings"
)

// ActivityLog accumulates log lines and renders them in a scrollable view.
// It is used to show real-time creation progress instead of a static spinner.
type ActivityLog struct {
	lines    []string
	maxLines int // how many lines to show (tail behavior)
	width    int
	done     bool
	err      error
}

// NewActivityLog creates an ActivityLog with the given display constraints.
func NewActivityLog(width, maxLines int) *ActivityLog {
	if maxLines < 3 {
		maxLines = 3
	}
	return &ActivityLog{
		width:    width,
		maxLines: maxLines,
	}
}

// AddLine appends a log line.
func (a *ActivityLog) AddLine(line string) {
	a.lines = append(a.lines, line)
}

// SetDone marks the log as complete, optionally with an error.
func (a *ActivityLog) SetDone(err error) {
	a.done = true
	a.err = err
}

// View renders the activity log lines with tail behavior.
func (a *ActivityLog) View(spinnerView string) string {
	if len(a.lines) == 0 && !a.done {
		return spinnerView + " Starting..."
	}

	var b strings.Builder

	// Determine which lines to show (tail)
	start := 0
	if len(a.lines) > a.maxLines {
		start = len(a.lines) - a.maxLines
	}

	for i := start; i < len(a.lines); i++ {
		line := a.lines[i]
		isLast := i == len(a.lines)-1

		if isLast && !a.done {
			// Last in-progress line gets the spinner
			b.WriteString(spinnerView + " " + line + "\n")
		} else {
			// Completed lines get a dim bullet
			b.WriteString(Styles.DetailDim.Render("  "+line) + "\n")
		}
	}

	if a.done {
		if a.err != nil {
			b.WriteString(Styles.ErrorText.Render("  "+SymbolError+" Failed: "+a.err.Error()) + "\n")
		} else {
			b.WriteString(Styles.SuccessText.Render("  "+SymbolSuccess+" Done") + "\n")
		}
	}

	return b.String()
}

// IsDone returns whether the activity log is complete.
func (a *ActivityLog) IsDone() bool {
	return a.done
}

// Symbols for activity log status lines.
const (
	SymbolSuccess = "\u2713" // check mark
	SymbolError   = "\u2717" // ballot x
)

// renderCreatingDetail renders an activity log for the detail pane in panel
// layouts (PR view, issue view). Falls back to spinner + fallbackMsg if no log.
func renderCreatingDetail(log *ActivityLog, spinnerView, fallbackMsg string) string {
	if log != nil {
		return log.View(spinnerView)
	}
	return spinnerView + " " + fallbackMsg
}

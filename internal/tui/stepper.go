package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Stepper is a visual progress indicator for multi-step wizards.
// It renders a connected dot-line showing completed, current, and future steps.
type Stepper struct {
	Steps   []string
	Current int
}

// NewStepper creates a Stepper with the given step labels.
func NewStepper(steps ...string) *Stepper {
	return &Stepper{
		Steps:   steps,
		Current: 0,
	}
}

// Advance moves to the next step, clamping at the last step.
func (s *Stepper) Advance() {
	if s.Current < len(s.Steps)-1 {
		s.Current++
	}
}

// Back moves to the previous step, clamping at 0.
func (s *Stepper) Back() {
	if s.Current > 0 {
		s.Current--
	}
}

// IsComplete returns true if the given step index has been completed
// (i.e., the current step is past it).
func (s *Stepper) IsComplete(idx int) bool {
	return idx < s.Current
}

// IsCurrent returns true if the given step index is the current step.
func (s *Stepper) IsCurrent(idx int) bool {
	return idx == s.Current
}

// View renders the stepper as a horizontal progress bar with labels.
//
//	●━━━━━━●━━━━━━○
//	Name   Branch  Create
//
// Completed steps are green ●, current is purple ●, future is gray ○.
// Connectors between completed steps are green ━, others gray ━.
func (s *Stepper) View(width int) string {
	if len(s.Steps) == 0 {
		return ""
	}

	completeDot := lipgloss.NewStyle().Foreground(Colors.Success).Render("●")
	currentDot := lipgloss.NewStyle().Foreground(Colors.Primary).Render("●")
	futureDot := lipgloss.NewStyle().Foreground(Colors.TextMuted).Render("○")

	completeConnStyle := lipgloss.NewStyle().Foreground(Colors.Success)
	futureConnStyle := lipgloss.NewStyle().Foreground(Colors.TextMuted)

	completeLabel := lipgloss.NewStyle().Foreground(Colors.Success)
	currentLabel := lipgloss.NewStyle().Foreground(Colors.Primary).Bold(true)
	futureLabel := lipgloss.NewStyle().Foreground(Colors.TextMuted)

	// Calculate connector width between dots
	n := len(s.Steps)
	// Reserve space for dots (1 char each) and minimum padding
	connWidth := 6
	if n > 1 {
		available := width - n // dots
		perGap := available / (n - 1)
		perGap = min(perGap, 24)
		perGap = max(perGap, 3)
		connWidth = perGap
	}

	// Build dot line
	var dotLine strings.Builder
	for i := range n {
		// Dot
		switch {
		case s.IsComplete(i):
			dotLine.WriteString(completeDot)
		case s.IsCurrent(i):
			dotLine.WriteString(currentDot)
		default:
			dotLine.WriteString(futureDot)
		}

		// Connector (not after last dot)
		if i < n-1 {
			connStyle := futureConnStyle
			if s.IsComplete(i) {
				connStyle = completeConnStyle
			}
			dotLine.WriteString(connStyle.Render(strings.Repeat("━", connWidth)))
		}
	}

	// Build label line — each step gets an equal-width column, label centered within it.
	// Total dot line visible width: (n-1)*connWidth + n = n-1 segments + n dots.
	totalWidth := n + (n-1)*connWidth

	var labelLine strings.Builder
	labelPos := 0
	for i, step := range s.Steps {
		var styled string
		switch {
		case s.IsComplete(i):
			styled = completeLabel.Render(step + " ✓")
		case s.IsCurrent(i):
			styled = currentLabel.Render(step)
		default:
			styled = futureLabel.Render(step)
		}

		styledWidth := lipgloss.Width(styled)

		// Center of this step's column
		var colStart int
		switch i {
		case 0:
			colStart = 0
		case n - 1:
			// Last label: right-align so it doesn't overshoot
			colStart = totalWidth - styledWidth
		default:
			// Center under the dot at position i*(connWidth+1)
			dotPos := i * (connWidth + 1)
			colStart = dotPos - styledWidth/2
		}

		// Clamp: don't overlap previous label, don't go negative
		if colStart < labelPos {
			colStart = labelPos
		}

		// Don't exceed total width
		if colStart+styledWidth > totalWidth {
			colStart = totalWidth - styledWidth
			if colStart < labelPos {
				colStart = labelPos
			}
		}

		if colStart > labelPos {
			labelLine.WriteString(strings.Repeat(" ", colStart-labelPos))
		}
		labelLine.WriteString(styled)
		labelPos = colStart + styledWidth
	}

	return lipgloss.JoinVertical(lipgloss.Left, dotLine.String(), labelLine.String())
}

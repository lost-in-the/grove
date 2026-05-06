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

// stepDot returns the styled dot character for a step at the given index.
func (s *Stepper) stepDot(idx int) string {
	switch {
	case s.IsComplete(idx):
		return lipgloss.NewStyle().Foreground(Colors.Success).Render("●")
	case s.IsCurrent(idx):
		return lipgloss.NewStyle().Foreground(Colors.Primary).Render("●")
	default:
		return lipgloss.NewStyle().Foreground(Colors.TextMuted).Render("○")
	}
}

// stepLabel returns the styled label string for a step at the given index.
func (s *Stepper) stepLabel(idx int) string {
	switch {
	case s.IsComplete(idx):
		return lipgloss.NewStyle().Foreground(Colors.Success).Render(s.Steps[idx] + " ✓")
	case s.IsCurrent(idx):
		return lipgloss.NewStyle().Foreground(Colors.Primary).Bold(true).Render(s.Steps[idx])
	default:
		return lipgloss.NewStyle().Foreground(Colors.TextMuted).Render(s.Steps[idx])
	}
}

// connectorWidth calculates the width of connectors between dots.
func (s *Stepper) connectorWidth(width int) int {
	n := len(s.Steps)
	if n <= 1 {
		return 6
	}
	available := width - n
	perGap := available / (n - 1)
	perGap = min(perGap, 24)
	perGap = max(perGap, 3)
	return perGap
}

// buildDotLine renders the dot-and-connector progress line.
func (s *Stepper) buildDotLine(connWidth int) string {
	n := len(s.Steps)
	completeConnStyle := lipgloss.NewStyle().Foreground(Colors.Success)
	futureConnStyle := lipgloss.NewStyle().Foreground(Colors.TextMuted)

	var dotLine strings.Builder
	for i := range n {
		dotLine.WriteString(s.stepDot(i))
		if i < n-1 {
			connStyle := futureConnStyle
			if s.IsComplete(i) {
				connStyle = completeConnStyle
			}
			dotLine.WriteString(connStyle.Render(strings.Repeat("━", connWidth)))
		}
	}
	return dotLine.String()
}

// labelColStart calculates the column start position for a label.
func labelColStart(idx, n, connWidth, totalWidth, styledWidth, labelPos int) int {
	var colStart int
	switch idx {
	case 0:
		colStart = 0
	case n - 1:
		colStart = totalWidth - styledWidth
	default:
		dotPos := idx * (connWidth + 1)
		colStart = dotPos - styledWidth/2
	}

	if colStart < labelPos {
		colStart = labelPos
	}

	if colStart+styledWidth > totalWidth {
		colStart = totalWidth - styledWidth
		if colStart < labelPos {
			colStart = labelPos
		}
	}
	return colStart
}

// buildLabelLine renders the centered labels beneath the dot line.
func (s *Stepper) buildLabelLine(connWidth int) string {
	n := len(s.Steps)
	totalWidth := n + (n-1)*connWidth

	var labelLine strings.Builder
	labelPos := 0
	for i := range s.Steps {
		styled := s.stepLabel(i)
		styledWidth := lipgloss.Width(styled)
		colStart := labelColStart(i, n, connWidth, totalWidth, styledWidth, labelPos)

		if colStart > labelPos {
			labelLine.WriteString(strings.Repeat(" ", colStart-labelPos))
		}
		labelLine.WriteString(styled)
		labelPos = colStart + styledWidth
	}
	return labelLine.String()
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

	connWidth := s.connectorWidth(width)
	dotLine := s.buildDotLine(connWidth)
	labelLine := s.buildLabelLine(connWidth)

	return lipgloss.JoinVertical(lipgloss.Left, dotLine, labelLine)
}

package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
)

// GroveSpinner returns a spinner configured with smooth animation and brand colors.
func GroveSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Spinner{
		Frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		FPS:    80 * time.Millisecond,
	}
	s.Style = lipgloss.NewStyle().Foreground(Colors.Primary)
	return s
}

// ToastOpacity returns a lipgloss.Style modifier based on how close the toast
// is to expiry. Returns full opacity for most of the lifetime, then fades in
// the final 800ms.
func ToastOpacity(t *Toast) lipgloss.Style {
	if t == nil {
		return lipgloss.NewStyle()
	}
	elapsed := time.Since(t.CreatedAt)
	remaining := t.Duration - elapsed
	fadeWindow := 800 * time.Millisecond

	if remaining <= 0 {
		return lipgloss.NewStyle().Faint(true)
	}
	if remaining < fadeWindow {
		return lipgloss.NewStyle().Faint(true)
	}
	return lipgloss.NewStyle()
}

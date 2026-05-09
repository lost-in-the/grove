package tui

import (
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/lipgloss/v2"
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

// Pre-built styles for ToastOpacity. Allocating a new lipgloss.Style on every
// render frame (toasts can be re-rendered on every tick) is wasted work — the
// only two states are "full" and "faint".
var (
	toastStyleFull  = lipgloss.NewStyle()
	toastStyleFaint = lipgloss.NewStyle().Faint(true)
)

// ToastOpacity returns a lipgloss.Style modifier based on how close the toast
// is to expiry. Returns full opacity for most of the lifetime, then fades in
// the final 800ms.
func ToastOpacity(t *Toast) lipgloss.Style {
	if t == nil {
		return toastStyleFull
	}
	elapsed := time.Since(t.CreatedAt)
	remaining := t.Duration - elapsed
	const fadeWindow = 800 * time.Millisecond

	if remaining < fadeWindow {
		return toastStyleFaint
	}
	return toastStyleFull
}

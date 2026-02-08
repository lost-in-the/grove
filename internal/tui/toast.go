package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// DefaultToastDuration is how long toasts display before auto-expiry.
const DefaultToastDuration = 3 * time.Second

// ToastLevel represents the severity of a toast notification.
type ToastLevel int

const (
	ToastSuccess ToastLevel = iota
	ToastWarning
	ToastError
	ToastInfo
)

// Icon returns the display icon for a toast level.
func (l ToastLevel) Icon() string {
	switch l {
	case ToastSuccess:
		return "✓"
	case ToastWarning:
		return "⚠"
	case ToastError:
		return "✗"
	case ToastInfo:
		return "ℹ"
	default:
		return "•"
	}
}

// borderColor returns the border color for a toast level.
func (l ToastLevel) borderColor() lipgloss.AdaptiveColor {
	switch l {
	case ToastSuccess:
		return Colors.Success
	case ToastWarning:
		return Colors.Warning
	case ToastError:
		return Colors.Danger
	case ToastInfo:
		return Colors.Info
	default:
		return Colors.SurfaceBorder
	}
}

// style returns the lipgloss style for a toast level.
func (l ToastLevel) style() lipgloss.Style {
	switch l {
	case ToastSuccess:
		return Styles.StatusSuccess
	case ToastWarning:
		return Styles.StatusWarning
	case ToastError:
		return Styles.StatusDanger
	case ToastInfo:
		return Styles.StatusInfo
	default:
		return Styles.TextNormal
	}
}

// Toast represents a single notification.
type Toast struct {
	Message   string
	Level     ToastLevel
	Duration  time.Duration
	CreatedAt time.Time
}

// NewToast creates a toast with the default duration.
func NewToast(message string, level ToastLevel) *Toast {
	return &Toast{
		Message:   message,
		Level:     level,
		Duration:  DefaultToastDuration,
		CreatedAt: time.Now(),
	}
}

// NewToastWithDuration creates a toast with a custom duration.
func NewToastWithDuration(message string, level ToastLevel, duration time.Duration) *Toast {
	return &Toast{
		Message:   message,
		Level:     level,
		Duration:  duration,
		CreatedAt: time.Now(),
	}
}

// Expired returns true if the toast has exceeded its duration.
func (t *Toast) Expired() bool {
	return time.Since(t.CreatedAt) >= t.Duration
}

// ToastModel manages the currently visible toast.
type ToastModel struct {
	Current *Toast
}

// NewToastModel creates an empty ToastModel.
func NewToastModel() *ToastModel {
	return &ToastModel{}
}

// Show displays a new toast, replacing any existing one.
func (tm *ToastModel) Show(toast *Toast) {
	tm.Current = toast
}

// Tick checks if the current toast has expired and clears it.
func (tm *ToastModel) Tick() {
	if tm.Current != nil && tm.Current.Expired() {
		tm.Current = nil
	}
}

// Dismiss immediately clears the current toast.
func (tm *ToastModel) Dismiss() {
	tm.Current = nil
}

// View renders the toast right-aligned within the given width.
// Returns empty string if no toast is active.
func (tm *ToastModel) View(width int) string {
	if tm.Current == nil {
		return ""
	}

	t := tm.Current
	icon := t.Level.style().Render(t.Level.Icon())
	content := fmt.Sprintf("%s %s", icon, t.Message)

	// Apply fade-out effect as toast nears expiry
	fadeStyle := ToastOpacity(t)

	box := fadeStyle.
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Level.borderColor()).
		Padding(0, 1).
		Render(content)

	boxWidth := lipgloss.Width(box)
	padding := width - boxWidth
	padding = max(padding, 0)

	return strings.Repeat(" ", padding) + box
}

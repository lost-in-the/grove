package cli

import (
	"fmt"
	"image/color"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/lost-in-the/grove/internal/theme"
)

// printWithIcon is a shared helper for the icon-prefixed print functions.
func printWithIcon(w *Writer, color color.Color, icon, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if w.UseColor() {
		styled := lipgloss.NewStyle().Foreground(color).Render(icon)
		_, _ = fmt.Fprintf(w, "%s %s\n", styled, msg)
	} else {
		_, _ = fmt.Fprintf(w, "%s %s\n", icon, msg)
	}
}

// Success prints a success message with a green checkmark.
func Success(w *Writer, format string, args ...any) {
	printWithIcon(w, theme.Colors.Success, "✓", format, args...)
}

// Warning prints a warning message with a yellow warning icon.
func Warning(w *Writer, format string, args ...any) {
	printWithIcon(w, theme.Colors.Warning, "⚠", format, args...)
}

// Error prints an error message with a red cross icon.
func Error(w *Writer, format string, args ...any) {
	printWithIcon(w, theme.Colors.Danger, "✗", format, args...)
}

// Info prints an informational message with a blue info icon.
func Info(w *Writer, format string, args ...any) {
	printWithIcon(w, theme.Colors.Info, "ℹ", format, args...)
}

// Header prints a styled header line with a separator.
func Header(w *Writer, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if w.UseColor() {
		title := lipgloss.NewStyle().Bold(true).Foreground(theme.Colors.TextBright).Render(msg)
		_, _ = fmt.Fprintln(w, title)
		sep := lipgloss.NewStyle().Foreground(theme.Colors.SurfaceDim).Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		_, _ = fmt.Fprintln(w, sep)
	} else {
		_, _ = fmt.Fprintln(w, msg)
		_, _ = fmt.Fprintln(w, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	}
}

// Step prints a step indicator for multi-step operations.
func Step(w *Writer, format string, args ...any) {
	printWithIcon(w, theme.Colors.Primary, "→", format, args...)
}

// Faint prints dim/muted text.
func Faint(w *Writer, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if w.UseColor() {
		_, _ = fmt.Fprintln(w, lipgloss.NewStyle().Foreground(theme.Colors.TextMuted).Render(msg))
	} else {
		_, _ = fmt.Fprintln(w, msg)
	}
}

// Bold prints bold text.
func Bold(w *Writer, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if w.UseColor() {
		_, _ = fmt.Fprintln(w, lipgloss.NewStyle().Bold(true).Foreground(theme.Colors.TextBright).Render(msg))
	} else {
		_, _ = fmt.Fprintln(w, msg)
	}
}

// Label prints a label: value pair with the label styled.
func Label(w *Writer, label, value string) {
	if w.UseColor() {
		l := lipgloss.NewStyle().Foreground(theme.Colors.TextMuted).Render(label)
		_, _ = fmt.Fprintf(w, "%s %s\n", l, value)
	} else {
		_, _ = fmt.Fprintf(w, "%s %s\n", label, value)
	}
}

// StatusLevel represents a semantic status for colored output.
type StatusLevel string

const (
	StatusClean    StatusLevel = "clean"
	StatusOK       StatusLevel = "ok"
	StatusActive   StatusLevel = "active"
	StatusAttached StatusLevel = "attached"
	StatusDirty    StatusLevel = "dirty"
	StatusWarning  StatusLevel = "warning"
	StatusDetached StatusLevel = "detached"
	StatusStale    StatusLevel = "stale"
	StatusError    StatusLevel = "error"
	StatusFail     StatusLevel = "fail"
	StatusInfo     StatusLevel = "info"
	StatusNone     StatusLevel = "none"
)

// StatusText returns styled text colored by status level.
func StatusText(w *Writer, status StatusLevel, text string) string {
	if !w.UseColor() {
		return text
	}
	switch status {
	case StatusClean, StatusOK, StatusActive, StatusAttached:
		return lipgloss.NewStyle().Foreground(theme.Colors.Success).Render(text)
	case StatusDirty, StatusWarning, StatusDetached:
		return lipgloss.NewStyle().Foreground(theme.Colors.Warning).Render(text)
	case StatusStale, StatusError, StatusFail:
		return lipgloss.NewStyle().Foreground(theme.Colors.Danger).Render(text)
	case StatusInfo, StatusNone:
		return lipgloss.NewStyle().Foreground(theme.Colors.TextMuted).Render(text)
	default:
		return text
	}
}

// Accent returns text styled with the primary accent color.
func Accent(w *Writer, text string) string {
	if !w.UseColor() {
		return text
	}
	return lipgloss.NewStyle().Foreground(theme.Colors.Primary).Render(text)
}

package tui

import (
	"github.com/lost-in-the/grove/internal/theme"
)

// RelativeLuminance delegates to the shared theme package.
var RelativeLuminance = theme.RelativeLuminance

// ContrastRatio delegates to the shared theme package.
var ContrastRatio = theme.ContrastRatio

// isHighContrast delegates to the shared theme package.
func isHighContrast() bool {
	return theme.IsHighContrast()
}

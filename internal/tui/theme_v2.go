package tui

import (
	"os"

	"github.com/charmbracelet/lipgloss"
)

// ColorScheme defines semantic colors using AdaptiveColor for automatic
// dark/light terminal adaptation.
type ColorScheme struct {
	// Brand
	Primary   lipgloss.AdaptiveColor
	Secondary lipgloss.AdaptiveColor

	// Status
	Success lipgloss.AdaptiveColor
	Warning lipgloss.AdaptiveColor
	Danger  lipgloss.AdaptiveColor
	Info    lipgloss.AdaptiveColor

	// Surface
	SurfaceBg     lipgloss.AdaptiveColor
	SurfaceFg     lipgloss.AdaptiveColor
	SurfaceDim    lipgloss.AdaptiveColor
	SurfaceBorder lipgloss.AdaptiveColor

	// Text
	TextNormal lipgloss.AdaptiveColor
	TextBright lipgloss.AdaptiveColor
	TextMuted  lipgloss.AdaptiveColor
}

// Colors is the global color scheme. Initialized respecting NO_COLOR.
var Colors = NewColorScheme()

// defaultColorScheme returns the full color palette.
func defaultColorScheme() ColorScheme {
	return ColorScheme{
		// Brand — purple/blue inspired by lazygit/charm aesthetics
		Primary:   lipgloss.AdaptiveColor{Dark: "#A78BFA", Light: "#7C3AED"},
		Secondary: lipgloss.AdaptiveColor{Dark: "#38BDF8", Light: "#0284C7"},

		// Status — Tailwind-inspired semantic colors
		Success: lipgloss.AdaptiveColor{Dark: "#34D399", Light: "#059669"},
		Warning: lipgloss.AdaptiveColor{Dark: "#FBBF24", Light: "#D97706"},
		Danger:  lipgloss.AdaptiveColor{Dark: "#F87171", Light: "#DC2626"},
		Info:    lipgloss.AdaptiveColor{Dark: "#60A5FA", Light: "#2563EB"},

		// Surface — Catppuccin Mocha (dark) / Slate (light)
		SurfaceBg:     lipgloss.AdaptiveColor{Dark: "#1E1E2E", Light: "#FFFFFF"},
		SurfaceFg:     lipgloss.AdaptiveColor{Dark: "#CDD6F4", Light: "#1E293B"},
		SurfaceDim:    lipgloss.AdaptiveColor{Dark: "#585B70", Light: "#94A3B8"},
		SurfaceBorder: lipgloss.AdaptiveColor{Dark: "#45475A", Light: "#CBD5E1"},

		// Text
		TextNormal: lipgloss.AdaptiveColor{Dark: "#CDD6F4", Light: "#1E293B"},
		TextBright: lipgloss.AdaptiveColor{Dark: "#FFFFFF", Light: "#0F172A"},
		TextMuted:  lipgloss.AdaptiveColor{Dark: "#6C7086", Light: "#64748B"},
	}
}

// noColorScheme returns a ColorScheme with all empty colors for NO_COLOR mode.
func noColorScheme() ColorScheme {
	return ColorScheme{}
}

// NewColorScheme creates a ColorScheme, respecting NO_COLOR and GROVE_NO_COLOR.
func NewColorScheme() ColorScheme {
	if isNoColor() {
		return noColorScheme()
	}
	return defaultColorScheme()
}

// isNoColor checks if color output should be suppressed.
func isNoColor() bool {
	_, nc := os.LookupEnv("NO_COLOR")
	_, gnc := os.LookupEnv("GROVE_NO_COLOR")
	return nc || gnc
}

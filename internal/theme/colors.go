package theme

import (
	"image/color"
	"os"
	"strconv"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
)

// ColorScheme defines semantic colors for dark/light terminal adaptation.
type ColorScheme struct {
	// Brand
	Primary   color.Color
	Secondary color.Color

	// Status
	Success color.Color
	Warning color.Color
	Danger  color.Color
	Info    color.Color

	// Surface
	SurfaceBg     color.Color
	SurfaceFg     color.Color
	SurfaceDim    color.Color
	SurfaceBorder color.Color

	// Selection / Header
	SelectionBg color.Color
	HeaderBg    color.Color

	// Text
	TextNormal color.Color
	TextBright color.Color
	TextMuted  color.Color
	TextDim    color.Color
}

// Colors is the global color scheme. Initialized respecting NO_COLOR.
var Colors = NewColorScheme()

// DefaultColorScheme returns the full color palette.
func DefaultColorScheme() ColorScheme {
	return ColorScheme{
		// Brand — purple/blue inspired by lazygit/charm aesthetics
		Primary:   AdaptiveColor("#A78BFA", "#7C3AED"),
		Secondary: AdaptiveColor("#38BDF8", "#0369A1"),

		// Status — Tailwind-inspired semantic colors (light adjusted for WCAG AA)
		Success: AdaptiveColor("#34D399", "#047857"),
		Warning: AdaptiveColor("#FBBF24", "#92400E"),
		Danger:  AdaptiveColor("#F87171", "#DC2626"),
		Info:    AdaptiveColor("#60A5FA", "#2563EB"),

		// Surface — Catppuccin Mocha (dark) / Slate (light)
		SurfaceBg:     AdaptiveColor("#1E1E2E", "#FFFFFF"),
		SurfaceFg:     AdaptiveColor("#CDD6F4", "#1E293B"),
		SurfaceDim:    AdaptiveColor("#585B70", "#94A3B8"),
		SurfaceBorder: AdaptiveColor("#45475A", "#CBD5E1"),

		// Selection / Header
		SelectionBg: AdaptiveColor("#313244", "#E2E8F0"),
		HeaderBg:    AdaptiveColor("#181825", "#F1F5F9"),

		// Text
		TextNormal: AdaptiveColor("#CDD6F4", "#1E293B"),
		TextBright: AdaptiveColor("#FFFFFF", "#0F172A"),
		TextMuted:  AdaptiveColor("#9399B2", "#475569"),
		TextDim:    AdaptiveColor("#7F849C", "#64748B"),
	}
}

// NoColorScheme returns a ColorScheme with all nil colors for NO_COLOR mode.
func NoColorScheme() ColorScheme {
	return ColorScheme{}
}

// NewColorScheme creates a ColorScheme, respecting NO_COLOR, GROVE_NO_COLOR,
// and GROVE_HIGH_CONTRAST environment variables.
func NewColorScheme() ColorScheme {
	if IsNoColor() {
		return NoColorScheme()
	}
	if IsHighContrast() {
		return HighContrastColorScheme()
	}
	return DefaultColorScheme()
}

// IsNoColor checks if color output should be suppressed.
func IsNoColor() bool {
	_, nc := os.LookupEnv("NO_COLOR")
	_, gnc := os.LookupEnv("GROVE_NO_COLOR")
	return nc || gnc
}

// IsHighContrast checks if high-contrast mode is requested.
func IsHighContrast() bool {
	_, hc := os.LookupEnv("GROVE_HIGH_CONTRAST")
	return hc
}

// AdaptiveColor picks between a dark and light hex color based on terminal mode.
func AdaptiveColor(dark, light string) color.Color {
	if IsLightMode() {
		return lipgloss.Color(light)
	}
	return lipgloss.Color(dark)
}

// IsLightMode checks if the terminal is in light mode.
func IsLightMode() bool {
	if val, ok := os.LookupEnv("GROVE_LIGHT_MODE"); ok {
		return val != "0" && val != "false" && val != "no"
	}
	if colorfgbg, ok := os.LookupEnv("COLORFGBG"); ok {
		parts := strings.Split(colorfgbg, ";")
		if len(parts) >= 2 {
			if bg, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
				return bg >= 8
			}
		}
	}
	return false
}

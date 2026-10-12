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
	return DefaultPalette.Scheme()
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

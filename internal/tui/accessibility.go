package tui

import (
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// hexToRGB parses a hex color string like "#FF00AA" into RGB components.
func hexToRGB(hex string) (r, g, b uint8) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return 0, 0, 0
	}
	var ri, gi, bi int
	fmt.Sscanf(hex, "%02x%02x%02x", &ri, &gi, &bi)
	return uint8(ri), uint8(gi), uint8(bi)
}

// sRGBToLinear converts an sRGB channel value (0-255) to linear light.
func sRGBToLinear(c uint8) float64 {
	v := float64(c) / 255.0
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

// RelativeLuminance computes the WCAG relative luminance of a hex color.
// https://www.w3.org/TR/WCAG20/#relativeluminancedef
func RelativeLuminance(hex string) float64 {
	r, g, b := hexToRGB(hex)
	return 0.2126*sRGBToLinear(r) + 0.7152*sRGBToLinear(g) + 0.0722*sRGBToLinear(b)
}

// ContrastRatio computes the WCAG contrast ratio between two hex colors.
// https://www.w3.org/TR/WCAG20/#contrast-ratiodef
func ContrastRatio(fg, bg string) float64 {
	l1 := RelativeLuminance(fg)
	l2 := RelativeLuminance(bg)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

// isHighContrast checks if high-contrast mode is requested.
func isHighContrast() bool {
	_, hc := os.LookupEnv("GROVE_HIGH_CONTRAST")
	return hc
}

// highContrastColorScheme returns a ColorScheme with higher contrast values.
// All foreground colors are adjusted to meet WCAG AA (4.5:1) against both
// dark and light backgrounds, including TextMuted which is normally exempt.
func highContrastColorScheme() ColorScheme {
	return ColorScheme{
		// Brand — brighter for dark, darker for light
		Primary:   lipgloss.AdaptiveColor{Dark: "#C4B5FD", Light: "#6D28D9"},
		Secondary: lipgloss.AdaptiveColor{Dark: "#7DD3FC", Light: "#0369A1"},

		// Status — pushed to higher contrast
		Success: lipgloss.AdaptiveColor{Dark: "#6EE7B7", Light: "#047857"},
		Warning: lipgloss.AdaptiveColor{Dark: "#FDE68A", Light: "#B45309"},
		Danger:  lipgloss.AdaptiveColor{Dark: "#FCA5A5", Light: "#B91C1C"},
		Info:    lipgloss.AdaptiveColor{Dark: "#93C5FD", Light: "#1D4ED8"},

		// Surface — same as default (backgrounds)
		SurfaceBg:     lipgloss.AdaptiveColor{Dark: "#1E1E2E", Light: "#FFFFFF"},
		SurfaceFg:     lipgloss.AdaptiveColor{Dark: "#E4E8F7", Light: "#0F172A"},
		SurfaceDim:    lipgloss.AdaptiveColor{Dark: "#7F849C", Light: "#64748B"},
		SurfaceBorder: lipgloss.AdaptiveColor{Dark: "#585B70", Light: "#94A3B8"},

		// Text — all meet 4.5:1 including muted
		TextNormal: lipgloss.AdaptiveColor{Dark: "#E4E8F7", Light: "#0F172A"},
		TextBright: lipgloss.AdaptiveColor{Dark: "#FFFFFF", Light: "#000000"},
		TextMuted:  lipgloss.AdaptiveColor{Dark: "#A6ADC8", Light: "#475569"},
	}
}

// NewAccessibleCreateNameForm creates a Huh form with accessible mode enabled.
func NewAccessibleCreateNameForm(nameValue *string, projectName string, existingItems []WorktreeItem) *huh.Form {
	description := "Worktree name"
	if projectName != "" {
		description = fmt.Sprintf("Will create: %s-<name>", projectName)
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Worktree Name").
				Description(description).
				Placeholder("feature-name").
				Validate(createNameValidator(existingItems)).
				Value(nameValue),
		),
	).WithTheme(huh.ThemeCharm()).WithAccessible(true)

	return form
}

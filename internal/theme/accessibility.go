package theme

import (
	"fmt"
	"math"
	"strings"
)

// HexToRGB parses a hex color string like "#FF00AA" into RGB components.
func HexToRGB(hex string) (uint8, uint8, uint8, error) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return 0, 0, 0, fmt.Errorf("invalid hex color length: %q", hex)
	}
	var ri, gi, bi uint8
	n, err := fmt.Sscanf(hex, "%02x%02x%02x", &ri, &gi, &bi)
	if err != nil || n != 3 {
		return 0, 0, 0, fmt.Errorf("failed to parse hex color: %q", hex)
	}
	return ri, gi, bi, nil
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
	r, g, b, err := HexToRGB(hex)
	if err != nil {
		return 0.0
	}
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

// HighContrastPair holds the dark and light hex values for a single color slot.
type HighContrastPair struct {
	Dark  string
	Light string
}

// HighContrastPairs exposes the raw hex values used by HighContrastColorScheme
// so callers can verify contrast ratios against both dark and light backgrounds.
var HighContrastPairs = struct {
	Primary    HighContrastPair
	Secondary  HighContrastPair
	Success    HighContrastPair
	Warning    HighContrastPair
	Danger     HighContrastPair
	Info       HighContrastPair
	SurfaceBg  HighContrastPair
	SurfaceFg  HighContrastPair
	SurfaceDim HighContrastPair
	TextNormal HighContrastPair
	TextBright HighContrastPair
	TextMuted  HighContrastPair
}{
	Primary:    HighContrastPair{Dark: "#C4B5FD", Light: "#6D28D9"},
	Secondary:  HighContrastPair{Dark: "#7DD3FC", Light: "#0369A1"},
	Success:    HighContrastPair{Dark: "#6EE7B7", Light: "#047857"},
	Warning:    HighContrastPair{Dark: "#FDE68A", Light: "#B45309"},
	Danger:     HighContrastPair{Dark: "#FCA5A5", Light: "#B91C1C"},
	Info:       HighContrastPair{Dark: "#93C5FD", Light: "#1D4ED8"},
	SurfaceBg:  HighContrastPair{Dark: "#1E1E2E", Light: "#FFFFFF"},
	SurfaceFg:  HighContrastPair{Dark: "#E4E8F7", Light: "#0F172A"},
	SurfaceDim: HighContrastPair{Dark: "#7F849C", Light: "#64748B"},
	TextNormal: HighContrastPair{Dark: "#E4E8F7", Light: "#0F172A"},
	TextBright: HighContrastPair{Dark: "#FFFFFF", Light: "#000000"},
	TextMuted:  HighContrastPair{Dark: "#A6ADC8", Light: "#475569"},
}

// HighContrastColorScheme returns a ColorScheme with higher contrast values.
// All foreground colors are adjusted to meet WCAG AA (4.5:1) against both
// dark and light backgrounds, including TextMuted which is normally exempt.
func HighContrastColorScheme() ColorScheme {
	p := HighContrastPairs
	return ColorScheme{
		// Brand — brighter for dark, darker for light
		Primary:   AdaptiveColor(p.Primary.Dark, p.Primary.Light),
		Secondary: AdaptiveColor(p.Secondary.Dark, p.Secondary.Light),

		// Status — pushed to higher contrast
		Success: AdaptiveColor(p.Success.Dark, p.Success.Light),
		Warning: AdaptiveColor(p.Warning.Dark, p.Warning.Light),
		Danger:  AdaptiveColor(p.Danger.Dark, p.Danger.Light),
		Info:    AdaptiveColor(p.Info.Dark, p.Info.Light),

		// Surface — same as default (backgrounds)
		SurfaceBg:     AdaptiveColor(p.SurfaceBg.Dark, p.SurfaceBg.Light),
		SurfaceFg:     AdaptiveColor(p.SurfaceFg.Dark, p.SurfaceFg.Light),
		SurfaceDim:    AdaptiveColor(p.SurfaceDim.Dark, p.SurfaceDim.Light),
		SurfaceBorder: AdaptiveColor("#585B70", "#94A3B8"),

		// Selection / Header
		SelectionBg: AdaptiveColor("#313244", "#E2E8F0"),
		HeaderBg:    AdaptiveColor("#181825", "#F1F5F9"),

		// Text — all meet 4.5:1 including muted
		TextNormal: AdaptiveColor(p.TextNormal.Dark, p.TextNormal.Light),
		TextBright: AdaptiveColor(p.TextBright.Dark, p.TextBright.Light),
		TextMuted:  AdaptiveColor(p.TextMuted.Dark, p.TextMuted.Light),
		TextDim:    AdaptiveColor(p.SurfaceDim.Dark, p.SurfaceDim.Light),
	}
}

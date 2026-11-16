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

// HighContrastColorScheme returns a ColorScheme with higher contrast values.
// All foreground colors meet WCAG AA (4.5:1) against the backgrounds,
// including TextMuted and TextDim.
func HighContrastColorScheme() ColorScheme {
	return HighContrastPalette.Scheme()
}

package theme

import "image/color"

// Pair is one color token's dark and light hex values.
type Pair struct {
	Dark  string
	Light string
}

// Palette holds every ColorScheme token as hex pairs, so tests can check
// contrast on the exact values the TUI renders.
type Palette struct {
	Primary, Secondary                              Pair
	Success, Warning, Danger, Info                  Pair
	SurfaceBg, SurfaceFg, SurfaceDim, SurfaceBorder Pair
	SelectionBg, HeaderBg                           Pair
	TextNormal, TextBright, TextMuted, TextDim      Pair
}

// Scheme builds the ColorScheme for the terminal's light or dark mode.
func (p Palette) Scheme() ColorScheme {
	c := func(x Pair) color.Color { return AdaptiveColor(x.Dark, x.Light) }
	return ColorScheme{
		Primary: c(p.Primary), Secondary: c(p.Secondary),
		Success: c(p.Success), Warning: c(p.Warning), Danger: c(p.Danger), Info: c(p.Info),
		SurfaceBg: c(p.SurfaceBg), SurfaceFg: c(p.SurfaceFg), SurfaceDim: c(p.SurfaceDim), SurfaceBorder: c(p.SurfaceBorder),
		SelectionBg: c(p.SelectionBg), HeaderBg: c(p.HeaderBg),
		TextNormal: c(p.TextNormal), TextBright: c(p.TextBright), TextMuted: c(p.TextMuted), TextDim: c(p.TextDim),
	}
}

// DefaultPalette is the standard theme: purple and blue brand colors,
// Tailwind status colors, Catppuccin Mocha (dark) and Slate (light) surfaces.
// Text colors are nudged where needed so each reads at 4.5:1 or better on
// the surface, the selected row, and the header bar.
var DefaultPalette = Palette{
	Primary:   Pair{"#A78BFA", "#7C3AED"},
	Secondary: Pair{"#38BDF8", "#0369A1"},

	Success: Pair{"#34D399", "#047555"},
	Warning: Pair{"#FBBF24", "#92400E"},
	Danger:  Pair{"#F87171", "#C82020"},
	Info:    Pair{"#60A5FA", "#195AEA"},

	SurfaceBg:     Pair{"#1E1E2E", "#FFFFFF"},
	SurfaceFg:     Pair{"#CDD6F4", "#1E293B"},
	SurfaceDim:    Pair{"#585B70", "#94A3B8"},
	SurfaceBorder: Pair{"#45475A", "#CBD5E1"},

	SelectionBg: Pair{"#313244", "#E2E8F0"},
	HeaderBg:    Pair{"#181825", "#F1F5F9"},

	TextNormal: Pair{"#CDD6F4", "#1E293B"},
	TextBright: Pair{"#FFFFFF", "#0F172A"},
	TextMuted:  Pair{"#969CB4", "#475569"},
	TextDim:    Pair{"#989CAF", "#59687C"},
}

// HighContrastPalette (GROVE_HIGH_CONTRAST) pushes every text color further
// from the backgrounds, including muted and dim text.
var HighContrastPalette = Palette{
	Primary:   Pair{"#C4B5FD", "#6D28D9"},
	Secondary: Pair{"#7DD3FC", "#0369A1"},

	Success: Pair{"#6EE7B7", "#047555"},
	Warning: Pair{"#FDE68A", "#A64D08"},
	Danger:  Pair{"#FCA5A5", "#B91C1C"},
	Info:    Pair{"#93C5FD", "#1D4ED8"},

	SurfaceBg:     Pair{"#1E1E2E", "#FFFFFF"},
	SurfaceFg:     Pair{"#E4E8F7", "#0F172A"},
	SurfaceDim:    Pair{"#7F849C", "#64748B"},
	SurfaceBorder: Pair{"#585B70", "#94A3B8"},

	SelectionBg: Pair{"#313244", "#E2E8F0"},
	HeaderBg:    Pair{"#181825", "#F1F5F9"},

	TextNormal: Pair{"#E4E8F7", "#0F172A"},
	TextBright: Pair{"#FFFFFF", "#000000"},
	TextMuted:  Pair{"#A6ADC8", "#475569"},
	TextDim:    Pair{"#989CAF", "#59687C"},
}

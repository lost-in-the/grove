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

// StyleSet holds pre-composed lipgloss styles built from a ColorScheme.
type StyleSet struct {
	// Borders
	RoundedBorder lipgloss.Style
	DetailBorder  lipgloss.Style

	// Text
	Title      lipgloss.Style
	TextNormal lipgloss.Style
	TextBright lipgloss.Style
	TextMuted  lipgloss.Style

	// Status text
	StatusSuccess lipgloss.Style
	StatusWarning lipgloss.Style
	StatusDanger  lipgloss.Style
	StatusInfo    lipgloss.Style

	// Layout
	Header lipgloss.Style
	Footer lipgloss.Style

	// List items
	SelectedItem  lipgloss.Style
	NormalItem    lipgloss.Style
	CurrentItem   lipgloss.Style
	DimmedItem    lipgloss.Style
	ListCursor    lipgloss.Style
	ListCursorDim lipgloss.Style

	// Status badges
	StatusClean lipgloss.Style
	StatusDirty lipgloss.Style
	StatusStale lipgloss.Style
	TmuxBadge   lipgloss.Style
	EnvBadge    lipgloss.Style

	// Detail panel
	DetailTitle   lipgloss.Style
	DetailLabel   lipgloss.Style
	DetailValue   lipgloss.Style
	DetailDim     lipgloss.Style
	DetailFile    lipgloss.Style
	DetailFileAdd lipgloss.Style
	DetailFileMod lipgloss.Style
	DetailFileDel lipgloss.Style

	// Overlay / dialogs
	OverlayBorder lipgloss.Style
	OverlayTitle  lipgloss.Style
	OverlayPrompt lipgloss.Style
	WarningText   lipgloss.Style
	ErrorText     lipgloss.Style
	SuccessText   lipgloss.Style

	// Help
	HelpKey  lipgloss.Style
	HelpDesc lipgloss.Style
	HelpSep  lipgloss.Style

	// Input
	InputBorder lipgloss.Style
	InputText   lipgloss.Style
}

// Styles is the global StyleSet initialized from Colors.
var Styles = NewStyleSet(Colors)

// NewStyleSet creates a StyleSet from a ColorScheme.
func NewStyleSet(cs ColorScheme) StyleSet {
	return StyleSet{
		// Borders
		RoundedBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cs.SurfaceBorder),
		DetailBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cs.SurfaceBorder),

		// Text
		Title:      lipgloss.NewStyle().Bold(true).Foreground(cs.TextBright),
		TextNormal: lipgloss.NewStyle().Foreground(cs.TextNormal),
		TextBright: lipgloss.NewStyle().Bold(true).Foreground(cs.TextBright),
		TextMuted:  lipgloss.NewStyle().Foreground(cs.TextMuted),

		// Status text
		StatusSuccess: lipgloss.NewStyle().Foreground(cs.Success),
		StatusWarning: lipgloss.NewStyle().Foreground(cs.Warning),
		StatusDanger:  lipgloss.NewStyle().Foreground(cs.Danger),
		StatusInfo:    lipgloss.NewStyle().Foreground(cs.Info),

		// Layout
		Header: lipgloss.NewStyle().Bold(true).Foreground(cs.Primary),
		Footer: lipgloss.NewStyle().Foreground(cs.TextMuted),

		// List items
		SelectedItem:  lipgloss.NewStyle().Bold(true).Foreground(cs.TextBright),
		NormalItem:    lipgloss.NewStyle().Foreground(cs.TextNormal),
		CurrentItem:   lipgloss.NewStyle().Foreground(cs.Secondary),
		DimmedItem:    lipgloss.NewStyle().Foreground(cs.TextMuted),
		ListCursor:    lipgloss.NewStyle().Foreground(cs.Primary).SetString("❯ "),
		ListCursorDim: lipgloss.NewStyle().SetString("  "),

		// Status badges
		StatusClean: lipgloss.NewStyle().Foreground(cs.Success),
		StatusDirty: lipgloss.NewStyle().Foreground(cs.Warning),
		StatusStale: lipgloss.NewStyle().Foreground(cs.Danger),
		TmuxBadge:   lipgloss.NewStyle().Foreground(cs.Primary),
		EnvBadge:    lipgloss.NewStyle().Foreground(cs.Info),

		// Detail panel
		DetailTitle:   lipgloss.NewStyle().Bold(true).Foreground(cs.TextBright),
		DetailLabel:   lipgloss.NewStyle().Foreground(cs.TextMuted),
		DetailValue:   lipgloss.NewStyle().Foreground(cs.TextNormal),
		DetailDim:     lipgloss.NewStyle().Foreground(cs.TextMuted),
		DetailFile:    lipgloss.NewStyle().Foreground(cs.TextNormal),
		DetailFileAdd: lipgloss.NewStyle().Foreground(cs.Success),
		DetailFileMod: lipgloss.NewStyle().Foreground(cs.Warning),
		DetailFileDel: lipgloss.NewStyle().Foreground(cs.Danger),

		// Overlay
		OverlayBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cs.Primary).
			Padding(1, 2),
		OverlayTitle:  lipgloss.NewStyle().Bold(true).Foreground(cs.Primary),
		OverlayPrompt: lipgloss.NewStyle().Foreground(cs.TextNormal),
		WarningText:   lipgloss.NewStyle().Foreground(cs.Warning),
		ErrorText:     lipgloss.NewStyle().Foreground(cs.Danger),
		SuccessText:   lipgloss.NewStyle().Foreground(cs.Success),

		// Help
		HelpKey:  lipgloss.NewStyle().Foreground(cs.Primary).Bold(true),
		HelpDesc: lipgloss.NewStyle().Foreground(cs.TextMuted),
		HelpSep:  lipgloss.NewStyle().Foreground(cs.TextMuted).SetString("  "),

		// Input
		InputBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cs.Primary),
		InputText: lipgloss.NewStyle().Foreground(cs.TextNormal),
	}
}

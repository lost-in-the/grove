package tui

import (
	lipgloss "charm.land/lipgloss/v2"

	"github.com/lost-in-the/grove/internal/theme"
)

// ColorScheme is re-exported from internal/theme for backward compatibility.
type ColorScheme = theme.ColorScheme

// Colors is the global color scheme, delegating to the shared theme package.
var Colors = theme.Colors

// NewColorScheme delegates to the shared theme package.
func NewColorScheme() ColorScheme {
	return theme.NewColorScheme()
}

// isNoColor delegates to the shared theme package.
func isNoColor() bool {
	return theme.IsNoColor()
}

// defaultColorScheme delegates to theme for backward compatibility with tests.
func defaultColorScheme() ColorScheme {
	return theme.DefaultColorScheme()
}

// noColorScheme delegates to theme for backward compatibility with tests.
func noColorScheme() ColorScheme {
	return theme.NoColorScheme()
}

// highContrastColorScheme delegates to theme for backward compatibility with tests.
func highContrastColorScheme() ColorScheme {
	return theme.HighContrastColorScheme()
}

// hexToRGB delegates to theme for backward compatibility with tests.
func hexToRGB(hex string) (r, g, b uint8, err error) {
	return theme.HexToRGB(hex)
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

	// List selection
	SelectionRow lipgloss.Style

	// Status badges
	StatusClean          lipgloss.Style
	StatusDirty          lipgloss.Style
	StatusStale          lipgloss.Style
	TmuxBadge            lipgloss.Style
	TmuxBadgeActive      lipgloss.Style
	EnvBadge             lipgloss.Style
	ContainerBadge       lipgloss.Style
	ContainerBadgeActive lipgloss.Style
	ContainerBadgeWarn   lipgloss.Style

	// Detail panel
	DetailTitle   lipgloss.Style
	DetailLabel   lipgloss.Style
	DetailValue   lipgloss.Style
	DetailDim     lipgloss.Style
	DetailFile    lipgloss.Style
	DetailFileAdd lipgloss.Style
	DetailFileMod lipgloss.Style
	DetailFileDel lipgloss.Style

	// Layout
	HeaderBar lipgloss.Style

	// Overlay / dialogs
	OverlayBorder        lipgloss.Style
	OverlayBorderDanger  lipgloss.Style
	OverlayBorderSuccess lipgloss.Style
	OverlayBorderInfo    lipgloss.Style
	OverlayTitle         lipgloss.Style
	OverlayPrompt        lipgloss.Style
	WarningText          lipgloss.Style
	ErrorText            lipgloss.Style
	SuccessText          lipgloss.Style

	// Help
	HelpKey          lipgloss.Style
	HelpKeyHighlight lipgloss.Style
	HelpDesc         lipgloss.Style
	HelpSep          lipgloss.Style

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

		// List selection
		SelectionRow: lipgloss.NewStyle().Background(cs.SelectionBg),

		// Layout
		Header: lipgloss.NewStyle().Bold(true).Foreground(cs.Info),
		Footer: lipgloss.NewStyle().Foreground(cs.TextMuted),
		HeaderBar: lipgloss.NewStyle().
			Background(cs.HeaderBg).
			Bold(true).
			Padding(0, 1),

		// List items
		SelectedItem:  lipgloss.NewStyle().Bold(true).Foreground(cs.TextBright),
		NormalItem:    lipgloss.NewStyle().Foreground(cs.TextNormal),
		CurrentItem:   lipgloss.NewStyle().Foreground(cs.Secondary),
		DimmedItem:    lipgloss.NewStyle().Foreground(cs.TextDim),
		ListCursor:    lipgloss.NewStyle().Foreground(cs.Info),
		ListCursorDim: lipgloss.NewStyle(),

		// Status badges
		StatusClean:          lipgloss.NewStyle().Foreground(cs.Success),
		StatusDirty:          lipgloss.NewStyle().Foreground(cs.Warning),
		StatusStale:          lipgloss.NewStyle().Foreground(cs.Danger),
		TmuxBadge:            lipgloss.NewStyle().Foreground(cs.Primary),
		TmuxBadgeActive:      lipgloss.NewStyle().Foreground(cs.Primary),
		EnvBadge:             lipgloss.NewStyle().Foreground(cs.Info),
		ContainerBadge:       lipgloss.NewStyle().Foreground(cs.Info),
		ContainerBadgeActive: lipgloss.NewStyle().Foreground(cs.Secondary),
		ContainerBadgeWarn:   lipgloss.NewStyle().Foreground(cs.Warning),

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
		OverlayBorderDanger: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cs.Danger).
			Padding(1, 2),
		OverlayBorderSuccess: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cs.Success).
			Padding(1, 2),
		OverlayBorderInfo: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cs.Info).
			Padding(1, 2),
		OverlayTitle:  lipgloss.NewStyle().Bold(true).Foreground(cs.Primary),
		OverlayPrompt: lipgloss.NewStyle().Foreground(cs.TextNormal),
		WarningText:   lipgloss.NewStyle().Foreground(cs.Warning),
		ErrorText:     lipgloss.NewStyle().Foreground(cs.Danger),
		SuccessText:   lipgloss.NewStyle().Foreground(cs.Success),

		// Help
		HelpKey:          lipgloss.NewStyle().Foreground(cs.Primary).Bold(true),
		HelpKeyHighlight: lipgloss.NewStyle().Foreground(cs.HeaderBg).Background(cs.Primary).Bold(true),
		HelpDesc:         lipgloss.NewStyle().Foreground(cs.TextMuted),
		HelpSep:          lipgloss.NewStyle().Foreground(cs.TextMuted),

		// Input
		InputBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cs.Primary),
		InputText: lipgloss.NewStyle().Foreground(cs.TextNormal),
	}
}

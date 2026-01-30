package tui

import "github.com/charmbracelet/lipgloss"

// Theme holds all styles for the TUI.
var Theme = struct {
	// Layout
	App          lipgloss.Style
	Header       lipgloss.Style
	Footer       lipgloss.Style
	DetailBorder lipgloss.Style

	// List items
	SelectedItem   lipgloss.Style
	NormalItem     lipgloss.Style
	CurrentItem    lipgloss.Style
	DimmedItem     lipgloss.Style
	ListCursor     lipgloss.Style
	ListCursorDim  lipgloss.Style

	// Status badges
	StatusClean lipgloss.Style
	StatusDirty lipgloss.Style
	StatusStale lipgloss.Style
	TmuxBadge   lipgloss.Style
	EnvBadge    lipgloss.Style

	// Detail panel
	DetailTitle    lipgloss.Style
	DetailLabel    lipgloss.Style
	DetailValue    lipgloss.Style
	DetailDim      lipgloss.Style
	DetailFile     lipgloss.Style
	DetailFileAdd  lipgloss.Style
	DetailFileMod  lipgloss.Style
	DetailFileDel  lipgloss.Style

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
}{
	App:    lipgloss.NewStyle(),
	Header: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10")),
	Footer: lipgloss.NewStyle().Foreground(lipgloss.Color("8")),

	DetailBorder: lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("8")),

	SelectedItem:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")),
	NormalItem:    lipgloss.NewStyle().Foreground(lipgloss.Color("7")),
	CurrentItem:   lipgloss.NewStyle().Foreground(lipgloss.Color("14")),
	DimmedItem:    lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
	ListCursor:    lipgloss.NewStyle().Foreground(lipgloss.Color("10")).SetString("❯ "),
	ListCursorDim: lipgloss.NewStyle().SetString("  "),

	StatusClean: lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
	StatusDirty: lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
	StatusStale: lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
	TmuxBadge:   lipgloss.NewStyle().Foreground(lipgloss.Color("13")),
	EnvBadge:    lipgloss.NewStyle().Foreground(lipgloss.Color("6")),

	DetailTitle:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")),
	DetailLabel:   lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
	DetailValue:   lipgloss.NewStyle().Foreground(lipgloss.Color("7")),
	DetailDim:     lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
	DetailFile:    lipgloss.NewStyle().Foreground(lipgloss.Color("7")),
	DetailFileAdd: lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
	DetailFileMod: lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
	DetailFileDel: lipgloss.NewStyle().Foreground(lipgloss.Color("9")),

	OverlayBorder: lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("12")).
		Padding(1, 2),
	OverlayTitle:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")),
	OverlayPrompt: lipgloss.NewStyle().Foreground(lipgloss.Color("7")),
	WarningText:   lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
	ErrorText:     lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
	SuccessText:   lipgloss.NewStyle().Foreground(lipgloss.Color("10")),

	HelpKey:  lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true),
	HelpDesc: lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
	HelpSep:  lipgloss.NewStyle().Foreground(lipgloss.Color("8")).SetString("  "),
}

package tui

import (
	"os"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/lost-in-the/grove/internal/theme"
)

func TestColorScheme_SemanticColorsNotNil(t *testing.T) {
	scheme := theme.DefaultColorScheme()
	colors := map[string]interface{}{
		"Primary":   scheme.Primary,
		"Secondary": scheme.Secondary,
		"Success":   scheme.Success,
		"Warning":   scheme.Warning,
		"Danger":    scheme.Danger,
		"Info":      scheme.Info,
	}

	for name, c := range colors {
		if c == nil {
			t.Errorf("%s is nil, want non-nil color", name)
		}
	}
}

func TestColorScheme_SurfaceColorsNotNil(t *testing.T) {
	scheme := theme.DefaultColorScheme()
	colors := map[string]interface{}{
		"SurfaceBg":     scheme.SurfaceBg,
		"SurfaceFg":     scheme.SurfaceFg,
		"SurfaceDim":    scheme.SurfaceDim,
		"SurfaceBorder": scheme.SurfaceBorder,
	}

	for name, c := range colors {
		if c == nil {
			t.Errorf("%s is nil, want non-nil color", name)
		}
	}
}

func TestColorScheme_TextColorsNotNil(t *testing.T) {
	scheme := theme.DefaultColorScheme()
	colors := map[string]interface{}{
		"TextNormal": scheme.TextNormal,
		"TextBright": scheme.TextBright,
		"TextMuted":  scheme.TextMuted,
	}

	for name, c := range colors {
		if c == nil {
			t.Errorf("%s is nil, want non-nil color", name)
		}
	}
}

func TestColorScheme_NOCOLORRespected(t *testing.T) {
	_ = os.Setenv("NO_COLOR", "1")
	defer func() { _ = os.Unsetenv("NO_COLOR") }()

	scheme := NewColorScheme()
	// When NO_COLOR is set, all colors should be nil (zero-value ColorScheme)
	if scheme.Primary != nil {
		t.Error("expected nil Primary when NO_COLOR is set")
	}
	if scheme.Success != nil {
		t.Error("expected nil Success when NO_COLOR is set")
	}
	if scheme.Danger != nil {
		t.Error("expected nil Danger when NO_COLOR is set")
	}
}

// --- StyleSet Tests ---

func TestStyleSet_NewStyleSetReturnsPopulated(t *testing.T) {
	scheme := theme.DefaultColorScheme()
	ss := NewStyleSet(scheme)

	// Verify key styles are non-zero (have properties set)
	tests := []struct {
		name  string
		style lipgloss.Style
		check func(lipgloss.Style) bool
		desc  string
	}{
		{"RoundedBorder has border", ss.RoundedBorder,
			func(s lipgloss.Style) bool { return s.GetBorderStyle() != lipgloss.Border{} },
			"should have a border"},
		{"Title is bold", ss.Title,
			func(s lipgloss.Style) bool { return s.GetBold() },
			"should be bold"},
		{"TextMuted has foreground", ss.TextMuted,
			func(s lipgloss.Style) bool { return s.GetForeground() != lipgloss.NoColor{} },
			"should have foreground color"},
		{"StatusSuccess has foreground", ss.StatusSuccess,
			func(s lipgloss.Style) bool { return s.GetForeground() != lipgloss.NoColor{} },
			"should have foreground color"},
		{"StatusWarning has foreground", ss.StatusWarning,
			func(s lipgloss.Style) bool { return s.GetForeground() != lipgloss.NoColor{} },
			"should have foreground color"},
		{"StatusDanger has foreground", ss.StatusDanger,
			func(s lipgloss.Style) bool { return s.GetForeground() != lipgloss.NoColor{} },
			"should have foreground color"},
		{"OverlayBorder has padding", ss.OverlayBorder,
			func(s lipgloss.Style) bool { return s.GetPaddingLeft() > 0 },
			"should have padding"},
		{"HelpKey is bold", ss.HelpKey,
			func(s lipgloss.Style) bool { return s.GetBold() },
			"should be bold"},
		{"ListCursor has foreground", ss.ListCursor,
			func(s lipgloss.Style) bool { return s.GetForeground() != lipgloss.NoColor{} },
			"should have foreground color"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.check(tt.style) {
				t.Errorf("StyleSet.%s: %s", tt.name, tt.desc)
			}
		})
	}
}

func TestStyleSet_AllCategoriesPresent(t *testing.T) {
	scheme := theme.DefaultColorScheme()
	ss := NewStyleSet(scheme)

	// Borders
	_ = ss.RoundedBorder
	_ = ss.DetailBorder

	// Text
	_ = ss.Title
	_ = ss.TextNormal
	_ = ss.TextBright
	_ = ss.TextMuted

	// Status
	_ = ss.StatusSuccess
	_ = ss.StatusWarning
	_ = ss.StatusDanger
	_ = ss.StatusInfo

	// List items
	_ = ss.SelectedItem
	_ = ss.NormalItem
	_ = ss.CurrentItem
	_ = ss.DimmedItem
	_ = ss.ListCursor
	_ = ss.ListCursorDim

	// Status badges
	_ = ss.StatusClean
	_ = ss.StatusDirty
	_ = ss.StatusStale
	_ = ss.TmuxBadge
	_ = ss.EnvBadge

	// Detail panel
	_ = ss.DetailTitle
	_ = ss.DetailLabel
	_ = ss.DetailValue
	_ = ss.DetailDim
	_ = ss.DetailFile
	_ = ss.DetailFileAdd
	_ = ss.DetailFileMod
	_ = ss.DetailFileDel

	// Overlay
	_ = ss.OverlayBorder
	_ = ss.OverlayTitle
	_ = ss.OverlayPrompt
	_ = ss.WarningText
	_ = ss.ErrorText
	_ = ss.SuccessText

	// Help
	_ = ss.HelpKey
	_ = ss.HelpDesc
	_ = ss.HelpSep

	// Footer
	_ = ss.Header
	_ = ss.Footer

	// Input
	_ = ss.InputBorder
	_ = ss.InputText
}

func TestStyleSet_GlobalStylesInitialized(t *testing.T) {
	// The global Styles variable should be initialized
	if Styles.Title.GetBold() != true {
		t.Error("global Styles.Title should be bold")
	}
}

func TestStyleSet_NOCOLORProducesPlainStyles(t *testing.T) {
	scheme := theme.NoColorScheme()
	ss := NewStyleSet(scheme)

	// Styles should still be valid (not panic), just without colors
	_ = ss.StatusSuccess.Render("test")
	_ = ss.RoundedBorder.Render("test")
	_ = ss.Title.Render("test")
}

func TestColorScheme_AllFieldsPopulated(t *testing.T) {
	scheme := theme.DefaultColorScheme()

	// Verify no semantic color is nil
	colors := map[string]interface{}{
		"Primary":       scheme.Primary,
		"Secondary":     scheme.Secondary,
		"Success":       scheme.Success,
		"Warning":       scheme.Warning,
		"Danger":        scheme.Danger,
		"Info":          scheme.Info,
		"SurfaceBg":     scheme.SurfaceBg,
		"SurfaceFg":     scheme.SurfaceFg,
		"SurfaceDim":    scheme.SurfaceDim,
		"SurfaceBorder": scheme.SurfaceBorder,
		"TextNormal":    scheme.TextNormal,
		"TextBright":    scheme.TextBright,
		"TextMuted":     scheme.TextMuted,
	}

	for name, c := range colors {
		if c == nil {
			t.Errorf("%s is nil, want non-nil color", name)
		}
	}
}

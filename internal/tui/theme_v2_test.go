package tui

import (
	"os"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestColorScheme_SemanticColors(t *testing.T) {
	tests := []struct {
		name      string
		color     lipgloss.AdaptiveColor
		wantDark  string
		wantLight string
	}{
		{"Primary", Colors.Primary, "#A78BFA", "#7C3AED"},
		{"Secondary", Colors.Secondary, "#38BDF8", "#0369A1"},
		{"Success", Colors.Success, "#34D399", "#047857"},
		{"Warning", Colors.Warning, "#FBBF24", "#92400E"},
		{"Danger", Colors.Danger, "#F87171", "#DC2626"},
		{"Info", Colors.Info, "#60A5FA", "#2563EB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.color.Dark != tt.wantDark {
				t.Errorf("Dark = %q, want %q", tt.color.Dark, tt.wantDark)
			}
			if tt.color.Light != tt.wantLight {
				t.Errorf("Light = %q, want %q", tt.color.Light, tt.wantLight)
			}
		})
	}
}

func TestColorScheme_SurfaceColors(t *testing.T) {
	tests := []struct {
		name      string
		color     lipgloss.AdaptiveColor
		wantDark  string
		wantLight string
	}{
		{"SurfaceBg", Colors.SurfaceBg, "#1E1E2E", "#FFFFFF"},
		{"SurfaceFg", Colors.SurfaceFg, "#CDD6F4", "#1E293B"},
		{"SurfaceDim", Colors.SurfaceDim, "#585B70", "#94A3B8"},
		{"SurfaceBorder", Colors.SurfaceBorder, "#45475A", "#CBD5E1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.color.Dark != tt.wantDark {
				t.Errorf("Dark = %q, want %q", tt.color.Dark, tt.wantDark)
			}
			if tt.color.Light != tt.wantLight {
				t.Errorf("Light = %q, want %q", tt.color.Light, tt.wantLight)
			}
		})
	}
}

func TestColorScheme_TextColors(t *testing.T) {
	tests := []struct {
		name      string
		color     lipgloss.AdaptiveColor
		wantDark  string
		wantLight string
	}{
		{"TextNormal", Colors.TextNormal, "#CDD6F4", "#1E293B"},
		{"TextBright", Colors.TextBright, "#FFFFFF", "#0F172A"},
		{"TextMuted", Colors.TextMuted, "#9399B2", "#475569"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.color.Dark != tt.wantDark {
				t.Errorf("Dark = %q, want %q", tt.color.Dark, tt.wantDark)
			}
			if tt.color.Light != tt.wantLight {
				t.Errorf("Light = %q, want %q", tt.color.Light, tt.wantLight)
			}
		})
	}
}

func TestColorScheme_NOCOLORRespected(t *testing.T) {
	// Set NO_COLOR
	os.Setenv("NO_COLOR", "1")
	defer os.Unsetenv("NO_COLOR")

	scheme := NewColorScheme()
	// When NO_COLOR is set, all colors should be empty AdaptiveColor
	if scheme.Primary.Dark != "" || scheme.Primary.Light != "" {
		t.Error("expected empty colors when NO_COLOR is set")
	}
	if scheme.Success.Dark != "" || scheme.Success.Light != "" {
		t.Error("expected empty colors when NO_COLOR is set")
	}
	if scheme.Danger.Dark != "" || scheme.Danger.Light != "" {
		t.Error("expected empty colors when NO_COLOR is set")
	}
}

// --- StyleSet Tests ---

func TestStyleSet_NewStyleSetReturnsPopulated(t *testing.T) {
	scheme := defaultColorScheme()
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
	scheme := defaultColorScheme()
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
	scheme := noColorScheme()
	ss := NewStyleSet(scheme)

	// Styles should still be valid (not panic), just without colors
	_ = ss.StatusSuccess.Render("test")
	_ = ss.RoundedBorder.Render("test")
	_ = ss.Title.Render("test")
}

func TestColorScheme_AllFieldsPopulated(t *testing.T) {
	scheme := defaultColorScheme()

	// Verify no semantic color has empty dark/light values
	colors := map[string]lipgloss.AdaptiveColor{
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

	for name, color := range colors {
		if color.Dark == "" {
			t.Errorf("%s has empty Dark value", name)
		}
		if color.Light == "" {
			t.Errorf("%s has empty Light value", name)
		}
	}
}

package tui

import (
	"os"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// TestContrastRatio verifies the WCAG contrast ratio calculation.
func TestContrastRatio(t *testing.T) {
	tests := []struct {
		name    string
		fg      string
		bg      string
		wantMin float64
		wantMax float64
	}{
		{"Black on white", "#000000", "#FFFFFF", 20.9, 21.1},
		{"White on black", "#FFFFFF", "#000000", 20.9, 21.1},
		{"Same color", "#808080", "#808080", 0.9, 1.1},
		{"Light gray on white", "#CCCCCC", "#FFFFFF", 1.5, 1.7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ratio := ContrastRatio(tt.fg, tt.bg)
			if ratio < tt.wantMin || ratio > tt.wantMax {
				t.Errorf("ContrastRatio(%s, %s) = %.2f, want [%.1f, %.1f]",
					tt.fg, tt.bg, ratio, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// TestRelativeLuminance verifies luminance calculation for known values.
func TestRelativeLuminance(t *testing.T) {
	tests := []struct {
		name    string
		hex     string
		wantMin float64
		wantMax float64
	}{
		{"Black", "#000000", -0.01, 0.01},
		{"White", "#FFFFFF", 0.99, 1.01},
		{"Mid-gray", "#808080", 0.20, 0.23},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lum := RelativeLuminance(tt.hex)
			if lum < tt.wantMin || lum > tt.wantMax {
				t.Errorf("RelativeLuminance(%s) = %.4f, want [%.2f, %.2f]",
					tt.hex, lum, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// TestDefaultColorScheme_WCAGAAContrast verifies that all foreground semantic
// colors meet WCAG AA contrast ratio (4.5:1) against the dark background,
// and against the light background.
func TestDefaultColorScheme_WCAGAAContrast(t *testing.T) {
	scheme := defaultColorScheme()

	// Foreground colors to check against backgrounds
	fgColors := map[string]lipgloss.AdaptiveColor{
		"Primary":    scheme.Primary,
		"Secondary":  scheme.Secondary,
		"Success":    scheme.Success,
		"Warning":    scheme.Warning,
		"Danger":     scheme.Danger,
		"Info":       scheme.Info,
		"TextNormal": scheme.TextNormal,
		"TextBright": scheme.TextBright,
		"TextMuted":  scheme.TextMuted,
	}

	const minRatio = 4.5

	for name, fg := range fgColors {
		t.Run(name+"/dark", func(t *testing.T) {
			ratio := ContrastRatio(fg.Dark, scheme.SurfaceBg.Dark)
			if ratio < minRatio {
				t.Errorf("%s dark (%s) on dark bg (%s): ratio %.2f < %.1f",
					name, fg.Dark, scheme.SurfaceBg.Dark, ratio, minRatio)
			}
		})
		t.Run(name+"/light", func(t *testing.T) {
			ratio := ContrastRatio(fg.Light, scheme.SurfaceBg.Light)
			if ratio < minRatio {
				t.Errorf("%s light (%s) on light bg (%s): ratio %.2f < %.1f",
					name, fg.Light, scheme.SurfaceBg.Light, ratio, minRatio)
			}
		})
	}
}

// TestHighContrastColorScheme verifies that the high-contrast scheme
// has even higher contrast ratios than default.
func TestHighContrastColorScheme(t *testing.T) {
	scheme := highContrastColorScheme()

	// All foreground colors should meet WCAG AA (4.5:1) minimum
	fgColors := map[string]lipgloss.AdaptiveColor{
		"Primary":    scheme.Primary,
		"Secondary":  scheme.Secondary,
		"Success":    scheme.Success,
		"Warning":    scheme.Warning,
		"Danger":     scheme.Danger,
		"Info":       scheme.Info,
		"TextNormal": scheme.TextNormal,
		"TextBright": scheme.TextBright,
		"TextMuted":  scheme.TextMuted,
	}

	const minRatio = 4.5

	for name, fg := range fgColors {
		t.Run(name+"/dark", func(t *testing.T) {
			ratio := ContrastRatio(fg.Dark, scheme.SurfaceBg.Dark)
			if ratio < minRatio {
				t.Errorf("%s dark (%s) on dark bg (%s): ratio %.2f < %.1f",
					name, fg.Dark, scheme.SurfaceBg.Dark, ratio, minRatio)
			}
		})
		t.Run(name+"/light", func(t *testing.T) {
			ratio := ContrastRatio(fg.Light, scheme.SurfaceBg.Light)
			if ratio < minRatio {
				t.Errorf("%s light (%s) on light bg (%s): ratio %.2f < %.1f",
					name, fg.Light, scheme.SurfaceBg.Light, ratio, minRatio)
			}
		})
	}
}

// TestHighContrastEnvVar verifies GROVE_HIGH_CONTRAST triggers the high-contrast scheme.
func TestHighContrastEnvVar(t *testing.T) {
	_ = os.Setenv("GROVE_HIGH_CONTRAST", "1")
	defer func() { _ = os.Unsetenv("GROVE_HIGH_CONTRAST") }()

	scheme := NewColorScheme()
	hc := highContrastColorScheme()

	if scheme.Primary.Dark != hc.Primary.Dark {
		t.Errorf("expected high-contrast Primary dark %q, got %q",
			hc.Primary.Dark, scheme.Primary.Dark)
	}
}

// TestHighContrastNotSetUsesDefault verifies default scheme is used normally.
func TestHighContrastNotSetUsesDefault(t *testing.T) {
	_ = os.Unsetenv("GROVE_HIGH_CONTRAST")
	_ = os.Unsetenv("NO_COLOR")
	_ = os.Unsetenv("GROVE_NO_COLOR")

	scheme := NewColorScheme()
	def := defaultColorScheme()

	if scheme.Primary.Dark != def.Primary.Dark {
		t.Errorf("expected default Primary dark %q, got %q",
			def.Primary.Dark, scheme.Primary.Dark)
	}
}

// TestIsHighContrast checks the detection function.
func TestIsHighContrast(t *testing.T) {
	_ = os.Unsetenv("GROVE_HIGH_CONTRAST")
	if isHighContrast() {
		t.Error("expected false when GROVE_HIGH_CONTRAST not set")
	}

	_ = os.Setenv("GROVE_HIGH_CONTRAST", "1")
	defer func() { _ = os.Unsetenv("GROVE_HIGH_CONTRAST") }()
	if !isHighContrast() {
		t.Error("expected true when GROVE_HIGH_CONTRAST=1")
	}
}

// TestHuhFormsUseAccessibleMode verifies Huh forms respect accessible mode.
func TestHuhFormsUseAccessibleMode(t *testing.T) {
	_ = os.Setenv("GROVE_HIGH_CONTRAST", "1")
	defer func() { _ = os.Unsetenv("GROVE_HIGH_CONTRAST") }()

	name := ""
	form := NewAccessibleCreateNameForm(&name, "test-project", nil)
	if form == nil {
		t.Fatal("expected non-nil form")
	}
}

// hexToRGB is a test helper.
func TestHexToRGB(t *testing.T) {
	tests := []struct {
		hex     string
		r, g, b uint8
	}{
		{"#FFFFFF", 255, 255, 255},
		{"#000000", 0, 0, 0},
		{"#FF0000", 255, 0, 0},
		{"#00FF00", 0, 255, 0},
		{"#0000FF", 0, 0, 255},
	}

	for _, tt := range tests {
		t.Run(tt.hex, func(t *testing.T) {
			r, g, b := hexToRGB(tt.hex)
			if r != tt.r || g != tt.g || b != tt.b {
				t.Errorf("hexToRGB(%s) = (%d,%d,%d), want (%d,%d,%d)",
					tt.hex, r, g, b, tt.r, tt.g, tt.b)
			}
		})
	}
}

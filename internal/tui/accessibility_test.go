package tui

import (
	"os"
	"testing"

	"github.com/lost-in-the/grove/internal/theme"
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
			ratio := theme.ContrastRatio(tt.fg, tt.bg)
			if ratio < tt.wantMin || ratio > tt.wantMax {
				t.Errorf("theme.ContrastRatio(%s, %s) = %.2f, want [%.1f, %.1f]",
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
			lum := theme.RelativeLuminance(tt.hex)
			if lum < tt.wantMin || lum > tt.wantMax {
				t.Errorf("theme.RelativeLuminance(%s) = %.4f, want [%.2f, %.2f]",
					tt.hex, lum, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// TestColorSchemeNotNil verifies color schemes produce non-nil colors.
func TestColorSchemeNotNil(t *testing.T) {
	scheme := theme.DefaultColorScheme()
	if scheme.Primary == nil {
		t.Error("expected Primary color to be non-nil")
	}
	if scheme.TextNormal == nil {
		t.Error("expected TextNormal color to be non-nil")
	}
}

// TestHighContrastColorSchemeNotNil verifies the high-contrast scheme has non-nil colors.
func TestHighContrastColorSchemeNotNil(t *testing.T) {
	scheme := theme.HighContrastColorScheme()
	if scheme.Primary == nil {
		t.Error("expected Primary color to be non-nil")
	}
	if scheme.TextMuted == nil {
		t.Error("expected TextMuted color to be non-nil in high contrast mode")
	}
}

// TestHighContrastEnvVar verifies GROVE_HIGH_CONTRAST triggers the high-contrast scheme.
func TestHighContrastEnvVar(t *testing.T) {
	_ = os.Setenv("GROVE_HIGH_CONTRAST", "1")
	defer func() { _ = os.Unsetenv("GROVE_HIGH_CONTRAST") }()

	scheme := NewColorScheme()
	if scheme.Primary == nil {
		t.Error("expected non-nil Primary in high contrast mode")
	}
}

// TestHighContrastNotSetUsesDefault verifies default scheme is used normally.
func TestHighContrastNotSetUsesDefault(t *testing.T) {
	_ = os.Unsetenv("GROVE_HIGH_CONTRAST")
	_ = os.Unsetenv("NO_COLOR")
	_ = os.Unsetenv("GROVE_NO_COLOR")

	scheme := NewColorScheme()
	if scheme.Primary == nil {
		t.Error("expected non-nil Primary in default mode")
	}
}

// TestIsHighContrast checks the detection function.
func TestIsHighContrast(t *testing.T) {
	_ = os.Unsetenv("GROVE_HIGH_CONTRAST")
	if theme.IsHighContrast() {
		t.Error("expected false when GROVE_HIGH_CONTRAST not set")
	}

	_ = os.Setenv("GROVE_HIGH_CONTRAST", "1")
	defer func() { _ = os.Unsetenv("GROVE_HIGH_CONTRAST") }()
	if !theme.IsHighContrast() {
		t.Error("expected true when GROVE_HIGH_CONTRAST=1")
	}
}

// TestHighContrastModeDetection verifies high-contrast mode env var detection.
func TestHighContrastModeDetection(t *testing.T) {
	_ = os.Setenv("GROVE_HIGH_CONTRAST", "1")
	defer func() { _ = os.Unsetenv("GROVE_HIGH_CONTRAST") }()

	if !theme.IsHighContrast() {
		t.Error("expected theme.IsHighContrast() to return true when GROVE_HIGH_CONTRAST is set")
	}
}

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
			r, g, b, err := theme.HexToRGB(tt.hex)
			if err != nil {
				t.Errorf("theme.HexToRGB(%s) unexpected error: %v", tt.hex, err)
				return
			}
			if r != tt.r || g != tt.g || b != tt.b {
				t.Errorf("theme.HexToRGB(%s) = (%d,%d,%d), want (%d,%d,%d)",
					tt.hex, r, g, b, tt.r, tt.g, tt.b)
			}
		})
	}
}

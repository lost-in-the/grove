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
		{"Secondary", Colors.Secondary, "#38BDF8", "#0284C7"},
		{"Success", Colors.Success, "#34D399", "#059669"},
		{"Warning", Colors.Warning, "#FBBF24", "#D97706"},
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
		{"TextMuted", Colors.TextMuted, "#6C7086", "#64748B"},
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

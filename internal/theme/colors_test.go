package theme

import "testing"

func TestNewColorScheme_Default(t *testing.T) {
	// Test DefaultColorScheme directly to avoid NO_COLOR env interference.
	def := DefaultColorScheme()
	if def.Primary == nil {
		t.Error("expected non-nil Primary in default color scheme")
	}
	if def.Success == nil {
		t.Error("expected non-nil Success in default color scheme")
	}
}

func TestNewColorScheme_NoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	cs := NewColorScheme()
	if cs.Primary != nil {
		t.Errorf("expected nil Primary in NO_COLOR mode, got %v", cs.Primary)
	}
}

func TestNewColorScheme_GroveNoColor(t *testing.T) {
	t.Setenv("GROVE_NO_COLOR", "1")

	cs := NewColorScheme()
	if cs.Primary != nil {
		t.Errorf("expected nil Primary in GROVE_NO_COLOR mode, got %v", cs.Primary)
	}
}

func TestNewColorScheme_HighContrast(t *testing.T) {
	// Test HighContrastColorScheme directly.
	hc := HighContrastColorScheme()
	if hc.Primary == nil {
		t.Error("expected non-nil Primary in high-contrast color scheme")
	}
}

func TestIsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if !IsNoColor() {
		t.Error("expected true when NO_COLOR is set")
	}
}

func TestIsHighContrast(t *testing.T) {
	t.Setenv("GROVE_HIGH_CONTRAST", "1")
	if !IsHighContrast() {
		t.Error("expected true when GROVE_HIGH_CONTRAST=1")
	}
}

func TestNoColorScheme_Empty(t *testing.T) {
	cs := NoColorScheme()
	if cs.Primary != nil {
		t.Errorf("expected nil Primary in NoColorScheme, got %v", cs.Primary)
	}
	if cs.Success != nil {
		t.Errorf("expected nil Success in NoColorScheme, got %v", cs.Success)
	}
}

func TestAdaptiveColor_DarkMode(t *testing.T) {
	t.Setenv("GROVE_LIGHT_MODE", "0")
	c := AdaptiveColor("#111111", "#EEEEEE")
	if c == nil {
		t.Fatal("expected non-nil color")
	}
}

func TestAdaptiveColor_LightMode(t *testing.T) {
	t.Setenv("GROVE_LIGHT_MODE", "1")
	c := AdaptiveColor("#111111", "#EEEEEE")
	if c == nil {
		t.Fatal("expected non-nil color")
	}
}

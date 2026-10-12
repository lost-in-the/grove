package theme

import "testing"

func TestHexToRGB(t *testing.T) {
	tests := []struct {
		hex     string
		r, g, b uint8
		wantErr bool
	}{
		{"#FFFFFF", 255, 255, 255, false},
		{"#000000", 0, 0, 0, false},
		{"#FF0000", 255, 0, 0, false},
		{"#00FF00", 0, 255, 0, false},
		{"#0000FF", 0, 0, 255, false},
		{"invalid", 0, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.hex, func(t *testing.T) {
			r, g, b, err := HexToRGB(tt.hex)
			if tt.wantErr {
				if err == nil {
					t.Errorf("HexToRGB(%s) expected error, got nil", tt.hex)
				}
				return
			}
			if err != nil {
				t.Errorf("HexToRGB(%s) unexpected error: %v", tt.hex, err)
				return
			}
			if r != tt.r || g != tt.g || b != tt.b {
				t.Errorf("HexToRGB(%s) = (%d,%d,%d), want (%d,%d,%d)",
					tt.hex, r, g, b, tt.r, tt.g, tt.b)
			}
		})
	}
}

func TestRelativeLuminance(t *testing.T) {
	tests := []struct {
		name    string
		hex     string
		wantMin float64
		wantMax float64
	}{
		{"Black", "#000000", -0.01, 0.01},
		{"White", "#FFFFFF", 0.99, 1.01},
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

func TestContrastRatio(t *testing.T) {
	// Black on white should be ~21:1
	ratio := ContrastRatio("#000000", "#FFFFFF")
	if ratio < 20.9 || ratio > 21.1 {
		t.Errorf("ContrastRatio(black, white) = %.2f, want ~21.0", ratio)
	}

	// Same color should be 1:1
	ratio = ContrastRatio("#808080", "#808080")
	if ratio < 0.9 || ratio > 1.1 {
		t.Errorf("ContrastRatio(same, same) = %.2f, want ~1.0", ratio)
	}

	// Order shouldn't matter
	r1 := ContrastRatio("#000000", "#FFFFFF")
	r2 := ContrastRatio("#FFFFFF", "#000000")
	if r1 != r2 {
		t.Errorf("ContrastRatio should be order-independent: %f != %f", r1, r2)
	}
}

// Every text color reads at 4.5:1 or better on every background it is drawn
// on: the surface, the selected row, and the header bar (spec 5.9).
func TestPalette_TextContrast(t *testing.T) {
	const minRatio = 4.5
	for pname, p := range map[string]Palette{"default": DefaultPalette, "high-contrast": HighContrastPalette} {
		texts := map[string]Pair{
			"Primary": p.Primary, "Secondary": p.Secondary,
			"Success": p.Success, "Warning": p.Warning, "Danger": p.Danger, "Info": p.Info,
			"TextNormal": p.TextNormal, "TextBright": p.TextBright, "TextMuted": p.TextMuted, "TextDim": p.TextDim,
		}
		bgs := map[string]Pair{"SurfaceBg": p.SurfaceBg, "SelectionBg": p.SelectionBg, "HeaderBg": p.HeaderBg}
		for tname, fg := range texts {
			for bname, bg := range bgs {
				for mode, pair := range map[string][2]string{"dark": {fg.Dark, bg.Dark}, "light": {fg.Light, bg.Light}} {
					if r := ContrastRatio(pair[0], pair[1]); r < minRatio {
						t.Errorf("%s %s: %s on %s = %.2f:1, want %.1f", pname, mode, tname, bname, r, minRatio)
					}
				}
			}
		}
	}
}

// Config form buttons put text on Primary (focused) and SurfaceBorder
// (blurred); those pairs read at 4.5:1 too.
func TestPalette_ButtonContrast(t *testing.T) {
	for pname, p := range map[string]Palette{"default": DefaultPalette, "high-contrast": HighContrastPalette} {
		for name, pair := range map[string][2]Pair{
			"SurfaceBg on Primary":        {p.SurfaceBg, p.Primary},
			"TextNormal on SurfaceBorder": {p.TextNormal, p.SurfaceBorder},
		} {
			for mode, hex := range map[string][2]string{"dark": {pair[0].Dark, pair[1].Dark}, "light": {pair[0].Light, pair[1].Light}} {
				if r := ContrastRatio(hex[0], hex[1]); r < 4.5 {
					t.Errorf("%s %s: %s = %.2f:1", pname, mode, name, r)
				}
			}
		}
	}
}

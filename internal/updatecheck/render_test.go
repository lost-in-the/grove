package updatecheck

import (
	"strings"
	"testing"
)

func TestCompareSemver(t *testing.T) {
	cases := []struct {
		current, latest string
		want            Severity
	}{
		{"0.5.0", "0.6.0", SeverityMinor},
		{"0.5.0", "1.0.0", SeverityMajor},
		{"0.5.0", "0.5.1", SeverityPatch},
		{"0.5.0", "0.5.0", SeverityNone},
		{"0.5.0", "0.4.99", SeverityNone},
		{"v0.5.0", "v0.6.0", SeverityMinor},
		{"abc", "0.6.0", SeverityNone},
		{"0.5.0", "abc", SeverityNone},
	}
	for _, tc := range cases {
		t.Run(tc.current+"->"+tc.latest, func(t *testing.T) {
			if got := CompareSemver(tc.current, tc.latest); got != tc.want {
				t.Errorf("CompareSemver(%q,%q) = %v, want %v", tc.current, tc.latest, got, tc.want)
			}
		})
	}
}

func TestParseSemver_EdgeCases(t *testing.T) {
	cases := []struct {
		in string
		ok bool
	}{
		{"", false},
		{"0.5", false},       // 2 parts
		{"0.5.0.0", false},   // 4 parts
		{"-1.2.3", false},    // negative (Atoi accepts -1, but spec wants real-world semver)
		{"0.5.0-dev", false}, // pre-release suffix
		{"0.5.0", true},
		{"v0.5.0", true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			_, got := parseSemver(tc.in)
			if got != tc.ok {
				t.Errorf("parseSemver(%q) ok=%v, want %v", tc.in, got, tc.ok)
			}
		})
	}
}

func TestRenderBox_Plain(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	out := RenderBox("0.5.0", "0.6.0",
		"https://github.com/lost-in-the/grove/releases/tag/v0.6.0",
		"brew upgrade lost-in-the/tap/grove",
	)
	// Plain mode: no ANSI escape codes
	if strings.Contains(out, "\x1b[") {
		t.Errorf("expected no ANSI escape sequences in plain output, got:\n%s", out)
	}
	mustContain(t, out, "Update available")
	mustContain(t, out, "0.5.0")
	mustContain(t, out, "0.6.0")
	mustContain(t, out, "brew upgrade lost-in-the/tap/grove")
	mustContain(t, out, "github.com/lost-in-the/grove/releases/tag/v0.6.0")
}

func TestRenderBox_ColoredEmitsAnsi(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "1") // hint to lipgloss to render colors even when not a TTY
	out := RenderBox("0.5.0", "1.0.0", "https://x", "brew upgrade x")
	if !strings.Contains(out, "\x1b[") {
		t.Errorf("expected ANSI escape sequences in colored output, got: %q", out)
	}
}

func TestRenderBox_PlainPaddingAlignsForUTF8(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	out := RenderBox("0.5.0", "0.6.0",
		"https://x",
		"brew upgrade x",
	)
	// Verify each line between the top and bottom borders has identical visual width.
	// In plain mode, lines are ASCII-only after the arrow swap, so byte length == rune count
	// and we can compare line lengths directly.
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected >= 3 lines in box output, got: %q", out)
	}
	want := len(lines[0])
	for i, l := range lines {
		if len(l) != want {
			t.Errorf("line %d width %d, want %d (line=%q)", i, len(l), want, l)
		}
	}
}

func mustContain(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected %q in output, got:\n%s", needle, haystack)
	}
}

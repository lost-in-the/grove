package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

const previewSample = "# Fix login\n\nThis **fixes** the redirect loop.\n\n- first\n- second\n"

func TestRenderMarkdown_NoColorHasNoEscapes(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	out := renderMarkdown(previewSample, 60)
	if strings.Contains(out, "\x1b[") {
		t.Fatalf("NO_COLOR output contains escape codes: %q", out)
	}
	for _, want := range []string{"Fix login", "fixes", "redirect loop", "first", "second"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRenderMarkdown_ColorKeepsText(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("GROVE_NO_COLOR", "")
	plain := ansi.Strip(renderMarkdown(previewSample, 60))
	for _, want := range []string{"Fix login", "fixes", "redirect loop", "first", "second"} {
		if !strings.Contains(plain, want) {
			t.Errorf("output missing %q:\n%s", want, plain)
		}
	}
}

func TestRenderMarkdown_WrapsToWidth(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	long := strings.Repeat("word ", 40)
	for _, line := range strings.Split(renderMarkdown(long, 30), "\n") {
		if w := ansi.StringWidth(line); w > 30 {
			t.Errorf("line width %d exceeds 30: %q", w, line)
		}
	}
}

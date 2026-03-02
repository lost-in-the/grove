package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestSuccess_NoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	Success(w, "operation completed")
	if got := buf.String(); got != "✓ operation completed\n" {
		t.Errorf("got %q, want %q", got, "✓ operation completed\n")
	}
}

func TestWarning_NoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	Warning(w, "something odd: %s", "details")
	if got := buf.String(); !strings.Contains(got, "⚠ something odd: details") {
		t.Errorf("got %q, want warning message", got)
	}
}

func TestError_NoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	Error(w, "failed: %v", "reason")
	if got := buf.String(); !strings.Contains(got, "✗ failed: reason") {
		t.Errorf("got %q, want error message", got)
	}
}

func TestInfo_NoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	Info(w, "note: %s", "info")
	if got := buf.String(); !strings.Contains(got, "ℹ note: info") {
		t.Errorf("got %q, want info message", got)
	}
}

func TestHeader_NoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	Header(w, "grove doctor")
	got := buf.String()
	if !strings.Contains(got, "grove doctor\n") {
		t.Errorf("missing title in %q", got)
	}
	if !strings.Contains(got, "━") {
		t.Errorf("missing separator in %q", got)
	}
}

func TestStep_NoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	Step(w, "doing stuff")
	if got := buf.String(); got != "→ doing stuff\n" {
		t.Errorf("got %q", got)
	}
}

func TestStatusText_NoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	w := NewWriter(&bytes.Buffer{}, false)
	got := StatusText(w, "clean", "clean")
	if got != "clean" {
		t.Errorf("expected plain text, got %q", got)
	}
}

func TestLabel_NoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	Label(w, "Path:", "/some/path")
	if got := buf.String(); !strings.Contains(got, "Path: /some/path") {
		t.Errorf("got %q", got)
	}
}

func TestFaint_NoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	Faint(w, "muted text")
	if got := buf.String(); got != "muted text\n" {
		t.Errorf("got %q, want %q", got, "muted text\n")
	}
}

func TestBold_NoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	Bold(w, "bold text")
	if got := buf.String(); got != "bold text\n" {
		t.Errorf("got %q, want %q", got, "bold text\n")
	}
}

func TestAccent_NoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	w := NewWriter(&bytes.Buffer{}, false)
	got := Accent(w, "accented")
	if got != "accented" {
		t.Errorf("expected plain text, got %q", got)
	}
}

func TestStatusText_AllCategories(t *testing.T) {
	// Use a color-enabled writer (isTTY=true) without NO_COLOR to exercise the
	// styled code paths. We assert strings.Contains so ANSI wrappers don't break the check.
	w := NewWriter(&bytes.Buffer{}, true)

	tests := []struct {
		status StatusLevel
		text   string
	}{
		// success
		{StatusClean, "clean"},
		{StatusOK, "ok"},
		{StatusActive, "active"},
		{StatusAttached, "attached"},
		// warning
		{StatusDirty, "dirty"},
		{StatusWarning, "warning"},
		{StatusDetached, "detached"},
		// danger
		{StatusStale, "stale"},
		{StatusError, "error"},
		{StatusFail, "fail"},
		// muted
		{StatusInfo, "info"},
		{StatusNone, "none"},
		// unknown — returns plain text unchanged
		{"unknown-status", "some text"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			got := StatusText(w, tt.status, tt.text)
			if !strings.Contains(got, tt.text) {
				t.Errorf("StatusText(%q, %q) = %q, want output containing %q", tt.status, tt.text, got, tt.text)
			}
		})
	}
}

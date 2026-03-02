package cli

import (
	"bytes"
	"testing"
)

func TestNewWriter_TTY(t *testing.T) {
	w := NewWriter(&bytes.Buffer{}, true)
	if !w.IsTTY() {
		t.Error("expected IsTTY() = true")
	}
}

func TestNewWriter_NonTTY(t *testing.T) {
	w := NewWriter(&bytes.Buffer{}, false)
	if w.IsTTY() {
		t.Error("expected IsTTY() = false")
	}
}

func TestWriter_UseColor_NonTTY(t *testing.T) {
	w := NewWriter(&bytes.Buffer{}, false)
	if w.UseColor() {
		t.Error("expected UseColor() = false for non-TTY")
	}
}

func TestWriter_UseColor_NoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	w := NewWriter(&bytes.Buffer{}, true)
	if w.UseColor() {
		t.Error("expected UseColor() = false when NO_COLOR is set")
	}
}

func TestWriter_Write(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, false)
	n, err := w.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 5 {
		t.Errorf("expected 5 bytes written, got %d", n)
	}
	if buf.String() != "hello" {
		t.Errorf("expected 'hello', got %q", buf.String())
	}
}

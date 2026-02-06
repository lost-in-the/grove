package tui

import (
	"testing"
	"time"
)

func TestGroveSpinner(t *testing.T) {
	s := GroveSpinner()

	if len(s.Spinner.Frames) != 10 {
		t.Errorf("expected 10 spinner frames, got %d", len(s.Spinner.Frames))
	}

	if s.Spinner.FPS != 80*time.Millisecond {
		t.Errorf("expected 80ms FPS, got %v", s.Spinner.FPS)
	}
}

func TestToastOpacity_Fresh(t *testing.T) {
	toast := &Toast{
		Message:   "test",
		Level:     ToastSuccess,
		Duration:  3 * time.Second,
		CreatedAt: time.Now(),
	}

	style := ToastOpacity(toast)
	// Fresh toast should not be faint
	if style.GetFaint() {
		t.Error("fresh toast should not be faint")
	}
}

func TestToastOpacity_FadeWindow(t *testing.T) {
	toast := &Toast{
		Message:   "test",
		Level:     ToastSuccess,
		Duration:  3 * time.Second,
		CreatedAt: time.Now().Add(-2500 * time.Millisecond), // 500ms remaining, within 800ms fade window
	}

	style := ToastOpacity(toast)
	if !style.GetFaint() {
		t.Error("toast in fade window should be faint")
	}
}

func TestToastOpacity_Expired(t *testing.T) {
	toast := &Toast{
		Message:   "test",
		Level:     ToastSuccess,
		Duration:  3 * time.Second,
		CreatedAt: time.Now().Add(-4 * time.Second), // expired
	}

	style := ToastOpacity(toast)
	if !style.GetFaint() {
		t.Error("expired toast should be faint")
	}
}

func TestToastOpacity_Nil(t *testing.T) {
	style := ToastOpacity(nil)
	// Should not panic and return a valid style
	if style.GetFaint() {
		t.Error("nil toast should return non-faint style")
	}
}

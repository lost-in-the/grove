package tui

import (
	"strings"
	"testing"
	"time"
)

func TestToastLevelIcon(t *testing.T) {
	tests := []struct {
		name  string
		level ToastLevel
		want  string
	}{
		{"Success", ToastSuccess, "✓"},
		{"Warning", ToastWarning, "⚠"},
		{"Error", ToastError, "✗"},
		{"Info", ToastInfo, "ℹ"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.level.Icon(); got != tt.want {
				t.Errorf("Icon() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewToast(t *testing.T) {
	toast := NewToast("Created worktree", ToastSuccess)

	if toast.Message != "Created worktree" {
		t.Errorf("Message = %q, want %q", toast.Message, "Created worktree")
	}
	if toast.Level != ToastSuccess {
		t.Errorf("Level = %v, want %v", toast.Level, ToastSuccess)
	}
	if toast.Duration != DefaultToastDuration {
		t.Errorf("Duration = %v, want %v", toast.Duration, DefaultToastDuration)
	}
	if toast.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestNewToastWithDuration(t *testing.T) {
	toast := NewToastWithDuration("msg", ToastError, 5*time.Second)

	if toast.Duration != 5*time.Second {
		t.Errorf("Duration = %v, want %v", toast.Duration, 5*time.Second)
	}
}

func TestToastExpired(t *testing.T) {
	tests := []struct {
		name        string
		age         time.Duration
		duration    time.Duration
		wantExpired bool
	}{
		{"Fresh toast not expired", 0, 3 * time.Second, false},
		{"Toast expires after duration", 4 * time.Second, 3 * time.Second, true},
		{"Toast at exact boundary is expired", 3 * time.Second, 3 * time.Second, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toast := &Toast{
				Message:   "test",
				Level:     ToastSuccess,
				Duration:  tt.duration,
				CreatedAt: time.Now().Add(-tt.age),
			}
			if got := toast.Expired(); got != tt.wantExpired {
				t.Errorf("Expired() = %v, want %v", got, tt.wantExpired)
			}
		})
	}
}

func TestToastModelShow(t *testing.T) {
	tm := NewToastModel()
	if tm.Current != nil {
		t.Fatal("new ToastModel should have nil Current")
	}

	toast := NewToast("hello", ToastInfo)
	tm.Show(toast)

	if tm.Current == nil {
		t.Fatal("Current should not be nil after Show")
	}
	if tm.Current.Message != "hello" {
		t.Errorf("Message = %q, want %q", tm.Current.Message, "hello")
	}
}

func TestToastModelShowReplacesExisting(t *testing.T) {
	tm := NewToastModel()
	tm.Show(NewToast("first", ToastInfo))
	tm.Show(NewToast("second", ToastWarning))

	if tm.Current == nil {
		t.Fatal("Current should not be nil")
	}
	if tm.Current.Message != "second" {
		t.Errorf("Message = %q, want %q", tm.Current.Message, "second")
	}
}

func TestToastModelTickClearsExpired(t *testing.T) {
	tm := NewToastModel()

	// Tick with no toast is a no-op
	tm.Tick()
	if tm.Current != nil {
		t.Error("Tick on empty model should leave Current nil")
	}

	// Show a toast with very short duration
	tm.Show(NewToastWithDuration("ephemeral", ToastSuccess, 1*time.Millisecond))
	if tm.Current == nil {
		t.Fatal("Current should not be nil after Show")
	}

	// Wait for expiry
	time.Sleep(5 * time.Millisecond)
	tm.Tick()
	if tm.Current != nil {
		t.Error("expired toast should be cleared by Tick")
	}
}

func TestToastModelTickKeepsFresh(t *testing.T) {
	tm := NewToastModel()
	tm.Show(NewToast("fresh", ToastSuccess))
	tm.Tick()
	if tm.Current == nil {
		t.Error("fresh toast should survive Tick")
	}
}

func TestToastModelDismiss(t *testing.T) {
	tm := NewToastModel()
	tm.Show(NewToast("msg", ToastInfo))
	tm.Dismiss()
	if tm.Current != nil {
		t.Error("Dismiss should clear Current")
	}
}

func TestToastViewNil(t *testing.T) {
	tm := NewToastModel()
	view := tm.View(80)
	if view != "" {
		t.Errorf("View with no toast should be empty, got %q", view)
	}
}

func TestToastViewContainsContent(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		level    ToastLevel
		contains []string
	}{
		{"Success", "Created worktree", ToastSuccess, []string{"✓", "Created worktree"}},
		{"Error", "Failed to delete", ToastError, []string{"✗", "Failed to delete"}},
		{"Warning", "Worktree has changes", ToastWarning, []string{"⚠", "Worktree has changes"}},
		{"Info", "Loading PRs", ToastInfo, []string{"ℹ", "Loading PRs"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm := NewToastModel()
			tm.Show(NewToast(tt.message, tt.level))
			view := tm.View(80)
			for _, s := range tt.contains {
				if !strings.Contains(view, s) {
					t.Errorf("View should contain %q, got:\n%s", s, view)
				}
			}
		})
	}
}

func TestToastViewRightAligned(t *testing.T) {
	tm := NewToastModel()
	tm.Show(NewToast("short", ToastSuccess))
	view := tm.View(80)

	if view == "" {
		t.Fatal("view should not be empty")
	}
	// The view should have leading spaces (right-aligned)
	if !strings.HasPrefix(view, " ") {
		t.Error("toast view should be right-aligned with leading spaces")
	}
}

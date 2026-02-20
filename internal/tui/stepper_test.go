package tui

import (
	"strings"
	"testing"
)

func TestNewStepper(t *testing.T) {
	s := NewStepper("Name", "Branch", "Create")

	if s.Current != 0 {
		t.Errorf("NewStepper Current = %d, want 0", s.Current)
	}
	if len(s.Steps) != 3 {
		t.Errorf("NewStepper Steps count = %d, want 3", len(s.Steps))
	}
	if s.Steps[0] != "Name" {
		t.Errorf("NewStepper Steps[0] = %q, want %q", s.Steps[0], "Name")
	}
}

func TestStepperAdvance(t *testing.T) {
	tests := []struct {
		name        string
		steps       []string
		advances    int
		wantCurrent int
	}{
		{"advance once", []string{"A", "B", "C"}, 1, 1},
		{"advance twice", []string{"A", "B", "C"}, 2, 2},
		{"advance past end clamps", []string{"A", "B", "C"}, 5, 2},
		{"advance zero stays", []string{"A", "B"}, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStepper(tt.steps...)
			for i := 0; i < tt.advances; i++ {
				s.Advance()
			}
			if s.Current != tt.wantCurrent {
				t.Errorf("Current = %d, want %d", s.Current, tt.wantCurrent)
			}
		})
	}
}

func TestStepperBack(t *testing.T) {
	tests := []struct {
		name        string
		steps       []string
		initial     int
		backs       int
		wantCurrent int
	}{
		{"back from 1", []string{"A", "B", "C"}, 1, 1, 0},
		{"back from 2", []string{"A", "B", "C"}, 2, 1, 1},
		{"back past start clamps", []string{"A", "B", "C"}, 1, 5, 0},
		{"back from 0 stays", []string{"A", "B"}, 0, 1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStepper(tt.steps...)
			s.Current = tt.initial
			for i := 0; i < tt.backs; i++ {
				s.Back()
			}
			if s.Current != tt.wantCurrent {
				t.Errorf("Current = %d, want %d", s.Current, tt.wantCurrent)
			}
		})
	}
}

func TestStepperIsComplete(t *testing.T) {
	s := NewStepper("A", "B", "C")
	if s.IsComplete(0) {
		t.Error("step 0 should not be complete when current is 0")
	}
	s.Current = 2
	if !s.IsComplete(0) {
		t.Error("step 0 should be complete when current is 2")
	}
	if !s.IsComplete(1) {
		t.Error("step 1 should be complete when current is 2")
	}
	if s.IsComplete(2) {
		t.Error("current step should not be complete")
	}
}

func TestStepperIsCurrent(t *testing.T) {
	s := NewStepper("A", "B", "C")
	s.Current = 1
	if s.IsCurrent(0) {
		t.Error("step 0 should not be current")
	}
	if !s.IsCurrent(1) {
		t.Error("step 1 should be current")
	}
}

func TestStepperView(t *testing.T) {
	tests := []struct {
		name       string
		steps      []string
		current    int
		width      int
		wantLabels []string
	}{
		{
			name:       "three steps at start",
			steps:      []string{"Name", "Branch", "Create"},
			current:    0,
			width:      60,
			wantLabels: []string{"Name", "Branch", "Create"},
		},
		{
			name:       "middle step",
			steps:      []string{"Name", "Branch", "Create"},
			current:    1,
			width:      60,
			wantLabels: []string{"Name", "Branch", "Create"},
		},
		{
			name:       "two steps",
			steps:      []string{"A", "B"},
			current:    0,
			width:      40,
			wantLabels: []string{"A", "B"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStepper(tt.steps...)
			s.Current = tt.current
			view := s.View(tt.width)
			for _, label := range tt.wantLabels {
				if !strings.Contains(view, label) {
					t.Errorf("View missing label %q:\n%s", label, view)
				}
			}
		})
	}
}

func TestStepperViewContainsDots(t *testing.T) {
	s := NewStepper("A", "B", "C")
	view := s.View(60)
	// Should contain step indicator dots (● for current/complete, ○ for future)
	if !strings.Contains(view, "●") && !strings.Contains(view, "○") {
		t.Errorf("View should contain step indicator dots:\n%s", view)
	}
}

func TestStepperViewConnectors(t *testing.T) {
	s := NewStepper("A", "B", "C")
	view := s.View(60)
	// Should contain connector lines between steps
	if !strings.Contains(view, "━") {
		t.Errorf("View should contain connector lines:\n%s", view)
	}
}

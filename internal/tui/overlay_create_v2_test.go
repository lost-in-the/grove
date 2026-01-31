package tui

import (
	"strings"
	"testing"
)

func TestRenderContextSummary(t *testing.T) {
	tests := []struct {
		name     string
		state    *CreateState
		width    int
		wantStrs []string
	}{
		{
			name: "shows name and full name",
			state: &CreateState{
				Name:        "feature-auth",
				ProjectName: "acupoll",
			},
			width:    60,
			wantStrs: []string{"feature-auth", "acupoll-feature-auth"},
		},
		{
			name: "shows branch when set",
			state: &CreateState{
				Name:        "feature-auth",
				ProjectName: "acupoll",
				BaseBranch:  "develop",
			},
			width:    60,
			wantStrs: []string{"feature-auth", "acupoll-feature-auth", "develop"},
		},
		{
			name: "empty name produces summary header",
			state: &CreateState{
				Name:        "",
				ProjectName: "acupoll",
			},
			width:    60,
			wantStrs: []string{"Summary"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderContextSummary(tt.state, tt.width)
			for _, want := range tt.wantStrs {
				if !strings.Contains(got, want) {
					t.Errorf("renderContextSummary() missing %q in output:\n%s", want, got)
				}
			}
		})
	}
}

func TestRenderCreateBranchV2(t *testing.T) {
	tests := []struct {
		name     string
		state    *CreateState
		width    int
		wantStrs []string
	}{
		{
			name: "branch step shows context summary from step 1",
			state: &CreateState{
				Step:        CreateStepBranch,
				Name:        "feature-auth",
				ProjectName: "acupoll",
			},
			width: 80,
			wantStrs: []string{
				"feature-auth",
				"acupoll-feature-auth",
				"Create new branch",
				"From existing branch",
			},
		},
		{
			name: "branch step shows stepper labels",
			state: &CreateState{
				Step:        CreateStepBranch,
				Name:        "my-feature",
				ProjectName: "grove-cli",
			},
			width:    80,
			wantStrs: []string{"Name", "Branch"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderCreateBranchV2(tt.state, tt.width)
			for _, want := range tt.wantStrs {
				if !strings.Contains(got, want) {
					t.Errorf("renderCreateBranchV2() missing %q in output:\n%s", want, got)
				}
			}
		})
	}
}

func TestRenderCreateNameV2(t *testing.T) {
	tests := []struct {
		name     string
		state    *CreateState
		width    int
		wantStrs []string
	}{
		{
			name: "name step shows stepper and input",
			state: &CreateState{
				Step:        CreateStepName,
				Name:        "test",
				ProjectName: "acupoll",
			},
			width:    80,
			wantStrs: []string{"Name", "test", "acupoll-test"},
		},
		{
			name: "name step shows error",
			state: &CreateState{
				Step:        CreateStepName,
				Name:        "bad name!",
				ProjectName: "acupoll",
				Error:       "invalid characters",
			},
			width:    80,
			wantStrs: []string{"invalid characters"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderCreateNameV2(tt.state, tt.width)
			for _, want := range tt.wantStrs {
				if !strings.Contains(got, want) {
					t.Errorf("renderCreateNameV2() missing %q in output:\n%s", want, got)
				}
			}
		})
	}
}

func TestRenderCreateV2Dispatch(t *testing.T) {
	tests := []struct {
		name     string
		state    *CreateState
		wantStrs []string
	}{
		{
			name: "dispatches to name step",
			state: &CreateState{
				Step:        CreateStepName,
				ProjectName: "proj",
			},
			wantStrs: []string{"Name"},
		},
		{
			name: "dispatches to branch step with context",
			state: &CreateState{
				Step:        CreateStepBranch,
				Name:        "feat",
				ProjectName: "proj",
			},
			wantStrs: []string{"feat", "proj-feat"},
		},
		{
			name: "dispatches to creating spinner",
			state: &CreateState{
				Creating: true,
				Name:     "feat",
			},
			wantStrs: []string{"Creating", "feat"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderCreateV2(tt.state, 80, "⠋")
			for _, want := range tt.wantStrs {
				if !strings.Contains(got, want) {
					t.Errorf("renderCreateV2() missing %q in output:\n%s", want, got)
				}
			}
		})
	}
}

func TestBackspacePreservesValues(t *testing.T) {
	s := &CreateState{
		Step:        CreateStepBranch,
		Name:        "my-feature",
		ProjectName: "acupoll",
	}

	// Simulate going back
	s.Step = CreateStepName

	if s.Name != "my-feature" {
		t.Errorf("Name not preserved: got %q, want %q", s.Name, "my-feature")
	}
	if s.ProjectName != "acupoll" {
		t.Errorf("ProjectName not preserved: got %q, want %q", s.ProjectName, "acupoll")
	}

	view := renderCreateNameV2(s, 80)
	if !strings.Contains(view, "my-feature") {
		t.Errorf("name step should show preserved name, got:\n%s", view)
	}
}

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

func TestRenderCreateBranchSelectorV2(t *testing.T) {
	tests := []struct {
		name     string
		state    *CreateState
		width    int
		wantStrs []string
	}{
		{
			name: "branch selector shows branches",
			state: &CreateState{
				Step:              CreateStepBranch,
				Branches:          []string{"main", "develop", "feature/auth"},
				BranchFilterInput: newBranchFilterInput(),
			},
			width: 80,
			wantStrs: []string{
				"main",
				"develop",
				"Branch",
			},
		},
		{
			name:  "branch selector with filter shows create new",
			state: createStateWithBranchFilter([]string{"main", "develop"}, "my-feat"),
			width: 80,
			wantStrs: []string{
				"Create new branch",
				"my-feat",
				"Filter",
			},
		},
		{
			name: "branch selector shows stepper labels",
			state: &CreateState{
				Step:              CreateStepBranch,
				Branches:          []string{"main"},
				BranchFilterInput: newBranchFilterInput(),
			},
			width:    80,
			wantStrs: []string{"Branch", "Name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderCreateBranchSelectorV2(tt.state, tt.width)
			for _, want := range tt.wantStrs {
				if !strings.Contains(got, want) {
					t.Errorf("renderCreateBranchSelectorV2() missing %q in output:\n%s", want, got)
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
			name:     "name step shows stepper and input",
			state:    createStateWithName("test", "acupoll"),
			width:    80,
			wantStrs: []string{"Name", "test", "acupoll-test"},
		},
		{
			name: "name step shows error",
			state: func() *CreateState {
				s := createStateWithName("bad name!", "acupoll")
				s.Error = "invalid characters"
				return s
			}(),
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
			state: func() *CreateState {
				s := createStateWithName("", "proj")
				return s
			}(),
			wantStrs: []string{"Name"},
		},
		{
			name: "dispatches to branch step",
			state: &CreateState{
				Step:              CreateStepBranch,
				Branches:          []string{"main", "develop"},
				BranchFilterInput: newBranchFilterInput(),
			},
			wantStrs: []string{"Branch", "main"},
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
	s := createStateWithName("my-feature", "acupoll")
	s.BaseBranch = "develop"

	// Simulate going back to branch step
	s.Step = CreateStepBranch

	if s.Name != "my-feature" {
		t.Errorf("Name not preserved: got %q, want %q", s.Name, "my-feature")
	}
	if s.ProjectName != "acupoll" {
		t.Errorf("ProjectName not preserved: got %q, want %q", s.ProjectName, "acupoll")
	}
	if s.BaseBranch != "develop" {
		t.Errorf("BaseBranch not preserved: got %q, want %q", s.BaseBranch, "develop")
	}

	// Navigate forward again — values should still be there
	s.Step = CreateStepName
	view := renderCreateNameV2(s, 80)
	if !strings.Contains(view, "my-feature") {
		t.Errorf("name step should show preserved name, got:\n%s", view)
	}
}

// createStateWithBranchFilter creates a CreateState with a pre-set branch filter.
func createStateWithBranchFilter(branches []string, filter string) *CreateState {
	bfi := newBranchFilterInput()
	bfi.SetValue(filter)
	return &CreateState{
		Step:              CreateStepBranch,
		Branches:          branches,
		BranchFilter:      filter,
		BranchFilterInput: bfi,
	}
}

// createStateWithName creates a CreateState at the name step with a pre-set name.
func createStateWithName(name, projectName string) *CreateState {
	ni := newNameInput("")
	ni.SetValue(name)
	return &CreateState{
		Step:        CreateStepName,
		Name:        name,
		ProjectName: projectName,
		NameInput:   ni,
	}
}

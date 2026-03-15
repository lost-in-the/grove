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

func TestRenderCreateBranchChoiceV2(t *testing.T) {
	state := &CreateState{
		Step:              CreateStepBranchChoice,
		Branches:          []string{"main", "develop"},
		BranchFilterInput: newBranchFilterInput(),
		BranchNameInput:   newBranchNameInput(),
	}
	got := renderCreateBranchChoiceV2(state, 80)

	for _, want := range []string{"Select an existing branch", "Create a new branch", "Branch"} {
		if !strings.Contains(got, want) {
			t.Errorf("renderCreateBranchChoiceV2() missing %q in output:\n%s", want, got)
		}
	}
}

func TestRenderCreateBranchSelectV2(t *testing.T) {
	tests := []struct {
		name     string
		state    *CreateState
		width    int
		wantStrs []string
	}{
		{
			name: "shows branch list",
			state: &CreateState{
				Step:              CreateStepBranchSelect,
				Branches:          []string{"main", "develop", "feature/auth"},
				BranchFilterInput: newBranchFilterInput(),
			},
			width:    80,
			wantStrs: []string{"main", "develop", "Branch"},
		},
		{
			name: "shows filter when active",
			state: func() *CreateState {
				bfi := newBranchFilterInput()
				bfi.SetValue("feat")
				return &CreateState{
					Step:              CreateStepBranchSelect,
					Branches:          []string{"main", "develop", "feature/auth"},
					BranchFilterInput: bfi,
					BranchFilterMode:  BranchFilterOn,
				}
			}(),
			width:    80,
			wantStrs: []string{"feature/auth", "Filter"},
		},
		{
			name: "shows stepper labels",
			state: &CreateState{
				Step:              CreateStepBranchSelect,
				Branches:          []string{"main"},
				BranchFilterInput: newBranchFilterInput(),
			},
			width:    80,
			wantStrs: []string{"Branch", "Name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderCreateBranchSelectV2(tt.state, tt.width)
			for _, want := range tt.wantStrs {
				if !strings.Contains(got, want) {
					t.Errorf("renderCreateBranchSelectV2() missing %q in output:\n%s", want, got)
				}
			}
		})
	}
}

func TestRenderCreateBranchCreateV2(t *testing.T) {
	state := &CreateState{
		Step:            CreateStepBranchCreate,
		BranchNameInput: newBranchNameInput(),
	}
	got := renderCreateBranchCreateV2(state, 80)

	for _, want := range []string{"Enter a name", "Branch"} {
		if !strings.Contains(got, want) {
			t.Errorf("renderCreateBranchCreateV2() missing %q in output:\n%s", want, got)
		}
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
			name: "dispatches to branch choice step",
			state: &CreateState{
				Step:              CreateStepBranchChoice,
				Branches:          []string{"main", "develop"},
				BranchFilterInput: newBranchFilterInput(),
				BranchNameInput:   newBranchNameInput(),
			},
			wantStrs: []string{"Select an existing branch"},
		},
		{
			name: "dispatches to branch select step",
			state: &CreateState{
				Step:              CreateStepBranchSelect,
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

	// Simulate going back to branch choice step
	s.Step = CreateStepBranchChoice

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

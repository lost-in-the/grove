package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/huh"
)

func TestNewCreateNameForm(t *testing.T) {
	tests := []struct {
		name        string
		projectName string
		wantNotNil  bool
	}{
		{"creates form with project name", "acupoll", true},
		{"creates form with empty project name", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var nameValue string
			form := NewCreateNameForm(&nameValue, tt.projectName, nil)
			if form == nil {
				t.Fatal("NewCreateNameForm returned nil")
			}
		})
	}
}

func TestNewCreateBranchForm(t *testing.T) {
	tests := []struct {
		name    string
		wantNil bool
	}{
		{"creates branch selection form", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var choice string
			form := NewCreateBranchForm(&choice)
			if (form == nil) != tt.wantNil {
				t.Errorf("NewCreateBranchForm() nil = %v, want nil = %v", form == nil, tt.wantNil)
			}
		})
	}
}

func TestNewBranchPickerForm(t *testing.T) {
	tests := []struct {
		name     string
		branches []string
		wantNil  bool
	}{
		{"creates picker with branches", []string{"main", "develop", "feature/auth"}, false},
		{"creates picker with empty branches", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var selected string
			form := NewBranchPickerForm(&selected, tt.branches)
			if (form == nil) != tt.wantNil {
				t.Errorf("NewBranchPickerForm() nil = %v, want nil = %v", form == nil, tt.wantNil)
			}
		})
	}
}

func TestCreateNameValidation(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty name is invalid", "", true},
		{"valid name passes", "feature-auth", false},
		{"name with spaces is invalid", "bad name", true},
		{"name with special chars is invalid", "bad*name", true},
		{"valid hyphenated name", "my-cool-feature", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := createNameValidator(nil)
			err := validator(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("createNameValidator(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestCreateNameValidationDetectsDuplicate(t *testing.T) {
	existing := []WorktreeItem{
		{ShortName: "feature-auth", Path: "/work/proj-feature-auth", Branch: "feature/auth"},
	}

	validator := createNameValidator(existing)

	err := validator("feature-auth")
	if err == nil {
		t.Error("expected error for duplicate name, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should mention 'already exists', got: %s", err.Error())
	}

	err = validator("feature-new")
	if err != nil {
		t.Errorf("expected no error for non-duplicate, got: %v", err)
	}
}

func TestCreateFormUsesCharmTheme(t *testing.T) {
	var nameValue string
	form := NewCreateNameForm(&nameValue, "proj", nil)

	// The form should render without panicking - basic smoke test
	view := form.View()
	if view == "" {
		t.Error("form View() returned empty string")
	}
}

func TestCreateFormAccessibleMode(t *testing.T) {
	var nameValue string
	form := NewCreateNameForm(&nameValue, "proj", nil)
	accessibleForm := form.WithAccessible(true)
	if accessibleForm == nil {
		t.Error("WithAccessible returned nil")
	}
}

func TestFormStateValues(t *testing.T) {
	// Verify that huh.FormState constants exist and are usable
	_ = huh.StateNormal
	_ = huh.StateCompleted
	_ = huh.StateAborted
}

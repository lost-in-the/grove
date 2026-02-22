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
			form := NewCreateNameForm(&nameValue, tt.projectName, nil, "")
			if form == nil {
				t.Fatal("NewCreateNameForm returned nil")
			}
		})
	}
}

func TestNewCreateNameFormWithPlaceholder(t *testing.T) {
	var nameValue string
	form := NewCreateNameForm(&nameValue, "proj", nil, "agent-slot-db")
	if form == nil {
		t.Fatal("NewCreateNameForm returned nil")
	}
	view := form.View()
	if view == "" {
		t.Error("form View() returned empty string")
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
			validator := createNameValidator(nil, "")
			err := validator(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("createNameValidator(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestCreateNameValidationWithSuggestion(t *testing.T) {
	// Empty input is valid when there's a real suggestion
	validator := createNameValidator(nil, "agent-slot-db")
	err := validator("")
	if err != nil {
		t.Errorf("expected nil error for empty input with suggestion, got: %v", err)
	}

	// Empty input is still invalid when suggestion is the default placeholder
	validator = createNameValidator(nil, "feature-name")
	err = validator("")
	if err == nil {
		t.Error("expected error for empty input with default placeholder")
	}
}

func TestCreateNameValidationDetectsDuplicate(t *testing.T) {
	existing := []WorktreeItem{
		{ShortName: "feature-auth", Path: "/work/proj-feature-auth", Branch: "feature/auth"},
	}

	validator := createNameValidator(existing, "")

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
	form := NewCreateNameForm(&nameValue, "proj", nil, "")

	// The form should render without panicking - basic smoke test
	view := form.View()
	if view == "" {
		t.Error("form View() returned empty string")
	}
}

func TestCreateFormAccessibleMode(t *testing.T) {
	var nameValue string
	form := NewCreateNameForm(&nameValue, "proj", nil, "")
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

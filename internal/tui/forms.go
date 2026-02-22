package tui

import (
	"fmt"

	"github.com/charmbracelet/huh"
)

// NewCreateNameForm creates a Huh form for the worktree name input step.
// The nameValue pointer will be populated with the user's input.
// existingItems is used for duplicate detection validation.
// placeholder is shown as dimmed hint text when input is empty (e.g. a name derived from the branch).
func NewCreateNameForm(nameValue *string, projectName string, existingItems []WorktreeItem, placeholder string) *huh.Form {
	if placeholder == "" {
		placeholder = "feature-name"
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Worktree Name").
				Placeholder(placeholder).
				Validate(createNameValidator(existingItems, placeholder)).
				Value(nameValue),
		),
	).WithTheme(huh.ThemeCharm()).WithShowHelp(false).WithAccessible(isHighContrast())

	return form
}

// createNameValidator returns a validation function for worktree names.
// It checks format validity and duplicate detection against existing worktrees.
// If suggestion is non-empty, an empty input is valid (will use the suggestion).
func createNameValidator(existingItems []WorktreeItem, suggestion string) func(string) error {
	return func(name string) error {
		effectiveName := name
		if effectiveName == "" {
			if suggestion != "" && suggestion != "feature-name" {
				effectiveName = suggestion
			} else {
				return fmt.Errorf("name cannot be empty")
			}
		}

		if errMsg := ValidateWorktreeName(effectiveName); errMsg != "" {
			return fmt.Errorf("%s", errMsg)
		}

		for _, item := range existingItems {
			if item.ShortName == effectiveName {
				return fmt.Errorf("worktree %q already exists at %s", effectiveName, item.Path)
			}
		}

		return nil
	}
}

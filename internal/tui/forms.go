package tui

import (
	"fmt"

	"github.com/charmbracelet/huh"
)

// NewCreateNameForm creates a Huh form for the worktree name input step.
// The nameValue pointer will be populated with the user's input.
// existingItems is used for duplicate detection validation.
func NewCreateNameForm(nameValue *string, projectName string, existingItems []WorktreeItem) *huh.Form {
	description := "Worktree name"
	if projectName != "" {
		description = fmt.Sprintf("Will create: %s-<name>", projectName)
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Worktree Name").
				Description(description).
				Placeholder("feature-name").
				Validate(createNameValidator(existingItems)).
				Value(nameValue),
		),
	).WithTheme(huh.ThemeCharm())

	return form
}

// NewCreateBranchForm creates a Huh form for selecting the branch strategy.
func NewCreateBranchForm(choice *string) *huh.Form {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Branch Strategy").
				Description("How should the worktree branch be created?").
				Options(
					huh.NewOption("Create new branch", "new"),
					huh.NewOption("From existing branch...", "existing"),
				).
				Value(choice),
		),
	).WithTheme(huh.ThemeCharm())

	return form
}

// NewBranchPickerForm creates a Huh form for selecting from existing branches.
func NewBranchPickerForm(selected *string, branches []string) *huh.Form {
	options := make([]huh.Option[string], len(branches))
	for i, b := range branches {
		options[i] = huh.NewOption(b, b)
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select Branch").
				Description("Choose a branch to base the worktree on").
				Options(options...).
				Filtering(true).
				Value(selected),
		),
	).WithTheme(huh.ThemeCharm())

	return form
}

// createNameValidator returns a validation function for worktree names.
// It checks format validity and duplicate detection against existing worktrees.
func createNameValidator(existingItems []WorktreeItem) func(string) error {
	return func(name string) error {
		if name == "" {
			return fmt.Errorf("name cannot be empty")
		}

		if errMsg := ValidateWorktreeName(name); errMsg != "" {
			return fmt.Errorf("%s", errMsg)
		}

		for _, item := range existingItems {
			if item.ShortName == name {
				return fmt.Errorf("worktree %q already exists at %s", name, item.Path)
			}
		}

		return nil
	}
}

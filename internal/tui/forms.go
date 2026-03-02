package tui

import (
	"fmt"
)

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

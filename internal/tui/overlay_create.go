package tui

import (
	"charm.land/bubbles/v2/textinput"
)

// CreateStep represents the current step in the create wizard.
type CreateStep int

const (
	CreateStepBranch       CreateStep = 0 // unified branch selector
	CreateStepBranchAction CreateStep = 1 // split/fork (conditional)
	CreateStepName         CreateStep = 2 // name with suggestion
	CreateStepConfirm      CreateStep = 3
)

// CreateState holds the state for the new worktree wizard.
type CreateState struct {
	Step           CreateStep
	Name           string
	NameSuggestion string // derived from branch, shown as placeholder
	ProjectName    string
	BaseBranch     string // set when using existing branch (split)
	NewBranchName  string // set when creating new branch via selector
	Error          string

	// Branch selector state (unified)
	Branches          []string
	BranchCursor      int
	BranchFilterInput textinput.Model

	// Name input
	NameInput textinput.Model

	// Branch action state (split vs fork)
	ActionChoice  int // 0 = split (use as-is), 1 = fork (new branch from it)
	DontShowAgain bool

	// Duplicate validation
	ExistingWorktree *WorktreeItem // populated if name conflicts with existing worktree

	// Creating state
	Creating    bool
	ActivityLog *ActivityLog // streaming creation progress
}

func (s *CreateState) getActivityLog() *ActivityLog { return s.ActivityLog }
func (s *CreateState) setCreatingDone(errMsg string) {
	s.Creating = false
	s.Error = errMsg
}

func newBranchFilterInput() textinput.Model {
	return newFilterInput("type to filter or create new")
}

// newNameInput creates a configured textinput for worktree naming.
func newNameInput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Prompt = "Name: "
	ti.Placeholder = placeholder
	ti.CharLimit = 100
	return ti
}

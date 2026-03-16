package tui

import (
	"charm.land/bubbles/v2/textinput"
)

// CreateStep represents the current step in the create wizard.
type CreateStep int

const (
	CreateStepBranchChoice CreateStep = 0 // "Select existing" vs "Create new"
	CreateStepBranchSelect CreateStep = 1 // filterable branch list (/ to filter)
	CreateStepBranchCreate CreateStep = 2 // text input for new branch name
	CreateStepBranchAction CreateStep = 3 // split/fork (conditional)
	CreateStepName         CreateStep = 4 // name with suggestion
	CreateStepConfirm      CreateStep = 5
)

// BranchFilterMode tracks whether the filter input is active in the branch select step.
type BranchFilterMode int

const (
	BranchFilterOff BranchFilterMode = iota // j/k navigate, / enters filter
	BranchFilterOn                          // textinput active, esc exits filter
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

	// Branch choice state (step 0)
	BranchChoice int // 0 = select existing, 1 = create new

	// Branch selector state (step 1: select existing)
	Branches          []string
	BranchCursor      int
	BranchFilterInput textinput.Model
	BranchFilterMode  BranchFilterMode

	// Branch create state (step 2: create new)
	BranchNameInput textinput.Model

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
	return newFilterInput("type to filter branches")
}

// newBranchNameInput creates a configured textinput for new branch name entry.
func newBranchNameInput() textinput.Model {
	ti := textinput.New()
	ti.Prompt = "Branch: "
	ti.Placeholder = ""
	ti.CharLimit = 100
	return ti
}

// newNameInput creates a configured textinput for worktree naming.
func newNameInput(suggestion string) textinput.Model {
	ti := textinput.New()
	ti.Prompt = "Name: "
	ti.Placeholder = ""
	ti.CharLimit = 100
	return ti
}

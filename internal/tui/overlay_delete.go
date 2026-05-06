package tui

// DeleteState holds the state for the delete confirmation overlay.
type DeleteState struct {
	Item         *WorktreeItem
	Warnings     []string
	DeleteBranch bool
	Deleting     bool
}

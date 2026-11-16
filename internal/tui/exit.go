package tui

// Exit is what the dashboard leaves for its caller after it closes.
type Exit struct {
	// Path is the worktree the user picked. Empty when they only quit.
	Path string
	// Name is the worktree's display name ("root" for the main worktree),
	// the name `grove to` resolves.
	Name string
	// Up is true after U: the caller runs `grove to <Name> --up`, which moves
	// the user and brings the stack. The dashboard has not moved them.
	Up bool
}

// Exit returns what the user asked for when they closed the dashboard.
func (m Model) Exit() Exit {
	return Exit{Path: m.switchTo, Name: m.switchToDisplayName, Up: m.switchForceUp}
}

// movesOnExit reports whether the dashboard's own exit path moves the user.
// After U it does not: the caller's switch flow brings the stack up before
// the multiplexer client moves, so the user sees the stack output.
func (m Model) movesOnExit() bool {
	return m.switchTo != "" && !m.switchForceUp
}

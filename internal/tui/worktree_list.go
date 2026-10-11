package tui

import "charm.land/bubbles/v2/list"

// newWorktreeList builds the dashboard list. Grove owns quitting, so the
// list's own Quit binding is disabled; otherwise a bubbles release that
// rebinds it would quit the app from an ordinary key. DisableQuitKeybindings
// is the only form that survives the list re-deriving its bindings on every
// SetItems and SetSize. It also disables ForceQuit, which is re-enabled
// because ctrl+c quits the dashboard through it.
func newWorktreeList(delegate list.ItemDelegate) list.Model {
	l := list.New(nil, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.KeyMap.Filter.SetKeys("/")
	l.KeyMap.CursorUp.SetKeys("up", "k")
	l.KeyMap.CursorDown.SetKeys("down", "j")
	l.DisableQuitKeybindings()
	l.KeyMap.ForceQuit.SetEnabled(true)
	return l
}

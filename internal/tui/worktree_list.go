package tui

import "charm.land/bubbles/v2/list"

// newWorktreeList builds the dashboard list. Grove owns quitting, so the
// list's own Quit binding is disabled; otherwise a bubbles release that
// rebinds it would quit the app from an ordinary key.
func newWorktreeList(delegate list.ItemDelegate) list.Model {
	l := list.New(nil, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.KeyMap.Filter.SetKeys("/")
	l.KeyMap.CursorUp.SetKeys("up", "k")
	l.KeyMap.CursorDown.SetKeys("down", "j")
	l.KeyMap.Quit.SetEnabled(false)
	return l
}

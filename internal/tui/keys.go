package tui

import (
	"github.com/charmbracelet/bubbles/key"
)

// KeyMap defines all keybindings for the TUI.
type KeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Enter   key.Binding
	New     key.Binding
	Delete  key.Binding
	Filter  key.Binding
	Refresh key.Binding
	Help    key.Binding
	Quit    key.Binding
	Escape  key.Binding
	Back    key.Binding
	Tab     key.Binding

	// Sort
	Sort key.Binding

	// PRs
	PRs key.Binding

	// Issues
	Issues key.Binding

	// Overlay-specific
	Confirm key.Binding
	Deny    key.Binding
	Toggle  key.Binding
	All     key.Binding

	// Navigation
	ShiftTab key.Binding

	// Fork/Sync/Config
	Fork   key.Binding
	Sync   key.Binding
	Config key.Binding
}

// ShortHelp returns keybindings for the short help view (help.KeyMap interface).
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Enter, k.New, k.Delete, k.Fork, k.Sync, k.Config, k.PRs, k.Issues, k.Sort, k.Filter, k.Help, k.Quit}
}

// FullHelp returns keybindings for the full help view (help.KeyMap interface).
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Enter},
		{k.New, k.Delete, k.Fork, k.Sync, k.Config, k.PRs, k.Issues, k.Sort, k.Filter, k.Refresh},
		{k.Help, k.Quit, k.Escape},
	}
}

// DefaultKeyMap returns the default set of keybindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "switch"),
		),
		New: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "new"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "quit"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Back: key.NewBinding(
			key.WithKeys("backspace"),
			key.WithHelp("backspace", "back"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "detail"),
		),
		ShiftTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "prev tab"),
		),

		Sort: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "sort"),
		),

		PRs: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "PRs"),
		),

		Issues: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "issues"),
		),

		// Overlay keys
		Confirm: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "confirm"),
		),
		Deny: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "cancel"),
		),
		Toggle: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "toggle"),
		),
		All: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "all merged"),
		),

		Fork: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "fork"),
		),
		Sync: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "sync"),
		),
		Config: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "config"),
		),
	}
}

package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// testOpt is a functional option for configuring a test model.
type testOpt func(*Model)

// newTestModel creates a Model suitable for unit tests, without requiring
// a real worktree.Manager. Must call withSize() to trigger ready=true.
func newTestModel(opts ...testOpt) Model {
	keys := DefaultKeyMap()

	s := GroveSpinner()

	delegate := NewWorktreeDelegate()
	l := list.New(nil, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.KeyMap.Filter.SetKeys("/")
	l.KeyMap.CursorUp.SetKeys("up", "k")
	l.KeyMap.CursorDown.SetKeys("down", "j")

	h := help.New()

	m := Model{
		projectRoot: "/tmp/test-project",
		projectName: "test-project",
		keys:        keys,
		list:        l,
		spinner:     s,
		help:        h,
		toast:       NewToastModel(),
		helpFooter:  NewHelpFooter(),
		detail:      viewport.New(80, 20),
		activeView:  ViewDashboard,
		loading:     false,
		ready:       true,
		width:       80,
		height:      24,
	}

	for _, opt := range opts {
		opt(&m)
	}

	return m
}

// withItems seeds the model with n test worktree items.
func withItems(n int) testOpt {
	return func(m *Model) {
		items := makeTestItems(n)
		listItems := make([]list.Item, len(items))
		for i, item := range items {
			listItems[i] = item
		}
		m.list.SetItems(listItems)
	}
}

// withSize sends a WindowSizeMsg through Update to properly initialize layout.
func withSize(w, h int) testOpt {
	return func(m *Model) {
		m.width = w
		m.height = h
		m.ready = true
		m.updateLayout()
	}
}

// withLoading sets the loading state.
func withLoading() testOpt {
	return func(m *Model) {
		m.loading = true
	}
}

// makeTestItems generates n WorktreeItem values for testing.
func makeTestItems(n int) []WorktreeItem {
	names := []string{"root", "feature-auth", "fix-bug", "testing", "refactor", "docs", "perf", "ci", "staging", "dev"}
	items := make([]WorktreeItem, n)
	for i := 0; i < n; i++ {
		name := names[i%len(names)]
		items[i] = WorktreeItem{
			ShortName:     name,
			FullName:      "test-project-" + name,
			Path:          "/tmp/test-project-" + name,
			Branch:        name,
			Commit:        "abc1234",
			CommitMessage: "test commit for " + name,
			CommitAge:     "2 hours ago",
			IsMain:        i == 0,
			IsCurrent:     i == 0,
			IsDirty:       i%3 == 0,
			TmuxStatus:    "none",
			LastAccessed:  time.Now().Add(-time.Duration(i) * time.Hour),
		}
	}
	return items
}

// sendKey sends a key message through Update and returns the resulting model.
func sendKey(m Model, keyStr string) Model {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(keyStr)}

	// Handle special keys
	switch keyStr {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEscape}
	case "tab":
		msg = tea.KeyMsg{Type: tea.KeyTab}
	case "backspace":
		msg = tea.KeyMsg{Type: tea.KeyBackspace}
	case "up":
		msg = tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		msg = tea.KeyMsg{Type: tea.KeyDown}
	case " ":
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}}
	}

	result, _ := m.Update(msg)
	return result.(Model)
}

// sendMsg sends an arbitrary message through Update and returns the resulting model.
func sendMsg(m Model, msg tea.Msg) Model {
	result, _ := m.Update(msg)
	return result.(Model)
}

// enterCreateManual enters the create wizard and disables Huh forms so that
// manual key handling tests continue to work.
func enterCreateManual(m Model) Model {
	m = sendKey(m, "n")
	if m.createState != nil {
		m.createState.UseHuhForms = false
	}
	return m
}

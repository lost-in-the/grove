package tui

import (
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

// testOpt is a functional option for configuring a test model.
type testOpt func(*Model)

// newTestModel creates a Model suitable for unit tests, without requiring
// a real worktree.Manager. Must call withSize() to trigger ready=true.
func newTestModel(opts ...testOpt) Model {
	keys := DefaultKeyMap()

	s := GroveSpinner()

	l := newWorktreeList(rowDelegate{Secondary: "branch", Now: func() time.Time { return testNow }})

	h := help.New()

	m := Model{
		projectRoot: "/tmp/test-project",
		projectName: "test-project",
		keys:        keys,
		list:        l,
		spinner:     s,
		help:        h,
		toast:       NewToastModel(),
		now:         func() time.Time { return testNow },
		compactMode: true,
		helpFooter:  NewHelpFooter(),
		helpOverlay: NewHelpOverlay(),
		detail:      viewport.New(viewport.WithWidth(80), viewport.WithHeight(20)),
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
// testNow is the fixed clock for tests and goldens, so ages never drift.
var testNow = time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)

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
			CommitTime:    testNow.Add(-time.Duration(i) * time.Hour),
			IsMain:        i == 0,
			IsCurrent:     i == 0,
			IsDirty:       i%3 == 0,
			TmuxStatus:    "none",
			LastAccessed:  time.Now().Add(-time.Duration(i) * time.Hour),
		}
	}
	return items
}

// makeKeyMsg constructs a tea.KeyPressMsg for the given key string.
// Central point for key message construction — v2 uses KeyPressMsg with Code and Text fields.
func makeKeyMsg(keyStr string) tea.KeyPressMsg {
	switch keyStr {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "pgdown":
		return tea.KeyPressMsg{Code: tea.KeyPgDown}
	case "pgup":
		return tea.KeyPressMsg{Code: tea.KeyPgUp}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	case "ctrl+j":
		return tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl}
	case "ctrl+k":
		return tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl}
	case "ctrl+d":
		return tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl}
	case "ctrl+u":
		return tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl}
	case "ctrl+n":
		return tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl}
	case "ctrl+p":
		return tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl}
	case "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	default:
		// Printable character(s)
		runes := []rune(keyStr)
		if len(runes) == 1 {
			return tea.KeyPressMsg{Code: runes[0], Text: keyStr}
		}
		return tea.KeyPressMsg{Code: runes[0], Text: keyStr}
	}
}

// sendKey sends a key message through Update and returns the resulting model.
func sendKey(m Model, keyStr string) Model {
	result, _ := m.Update(makeKeyMsg(keyStr))
	return result.(Model)
}

// sendMsg sends an arbitrary message through Update and returns the resulting model.
func sendMsg(m Model, msg tea.Msg) Model {
	result, _ := m.Update(msg)
	return result.(Model)
}

// setViewForTest makes v the only frame above the dashboard root.
func (m *Model) setViewForTest(v ActiveView) {
	m.views = nil
	if v != ViewDashboard {
		m.pushView(v)
	}
}

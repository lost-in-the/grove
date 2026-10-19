package tui

import (
	"slices"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/lost-in-the/grove/internal/version"
)

// Action is one entry in the dashboard's action table (spec 15.4). Key
// dispatch, footer hints, and the strips all read this table, so they
// cannot drift apart. Omni (phase 5b) searches it by label and synonyms.
type Action struct {
	ID       string
	Keys     []string // the keys that run it, as key.Binding spells them
	Display  string   // key text in the footer and strips; Keys[0] when empty
	Layer    string   // "" for bare keys, "g" for the goto strip
	Label    string   // what the action does, as omni lists it
	Short    string   // footer text when it differs from Label
	Synonyms []string
	Footer   int // position in the dashboard footer; 0 = not shown

	// Enabled reports whether the action applies now. A disabled action
	// still consumes its key. nil means always enabled.
	Enabled func(m Model) bool
	// Run performs the action; k is the key that triggered it. nil means
	// the list owns the key (cursor movement, filter, paging).
	Run func(m Model, k string) (tea.Model, tea.Cmd)
}

// display is the key text shown for the action.
func (a Action) display() string {
	switch {
	case a.Display != "":
		return a.Display
	case len(a.Keys) > 0:
		return a.Keys[0]
	}
	return ""
}

func (a Action) matches(layer, k string) bool {
	return a.Layer == layer && slices.Contains(a.Keys, k)
}

// findAction returns the action bound to k in layer.
func findAction(layer, k string) (Action, bool) {
	for _, a := range dashboardActions() {
		if a.matches(layer, k) {
			return a, true
		}
	}
	return Action{}, false
}

// dashboardActions is the dashboard's action table, in footer order where
// it matters.
func dashboardActions() []Action {
	return []Action{
		{ID: "list.navigate", Keys: []string{"up", "k", "down", "j"}, Display: "↑↓", Label: "navigate", Footer: 1},
		{ID: "worktree.switch", Keys: []string{"enter"}, Label: "switch to worktree", Short: "switch", Footer: 2,
			Synonyms: []string{"go to", "open", "cd"},
			Run:      func(m Model, _ string) (tea.Model, tea.Cmd) { return m.handleDashboardEnterKey() }},
		{ID: "worktree.switch_up", Keys: []string{"U"}, Label: "switch and start containers", Short: "up",
			Synonyms: []string{"switch and start containers", "docker up"},
			Run:      func(m Model, _ string) (tea.Model, tea.Cmd) { return m.handleDashboardSwitchUpKey() }},
		{ID: "worktree.new", Keys: []string{"n"}, Label: "new worktree", Short: "new",
			Synonyms: []string{"create", "add", "new worktree"},
			Run:      func(m Model, _ string) (tea.Model, tea.Cmd) { return m.handleDashboardNewKey() }},
		{ID: "omni.open", Keys: []string{"space"}, Display: "␣", Label: "find", Footer: 3,
			Run: func(m Model, _ string) (tea.Model, tea.Cmd) { return m.openOmni(omniScopeAll) }},
		{ID: "list.filter", Keys: []string{"/"}, Label: "filter", Footer: 5, Synonyms: []string{"search"}},
		{ID: "detail.open", Keys: []string{"tab", "l"}, Label: "show detail", Short: "detail", Footer: 4,
			Synonyms: []string{"details", "info"},
			Run:      func(m Model, _ string) (tea.Model, tea.Cmd) { m.openDetail(); return m, nil }},
		{ID: "list.sort", Keys: []string{"o"}, Label: "change sort", Short: "sort",
			Run: func(m Model, _ string) (tea.Model, tea.Cmd) {
				m.helpFooter.SetHighlight("o")
				m.sortMode = m.sortMode.Next(m.agentAware)
				return m, m.applySortToList()
			}},
		{ID: "goto.open", Keys: []string{"g"}, Label: "goto", Footer: 6,
			Run: func(m Model, _ string) (tea.Model, tea.Cmd) {
				m.pendingG = true
				m.updateLayout()
				return m, nil
			}},
		// ? runs from handleKey in every view; its entry feeds the footer
		// and keeps the key map and this table in step.
		{ID: "app.help", Keys: []string{"?"}, Label: "help", Footer: 7},
		{ID: "app.messages", Label: "messages", Synonyms: []string{"warnings", "errors", "log", "history"},
			Run: func(m Model, _ string) (tea.Model, tea.Cmd) { return m.openMessages() }},
		{ID: "app.shortcuts", Label: "keyboard shortcuts", Synonyms: []string{"help", "keys", "cli commands"},
			Run: func(m Model, _ string) (tea.Model, tea.Cmd) {
				m.helpOverlay.Open(ViewDashboard, m.width, m.height)
				m.pushView(ViewHelp)
				return m, nil
			}},
		{ID: "app.quit", Keys: []string{"q"}, Label: "quit", Synonyms: []string{"exit"},
			Run: func(m Model, _ string) (tea.Model, tea.Cmd) { return m, tea.Quit }},
		{ID: "app.back", Keys: []string{"esc"}, Label: "back",
			Run: func(m Model, _ string) (tea.Model, tea.Cmd) {
				// With a filter applied, esc clears it instead of quitting.
				if m.list.FilterState() == list.FilterApplied {
					m.list.ResetFilter()
					return m, nil
				}
				return m, tea.Quit
			}},

		{ID: "worktree.delete", Enabled: notMainOrProtected, Keys: []string{"d"}, Label: "delete worktree",
			Synonyms: []string{"remove", "trash", "rm"},
			Run:      func(m Model, _ string) (tea.Model, tea.Cmd) { return m.handleDashboardDeleteKey() }},
		{ID: "worktree.rename", Enabled: notMainOrProtected, Keys: []string{"R"}, Label: "rename worktree",
			Synonyms: []string{"move", "retitle"},
			Run:      func(m Model, _ string) (tea.Model, tea.Cmd) { return m.handleDashboardRenameKey() }},
		{ID: "worktree.copy", Keys: []string{"f"}, Label: "copy worktree",
			Synonyms: []string{"fork", "duplicate", "clone"},
			Run:      func(m Model, _ string) (tea.Model, tea.Cmd) { return m.handleDashboardForkKey() }},
		{ID: "worktree.switch_branch", Enabled: notMainOrProtected, Keys: []string{"b"}, Label: "switch branch",
			Synonyms: []string{"checkout", "change branch"},
			Run:      func(m Model, _ string) (tea.Model, tea.Cmd) { return m.handleDashboardCheckoutKey() }},
		{ID: "worktree.sync", Keys: []string{"s"}, Label: "sync changes",
			Synonyms: []string{"pull changes", "bring over"},
			Run: func(m Model, _ string) (tea.Model, tea.Cmd) {
				m.pushView(ViewSync)
				m.syncState = NewSyncState(m.existingWorktreeItems())
				return m, gatherWIPInfoCmd(m.existingWorktreeItems())
			}},
		{ID: "worktree.bulk_delete", Keys: []string{"a"}, Label: "bulk delete",
			Synonyms: []string{"remove several", "clean up"},
			Run:      func(m Model, _ string) (tea.Model, tea.Cmd) { return m.enterBulkMode() }},
		{ID: "worktree.open_pr", Keys: []string{"B"}, Label: "open PR in browser",
			Synonyms: []string{"pull request", "github"},
			Run:      func(m Model, _ string) (tea.Model, tea.Cmd) { m.openSelectedPRURL(); return m, nil }},
		{ID: "view.prs", Keys: []string{"p"}, Label: "pull requests",
			Synonyms: []string{"prs", "reviews"},
			Run:      func(m Model, _ string) (tea.Model, tea.Cmd) { return m.enterPRView() }},
		{ID: "view.issues", Keys: []string{"i"}, Label: "issues", Synonyms: []string{"tickets", "bugs"},
			Run: func(m Model, _ string) (tea.Model, tea.Cmd) { return m.enterIssueView() }},
		{ID: "view.config", Keys: []string{"c"}, Label: "config", Synonyms: []string{"settings", "preferences"},
			Run: func(m Model, _ string) (tea.Model, tea.Cmd) { return m.openConfig() }},
		{ID: "list.refresh", Keys: []string{"r"}, Label: "refresh", Synonyms: []string{"reload"},
			Run: func(m Model, _ string) (tea.Model, tea.Cmd) {
				m.helpFooter.SetHighlight("r")
				m.loading = true
				return m, tea.Batch(m.spinner.Tick, m.fetchWorktrees)
			}},
		{ID: "list.density", Keys: []string{"v"}, Label: "show or hide commit messages",
			Synonyms: []string{"density", "compact", "view mode"},
			Run: func(m Model, _ string) (tea.Model, tea.Cmd) {
				m.helpFooter.SetHighlight("v")
				m.toggleCompactMode()
				return m, nil
			}},
		{ID: "list.sort_reverse", Keys: []string{"O"}, Label: "reverse sort",
			Run: func(m Model, _ string) (tea.Model, tea.Cmd) {
				m.helpFooter.SetHighlight("O")
				m.sortReversed = !m.sortReversed
				return m, m.applySortToList()
			}},
		{ID: "list.bottom", Keys: []string{"G"}, Label: "bottom of list",
			Run: func(m Model, _ string) (tea.Model, tea.Cmd) {
				m.moveCursorTo(len(m.list.VisibleItems()) - 1)
				return m, nil
			}},
		{ID: "list.half_down", Keys: []string{"ctrl+d"}, Label: "half page down",
			Run: func(m Model, _ string) (tea.Model, tea.Cmd) {
				m.moveCursorTo(m.list.Index() + m.halfPage())
				return m, nil
			}},
		{ID: "list.half_up", Keys: []string{"ctrl+u"}, Label: "half page up",
			Run: func(m Model, _ string) (tea.Model, tea.Cmd) {
				m.moveCursorTo(m.list.Index() - m.halfPage())
				return m, nil
			}},
		{ID: "list.page", Keys: []string{"pgup", "pgdown", "home", "end"}, Display: "pgup/pgdown", Label: "page"},
		// h closes the detail pane; on the list there is nothing to close.
		{ID: "detail.close", Keys: []string{"h"}, Label: "back to list",
			Run: func(m Model, _ string) (tea.Model, tea.Cmd) { return m, nil }},
		{ID: "pin.open", Keys: []string{"m"}, Label: "pin to a slot", Synonyms: []string{"pin", "bookmark", "mark"},
			Run: func(m Model, _ string) (tea.Model, tea.Cmd) {
				m.pendingM = true
				m.updateLayout()
				return m, nil
			}},
		{ID: "pin.switch", Keys: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"}, Display: "1-9",
			Label: "switch to pinned worktree",
			Run:   func(m Model, k string) (tea.Model, tea.Cmd) { return m.switchToSlot(k) }},
		{ID: "worktree.last", Keys: []string{"-"}, Label: "previous worktree", Synonyms: []string{"last", "back"},
			Run: func(m Model, _ string) (tea.Model, tea.Cmd) { return m.switchToLastActive() }},

		// The g strip (spec 6.3).
		{ID: "goto.top", Layer: "g", Keys: []string{"g"}, Label: "top of list",
			Run: func(m Model, _ string) (tea.Model, tea.Cmd) { m.moveCursorTo(0); return m, nil }},
		{ID: "goto.prs", Layer: "g", Keys: []string{"p"}, Label: "pull requests",
			Run: func(m Model, _ string) (tea.Model, tea.Cmd) { return m.enterPRView() }},
		{ID: "goto.issues", Layer: "g", Keys: []string{"i"}, Label: "issues",
			Run: func(m Model, _ string) (tea.Model, tea.Cmd) { return m.enterIssueView() }},
		{ID: "goto.config", Layer: "g", Keys: []string{"c"}, Label: "config",
			Run: func(m Model, _ string) (tea.Model, tea.Cmd) { return m.openConfig() }},
		{ID: "goto.upgrade", Layer: "g", Keys: []string{"u"}, Label: "upgrade grove",
			Synonyms: []string{"update", "release", "new version"},
			Run:      func(m Model, _ string) (tea.Model, tea.Cmd) { return m.openUpgrade() }},
		{ID: "goto.agent", Layer: "g", Keys: []string{"a"}, Label: "agent needing you",
			Synonyms: []string{"blocked", "attention", "needs you"},
			Enabled:  func(m Model) bool { return m.agentAware },
			Run:      func(m Model, _ string) (tea.Model, tea.Cmd) { return m.gotoAgent() }},
	}
}

// openConfig opens the config editor.
func (m Model) openConfig() (tea.Model, tea.Cmd) {
	m.pushView(ViewConfig)
	m.configState = NewConfigState()
	m.configState.resize(m.configFormSize())
	return m, loadConfigCmd()
}

// openUpgrade opens the upgrade overlay: the newer release when one is
// cached, otherwise that grove is up to date (spec 12).
func (m Model) openUpgrade() (tea.Model, tea.Cmd) {
	if m.updateOverlay == nil {
		m.updateOverlay = NewUpdateOverlay()
	}
	if m.updateAvailable() {
		m.updateOverlay.Open(version.Version, m.updateLatestVersion, m.updateLatestURL)
	} else {
		m.updateOverlay.Open(version.Version, "", "")
	}
	m.pushView(ViewUpdate)
	return m, nil
}

// dashboardFooterHints are the footer-marked actions, in footer order.
func dashboardFooterHints() []Hint {
	var marked []Action
	for _, a := range dashboardActions() {
		if a.Footer > 0 {
			marked = append(marked, a)
		}
	}
	slices.SortStableFunc(marked, func(x, y Action) int { return x.Footer - y.Footer })
	hints := make([]Hint, len(marked))
	for i, a := range marked {
		label := a.Label
		if a.Short != "" {
			label = a.Short
		}
		hints[i] = Hint{a.display(), label}
	}
	return hints
}

// notMainOrProtected enables actions that cannot apply to the main or a
// protected worktree.
func notMainOrProtected(m Model) bool {
	item, ok := m.selectedItem()
	return ok && !item.IsMain && !item.IsProtected
}

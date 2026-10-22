package tui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const (
	omniMaxHeight = 20
	omniGroupW    = 15 // group name column; fits "pull requests" plus a gap
)

// OmniState is the open omni overlay (spec 9).
type OmniState struct {
	Scope  omniScope
	Input  textinput.Model
	Target string // path of the worktree selected when omni opened
	Cursor int    // index into rows()
	src    omniSource
	groups []omniGroup
}

// omniRow is one result with the group it belongs to.
type omniRow struct {
	Group string // set on the first row of a group only
	Item  omniItem
}

func (s *OmniState) refresh() {
	s.groups = omniResults(s.Input.Value(), s.Scope, s.src)
	s.Cursor = max(0, min(s.Cursor, len(s.rows())-1))
}

func (s *OmniState) rows() []omniRow {
	var out []omniRow
	for _, g := range s.groups {
		for i, it := range g.Items {
			r := omniRow{Item: it}
			if i == 0 {
				r.Group = g.Name
			}
			out = append(out, r)
		}
	}
	return out
}

func (s *OmniState) selected() (omniItem, bool) {
	rows := s.rows()
	if s.Cursor < 0 || s.Cursor >= len(rows) {
		return omniItem{}, false
	}
	return rows[s.Cursor].Item, true
}

// omniSourceFor snapshots what omni can find. Actions are the runnable ones
// enabled for the selected worktree.
func (m Model) omniSourceFor() omniSource {
	var actions []Action
	for _, a := range omniActionsFrom(dashboardActions()) {
		if a.Enabled == nil || a.Enabled(m) {
			actions = append(actions, a)
		}
	}
	return omniSource{
		Worktrees:  m.existingWorktreeItems(),
		Actions:    actions,
		PRs:        m.prCache,
		Issues:     m.issueCache,
		Pins:       m.pins,
		AgentAware: m.agentAware,
	}
}

// openOmni pushes omni over the current view. It never fetches.
func (m Model) openOmni(scope omniScope) (tea.Model, tea.Cmd) {
	in := textinput.New()
	in.Prompt = ""
	in.Placeholder = "type a worktree, action, or #PR"
	if scope == omniScopeActions {
		in.Placeholder = "type an action"
	}
	w, _ := omniFrameSize(m.width, m.height)
	in.SetWidth(max(4, w-8))
	s := &OmniState{Scope: scope, Input: in, src: m.omniSourceFor()}
	if item, ok := m.selectedItem(); ok {
		s.Target = item.Path
	}
	s.refresh()
	m.omni = s
	m.pushView(ViewOmni)
	return m, m.omni.Input.Focus()
}

func (m Model) handleOmniKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := m.omni
	if s == nil {
		m.popView()
		return m, nil
	}
	switch k := msg.String(); {
	case key.Matches(msg, m.keys.Escape):
		m.omni = nil
		m.popView()
		return m, nil
	case k == "down" || k == "ctrl+j":
		s.Cursor = min(s.Cursor+1, max(0, len(s.rows())-1))
		return m, nil
	case k == "up" || k == "ctrl+k":
		s.Cursor = max(0, s.Cursor-1)
		return m, nil
	case k == "enter":
		return m.chooseOmni()
	}
	return m, m.typeOmni(msg)
}

func (m Model) typeOmni(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	m.omni.Input, cmd = m.omni.Input.Update(msg)
	m.omni.Cursor = 0
	m.omni.refresh()
	return cmd
}

// chooseOmni runs the selected row (spec 9.3): switch to a worktree, run an
// action on the target, or open a PR or issue in its browser.
func (m Model) chooseOmni() (tea.Model, tea.Cmd) {
	s := m.omni
	it, ok := s.selected()
	if !ok {
		return m, nil
	}
	m.omni = nil
	m.popView() // restores the list cursor to the target worktree
	switch it.Kind {
	case omniWorktree:
		for _, w := range m.existingWorktreeItems() {
			if w.Path == it.Ref {
				if !w.IsCurrent {
					m.switchTo = w.Path
					m.switchToDisplayName = w.displayName()
				}
				return m, tea.Quit
			}
		}
		m.toast.Show(NewToast("that worktree is gone", ToastWarning))
		return m, nil
	case omniAction:
		// Only an action opened with a worktree selected needs that
		// worktree; config, new, pull requests and the like do not.
		if s.Target != "" {
			if sel, ok := m.selectedItem(); !ok || sel.Path != s.Target {
				m.toast.Show(NewToast("that worktree is gone", ToastWarning))
				return m, nil
			}
		}
		for _, a := range dashboardActions() {
			if a.ID != it.Ref || a.Run == nil {
				continue
			}
			if a.Enabled != nil && !a.Enabled(m) {
				return m, nil
			}
			k := ""
			if len(a.Keys) > 0 {
				k = a.Keys[0]
			}
			return a.Run(m, k)
		}
	case omniPR:
		n, _ := strconv.Atoi(it.Ref)
		for _, pr := range m.prCache {
			if pr.Number == n {
				return m.openPRReview(pr)
			}
		}
		return m.enterPRView()
	case omniIssue:
		m.pendingIssue, _ = strconv.Atoi(it.Ref)
		return m.enterIssueView()
	}
	return m, nil
}

// omniFrameSize is omni's frame size. Like the branch form, it depends only
// on the terminal, so typing never changes it.
func omniFrameSize(termW, termH int) (int, int) {
	w, _ := branchFormSize(termW, termH)
	h := min(omniMaxHeight, termH-formTop-1)
	return w, max(h, 7)
}

func renderOmni(s *OmniState, termW, termH int) string {
	w, h := omniFrameSize(termW, termH)
	inner := w - 4
	bodyRows := h - 5 // border, input, status, key line

	title := "Find"
	if s.Scope == omniScopeActions {
		title = "Actions"
	}
	rows := s.rows()
	body := make([]string, 0, len(rows))
	for i, r := range rows {
		body = append(body, renderOmniRow(r, i == s.Cursor, inner))
	}
	if len(body) == 0 {
		body = append(body, Styles.DetailDim.Render("no matches"))
	}
	body = windowLines(body, bodyRows, s.Cursor)

	status, keys := omniStatus(s)
	lines := []string{Styles.OverlayTitle.Render(title), Styles.HelpKey.Render("␣ ▏") + s.Input.View()}
	lines = append(lines, body...)
	lines = append(lines, Styles.DetailDim.Render(status), Styles.Footer.Render(keys))
	for i, l := range lines {
		lines[i] = " " + padCells(l, inner) + " "
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Colors.Primary).
		Render(strings.Join(lines, "\n"))
}

// renderOmniRow draws one result: group name, cursor, label with matched
// letters lit, the matched synonym or branch when it is not the label, and
// the detail on the right.
func renderOmniRow(r omniRow, selected bool, width int) string {
	group := Styles.DetailLabel.Render(fmt.Sprintf("%-*s", omniGroupW-1, r.Group))
	cursor := "  "
	if selected {
		cursor = Styles.ListCursor.Render("❯ ")
	}
	it := r.Item
	labelHits, matchedHits := it.Hits, []int(nil)
	if it.Matched != "" {
		labelHits, matchedHits = nil, it.Hits
	}
	text := litRunes(it.Label, labelHits, Styles.NormalItem)
	if it.Matched != "" {
		text += Styles.DetailDim.Render(" · ") + litRunes(it.Matched, matchedHits, Styles.DetailDim)
	}
	detail := Styles.DetailDim.Render(it.Detail)
	left := group + " " + cursor + text
	room := width - lipgloss.Width(detail) - 1
	left = padCells(left, max(1, room))
	return left + " " + detail
}

// litRunes renders s with the runes at hits highlighted.
func litRunes(s string, hits []int, base lipgloss.Style) string {
	if len(hits) == 0 {
		return base.Render(s)
	}
	lit := map[int]bool{}
	for _, h := range hits {
		lit[h] = true
	}
	var b strings.Builder
	for i, r := range []rune(s) {
		if lit[i] {
			b.WriteString(Styles.HelpKey.Render(string(r)))
		} else {
			b.WriteString(base.Render(string(r)))
		}
	}
	return b.String()
}

// omniStatus says what enter does for the selected row.
func omniStatus(s *OmniState) (string, string) {
	keys := "⏎ choose · ctrl+j/k move · esc back"
	it, ok := s.selected()
	if !ok {
		if strings.TrimSpace(s.Input.Value()) == "" {
			return "type to search", keys
		}
		return "no matches", keys
	}
	switch it.Kind {
	case omniWorktree:
		if it.Current {
			return "⏎ stays on " + it.Label, keys
		}
		return "⏎ switches to " + it.Label, keys
	case omniAction:
		return "⏎ runs " + it.Label, keys
	case omniPR:
		return "⏎ opens pull request " + it.Detail, keys
	case omniIssue:
		return "⏎ opens issue " + it.Detail, keys
	}
	return "", keys
}

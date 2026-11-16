package tui

// viewFrame is one level of navigation above the dashboard: the view shown,
// and the dashboard cursor to restore when the frame is popped.
type viewFrame struct {
	view        ActiveView
	returnPath  string // worktree selected when the frame was pushed
	returnIndex int    // fallback row when that worktree is gone
}

// viewStack holds the frames above the dashboard root. Model is copied by
// value on every update, so push always allocates: two copies of a stack
// never share a backing array that one of them could overwrite.
type viewStack []viewFrame

func (s viewStack) top() ActiveView {
	if len(s) == 0 {
		return ViewDashboard
	}
	return s[len(s)-1].view
}

func (s viewStack) push(f viewFrame) viewStack {
	out := make(viewStack, len(s), len(s)+1)
	copy(out, s)
	return append(out, f)
}

// pop removes the top frame. At the root it changes nothing and ok is false.
func (s viewStack) pop() (viewStack, viewFrame, bool) {
	if len(s) == 0 {
		return s, viewFrame{}, false
	}
	return s[: len(s)-1 : len(s)-1], s[len(s)-1], true
}

// toRoot removes every frame and returns the bottom one, which holds the
// cursor from before any frame opened.
func (s viewStack) toRoot() (viewStack, viewFrame, bool) {
	if len(s) == 0 {
		return s, viewFrame{}, false
	}
	return nil, s[0], true
}

// restoreCursor selects the worktree saved in f. A refresh can reorder or
// remove rows while a frame is open, so the path wins over the row index.
// Indexes refer to visible rows, which differ from all rows under a filter.
// The detail pane is only re-rendered when the selection changed, so closing
// a frame keeps its scroll position and focused style.
func (m *Model) restoreCursor(f viewFrame) {
	rows := m.list.VisibleItems()
	if len(rows) == 0 {
		return
	}
	target := min(f.returnIndex, len(rows)-1)
	if f.returnPath != "" {
		for i, row := range rows {
			if item, ok := row.(WorktreeItem); ok && item.Path == f.returnPath {
				target = i
				break
			}
		}
	}
	before, _ := m.selectedItem()
	m.list.Select(target)
	if after, _ := m.selectedItem(); after.Path == before.Path {
		return
	}
	if m.detailFocused {
		m.refreshDetailForFocus()
		m.detail.GotoTop()
		return
	}
	m.updateDetailContent()
}

// activeView is the view on top of the stack, or the dashboard at the root.
func (m Model) activeView() ActiveView { return m.views.top() }

// pushView opens v, remembering the selected worktree to return to.
func (m *Model) pushView(v ActiveView) {
	f := viewFrame{view: v, returnIndex: m.list.Index()}
	if item, ok := m.selectedItem(); ok {
		f.returnPath = item.Path
	}
	m.views = m.views.push(f)
	m.pendingG = false
	m.pendingM = false
}

// popView goes back exactly one level. At the root it does nothing.
func (m *Model) popView() {
	var f viewFrame
	var ok bool
	if m.views, f, ok = m.views.pop(); ok {
		m.restoreCursor(f)
	}
}

// popToRoot closes every frame and returns to the dashboard.
func (m *Model) popToRoot() {
	var f viewFrame
	var ok bool
	if m.views, f, ok = m.views.toRoot(); ok {
		m.restoreCursor(f)
	}
}

// layoutView is the top view that owns layout and help context. Help and
// update overlays describe the view beneath them, so they are skipped.
func (s viewStack) layoutView() ActiveView {
	for i := len(s) - 1; i >= 0; i-- {
		if v := s[i].view; v != ViewHelp && v != ViewUpdate && v != ViewOmni && v != ViewMessages {
			return v
		}
	}
	return ViewDashboard
}

func (m Model) layoutView() ActiveView { return m.views.layoutView() }
